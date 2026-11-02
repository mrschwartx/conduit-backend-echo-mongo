//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"conduit/backend/test/support"
)

var (
	apiBinary   string
	seedBinary  string
	listenLine  = regexp.MustCompile(`Server is running on http://127\.0\.0\.1:(\d+)`)
	startupWait = 20 * time.Second
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "conduit-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	apiBinary = filepath.Join(dir, "api")
	seedBinary = filepath.Join(dir, "seed")

	for target, output := range map[string]string{"./cmd/backend": apiBinary, "./cmd/seed": seedBinary} {
		build := exec.Command("go", "build", "-o", output, target)
		build.Dir = support.ProjectRoot()
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build %s failed: %v\n%s", target, err, out)
			os.RemoveAll(dir)
			os.Exit(1)
		}
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type process struct {
	cmd    *exec.Cmd
	stdout *syncBuffer
	stderr *syncBuffer
	exited chan struct{}
	err    error
}

type syncBuffer struct {
	buffer bytes.Buffer
	lines  chan string
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.buffer.Write(data)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		select {
		case b.lines <- scanner.Text():
		default:
		}
	}
	return len(data), nil
}

func environment(overrides map[string]string) []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	for key, value := range support.TestEnv(overrides) {
		env = append(env, key+"="+value)
	}
	return env
}

func start(t *testing.T, binary string, overrides map[string]string) *process {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Dir = t.TempDir()
	cmd.Env = environment(overrides)

	p := &process{
		cmd:    cmd,
		stdout: &syncBuffer{lines: make(chan string, 256)},
		stderr: &syncBuffer{lines: make(chan string, 256)},
		exited: make(chan struct{}),
	}
	cmd.Stdout = p.stdout
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		p.err = cmd.Wait()
		close(p.exited)
	}()
	t.Cleanup(func() {
		select {
		case <-p.exited:
		default:
			_ = cmd.Process.Kill()
			<-p.exited
		}
	})
	return p
}

func (p *process) waitForPort(t *testing.T) int {
	t.Helper()
	deadline := time.After(startupWait)
	for {
		select {
		case line := <-p.stdout.lines:
			if match := listenLine.FindStringSubmatch(line); match != nil {
				port, _ := strconv.Atoi(match[1])
				return port
			}
		case <-p.exited:
			t.Fatalf("process exited early: %v\nstdout: %s\nstderr: %s", p.err, p.stdout.buffer.String(), p.stderr.buffer.String())
		case <-deadline:
			t.Fatalf("server did not start\nstdout: %s\nstderr: %s", p.stdout.buffer.String(), p.stderr.buffer.String())
		}
	}
}

func (p *process) wait(t *testing.T) int {
	t.Helper()
	select {
	case <-p.exited:
		if p.cmd.ProcessState == nil {
			return -1
		}
		return p.cmd.ProcessState.ExitCode()
	case <-time.After(startupWait):
		t.Fatalf("process did not exit\nstdout: %s\nstderr: %s", p.stdout.buffer.String(), p.stderr.buffer.String())
		return -1
	}
}

func serverEnv(t *testing.T) map[string]string {
	t.Helper()
	database := support.StartTestDatabase(t)
	return map[string]string{
		"DATABASE_URL": database.URL,
		"LOG_SILENT":   "false",
		"APP_LEVEL":    "production",
		"LOG_DIR":      t.TempDir(),
	}
}

func TestServerStartsServesAndShutsDownGracefully(t *testing.T) {
	process := start(t, apiBinary, serverEnv(t))
	port := process.waitForPort(t)

	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("status = %d", response.StatusCode)
	}

	if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := process.wait(t); code != 0 {
		t.Fatalf("exit code = %d\nstdout: %s\nstderr: %s", code, process.stdout.buffer.String(), process.stderr.buffer.String())
	}
	if !strings.Contains(process.stdout.buffer.String(), "Shutdown complete") {
		t.Fatalf("missing shutdown log: %s", process.stdout.buffer.String())
	}
}

