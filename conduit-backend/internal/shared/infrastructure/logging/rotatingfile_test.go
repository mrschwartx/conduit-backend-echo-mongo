package logging

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestRotatingFileWritesToDatedFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	file, err := NewRotatingFile(RotatingFileConfig{Dir: dir, Prefix: "app", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "app-2026-03-01.log"))
	if err != nil || string(content) != "hello\n" {
		t.Fatalf("unexpected content %q %v", content, err)
	}
}

func TestRotatingFileRotatesWhenTheDayChanges(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 1, 23, 59, 0, 0, time.UTC)
	file, err := NewRotatingFile(RotatingFileConfig{Dir: dir, Prefix: "app", Compress: true, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("day one\n"))
	now = now.Add(2 * time.Minute)
	_, _ = file.Write([]byte("day two\n"))
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	names := strings.Join(listFiles(t, dir), ",")
	if names != "app-2026-03-01.log.gz,app-2026-03-02.log" {
		t.Fatalf("unexpected files %s", names)
	}
	archive, _ := os.Open(filepath.Join(dir, "app-2026-03-01.log.gz"))
	defer archive.Close()
	reader, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := io.ReadAll(reader)
	if string(restored) != "day one\n" {
		t.Fatalf("archive content %q", restored)
	}
}

func TestRotatingFileRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	file, err := NewRotatingFile(RotatingFileConfig{Dir: dir, Prefix: "app", MaxBytes: 10, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := file.Write([]byte("123456\n")); err != nil {
			t.Fatal(err)
		}
	}
	_ = file.Close()

	names := listFiles(t, dir)
	if len(names) != 3 {
		t.Fatalf("expected 3 files, got %v", names)
	}
}

func TestRotatingFilePrunesOldFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	old := filepath.Join(dir, "app-2026-03-01.log.gz")
	recent := filepath.Join(dir, "app-2026-03-19.log.gz")
	unrelated := filepath.Join(dir, "other.log")
	for _, path := range []string{old, recent, unrelated} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.Chtimes(old, now.AddDate(0, 0, -19), now.AddDate(0, 0, -19))
	_ = os.Chtimes(recent, now.AddDate(0, 0, -1), now.AddDate(0, 0, -1))
	_ = os.Chtimes(unrelated, now.AddDate(0, 0, -90), now.AddDate(0, 0, -90))

	file, err := NewRotatingFile(RotatingFileConfig{Dir: dir, Prefix: "app", MaxAgeDays: 14, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("old logs must be pruned")
	}
	for _, kept := range []string{recent, unrelated} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s must be kept", kept)
		}
	}
}
