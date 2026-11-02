package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RotatingFileConfig struct {
	Dir        string
	Prefix     string
	MaxBytes   int64
	MaxAgeDays int
	Compress   bool
	Now        func() time.Time
}

type RotatingFile struct {
	cfg     RotatingFileConfig
	mu      sync.Mutex
	file    *os.File
	size    int64
	day     string
	pending sync.WaitGroup
}

func NewRotatingFile(cfg RotatingFileConfig) (*RotatingFile, error) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, err
	}
	rotating := &RotatingFile{cfg: cfg}
	rotating.mu.Lock()
	defer rotating.mu.Unlock()
	if err := rotating.open(); err != nil {
		return nil, err
	}
	rotating.prune()
	return rotating, nil
}

func (r *RotatingFile) Write(payload []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil || r.day != r.today() || (r.cfg.MaxBytes > 0 && r.size+int64(len(payload)) > r.cfg.MaxBytes) {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}
	written, err := r.file.Write(payload)
	r.size += int64(written)
	return written, err
}

func (r *RotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var err error
	if r.file != nil {
		err = r.file.Close()
		r.file = nil
	}
	r.pending.Wait()
	return err
}

func (r *RotatingFile) today() string {
	return r.cfg.Now().Format("2006-01-02")
}

func (r *RotatingFile) activePath(day string) string {
	return filepath.Join(r.cfg.Dir, fmt.Sprintf("%s-%s.log", r.cfg.Prefix, day))
}

func (r *RotatingFile) open() error {
	day := r.today()
	file, err := os.OpenFile(r.activePath(day), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	r.file = file
	r.size = info.Size()
	r.day = day
	return nil
}

func (r *RotatingFile) rotate() error {
	previousDay := r.day
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			return err
		}
		r.file = nil
		if previousDay == r.today() {
			archived := r.archivePath(previousDay)
			if err := os.Rename(r.activePath(previousDay), archived); err != nil {
				return err
			}
			r.compress(archived)
		} else {
			r.compress(r.activePath(previousDay))
		}
	}
	if err := r.open(); err != nil {
		return err
	}
	r.prune()
	return nil
}

func (r *RotatingFile) archivePath(day string) string {
	for index := 1; ; index++ {
		candidate := filepath.Join(r.cfg.Dir, fmt.Sprintf("%s-%s.%d.log", r.cfg.Prefix, day, index))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			if _, err := os.Stat(candidate + ".gz"); os.IsNotExist(err) {
				return candidate
			}
		}
	}
}

func (r *RotatingFile) compress(path string) {
	if !r.cfg.Compress {
		return
	}
	r.pending.Add(1)
	go func() {
		defer r.pending.Done()
		_ = gzipFile(path)
	}()
}

func (r *RotatingFile) prune() {
	if r.cfg.MaxAgeDays <= 0 {
		return
	}
	cutoff := r.cfg.Now().AddDate(0, 0, -r.cfg.MaxAgeDays)
	entries, err := os.ReadDir(r.cfg.Dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, r.cfg.Prefix+"-") {
			continue
		}
		if !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".log.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(r.cfg.Dir, name))
		}
	}
}

func gzipFile(path string) error {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(path+".gz", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	writer := gzip.NewWriter(target)
	if _, err := io.Copy(writer, source); err != nil {
		_ = writer.Close()
		_ = target.Close()
		_ = os.Remove(path + ".gz")
		return err
	}
	if err := writer.Close(); err != nil {
		_ = target.Close()
		_ = os.Remove(path + ".gz")
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}
	return os.Remove(path)
}