func TestServerShutsDownOnInterrupt(t *testing.T) {
	process := start(t, apiBinary, serverEnv(t))
	process.waitForPort(t)
	if err := process.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if code := process.wait(t); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestServerWritesJSONLogsToFileWhenEnabled(t *testing.T) {
	env := serverEnv(t)
	env["LOG_TO_FILE"] = "true"
	process := start(t, apiBinary, env)
	port := process.waitForPort(t)

	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	_ = process.cmd.Process.Signal(syscall.SIGTERM)
	process.wait(t)

	files, _ := filepath.Glob(filepath.Join(env["LOG_DIR"], "conduit-api-*.log*"))
	if len(files) == 0 {
		t.Fatal("no log file written")
	}
	content, _ := os.ReadFile(files[0])
	if !strings.Contains(string(content), `"msg":"HTTP request"`) {
		t.Fatalf("request log missing: %s", content)
	}
}

func TestServerRefusesToStartWithInvalidConfiguration(t *testing.T) {
	cases := map[string]struct {
		overrides map[string]string
		remove    string
		want      string
	}{
		"missing secret":   {remove: "AUTH_SECRET", want: "AUTH_SECRET"},
		"short secret":     {overrides: map[string]string{"AUTH_SECRET": "short"}, want: "at least 32"},
		"missing database": {remove: "DATABASE_URL", want: "DATABASE_URL"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(apiBinary)
			cmd.Dir = t.TempDir()
			env := support.TestEnv(tc.overrides)
			if tc.remove != "" {
				delete(env, tc.remove)
			}
			cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
			for key, value := range env {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() == 0 {
				t.Fatalf("expected a non zero exit, got %v", err)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr %q lacks %q", stderr.String(), tc.want)
			}
		})
	}
}

func TestServerFailsWhenTheDatabaseIsUnreachable(t *testing.T) {
	process := start(t, apiBinary, map[string]string{
		"DATABASE_URL":     "postgres://nobody:nothing@127.0.0.1:1/none",
		"DATABASE_TIMEOUT": "500",
	})
	if code := process.wait(t); code == 0 {
		t.Fatal("expected a failing exit code")
	}
}

func TestSeedPopulatesIdempotently(t *testing.T) {
	env := serverEnv(t)
	env["APP_LEVEL"] = "development"

	for range 2 {
		process := start(t, seedBinary, env)
		if code := process.wait(t); code != 0 {
			t.Fatalf("seed exit code = %d\nstderr: %s", code, process.stderr.buffer.String())
		}
	}

	server := start(t, apiBinary, env)
	port := server.waitForPort(t)
	client := support.NewClient(t, fmt.Sprintf("http://127.0.0.1:%d", port))

	login := client.Post("/api/v1/users/login", support.RequestOptions{JSON: map[string]any{"user": map[string]any{
		"email": "jane@mail.com", "password": "securepassword456",
	}}})
	if login.Status != 200 {
		t.Fatalf("seeded user cannot sign in: %d %s", login.Status, login.Raw)
	}
	token := login.String("user", "token")

	articles := client.Get("/api/v1/articles")
	if articles.Number("articlesCount") != 2 {
		t.Fatalf("seeding twice must not duplicate articles: %s", articles.Raw)
	}
	feed := client.Get("/api/v1/articles/feed", support.RequestOptions{Token: token})
	if feed.Number("articlesCount") != 1 {
		t.Fatalf("jane follows john, so the feed has one article: %s", feed.Raw)
	}
	tags := client.Get("/api/v1/tags")
	if len(tags.Slice("tags")) != 3 {
		t.Fatalf("unexpected tags: %s", tags.Raw)
	}
	comments := client.Get("/api/v1/articles/getting-started-with-conduit/comments")
	if len(comments.Slice("comments")) != 1 {
		t.Fatalf("unexpected comments: %s", comments.Raw)
	}
}

func TestSeedRefusesToRunInProduction(t *testing.T) {
	env := serverEnv(t)
	env["APP_LEVEL"] = "production"
	process := start(t, seedBinary, env)
	if code := process.wait(t); code == 0 {
		t.Fatal("seed must refuse production")
	}
}

func TestUserJourney(t *testing.T) {
	process := start(t, apiBinary, serverEnv(t))
	port := process.waitForPort(t)
	client := support.NewClient(t, fmt.Sprintf("http://127.0.0.1:%d", port))

	author, authorToken := support.MustRegisterUser(t, client)
	_, readerToken := support.MustRegisterUser(t, client)
	authorAuth := support.RequestOptions{Token: authorToken}
	readerAuth := support.RequestOptions{Token: readerToken}

	created := client.Post("/api/v1/articles", support.RequestOptions{
		Token: authorToken,
		JSON:  support.ArticlePayload(nil),
	})
	if created.Status != 201 {
		t.Fatalf("create failed: %s", created.Raw)
	}
	path := "/api/v1/articles/" + created.String("article", "slug")

	if follow := client.Post("/api/v1/profiles/"+author.Username+"/follow", readerAuth); follow.Status != 200 {
		t.Fatalf("follow failed: %s", follow.Raw)
	}
	if feed := client.Get("/api/v1/articles/feed", readerAuth); feed.Number("articlesCount") != 1 {
		t.Fatalf("feed failed: %s", feed.Raw)
	}

	if favorite := client.Post(path+"/favorite", readerAuth); favorite.Number("article", "favoritesCount") != 1 {
		t.Fatalf("favorite failed: %s", favorite.Raw)
	}

	comment := client.Post(path+"/comments", support.RequestOptions{
		Token: readerToken,
		JSON:  map[string]any{"comment": map[string]any{"body": "Great article"}},
	})
	if comment.Status != 201 {
		t.Fatalf("comment failed: %s", comment.Raw)
	}
	commentPath := fmt.Sprintf("%s/comments/%d", path, int(comment.Number("comment", "id")))

	updated := client.Put(path, support.RequestOptions{
		Token: authorToken,
		JSON:  map[string]any{"article": map[string]any{"title": "A better title"}},
	})
	if updated.String("article", "slug") != "a-better-title" {
		t.Fatalf("update failed: %s", updated.Raw)
	}
	path = "/api/v1/articles/a-better-title"
	commentPath = path + commentPath[strings.LastIndex(commentPath, "/comments"):]

	if deleted := client.Delete(commentPath, readerAuth); deleted.Status != 204 {
		t.Fatalf("comment delete failed: %d %s", deleted.Status, deleted.Raw)
	}
	if deleted := client.Delete(path, authorAuth); deleted.Status != 204 {
		t.Fatalf("article delete failed: %d %s", deleted.Status, deleted.Raw)
	}
	if gone := client.Get(path); gone.Status != 404 {
		t.Fatalf("expected 404 after deletion, got %d", gone.Status)
	}
}
