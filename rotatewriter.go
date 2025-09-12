package rotatelogger

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultMaxSize       = 100
	DefaultMaxBackups    = 7
	DefaultAutoDirCreate = false
)

type RotateWriter struct {
	Filename      string
	MaxSize       int
	MaxBackups    int
	AutoDirCreate bool
	currentSize   int64
	file          *os.File
	dir           string
	basename      string
	ext           string
	mu            sync.Mutex
}

type WriterOption func(*RotateWriter)

func Filename(name string) WriterOption {
	return func(wc *RotateWriter) {
		wc.Filename = name
	}
}

func MaxSize(size int) WriterOption {
	return func(wc *RotateWriter) {
		wc.MaxSize = size
	}
}

func MaxBackups(num int) WriterOption {
	return func(wc *RotateWriter) {
		wc.MaxBackups = num
	}
}

func AutoDirCreate(auto bool) WriterOption {
	return func(wc *RotateWriter) {
		wc.AutoDirCreate = auto
	}
}

func New(ops ...WriterOption) (*RotateWriter, error) {
	w := &RotateWriter{
		MaxSize:       DefaultMaxSize,
		MaxBackups:    DefaultMaxBackups,
		AutoDirCreate: DefaultAutoDirCreate,
	}
	for _, op := range ops {
		op(w)
	}
	if w.Filename == "" {
		return nil, fmt.Errorf("filename is required")
	}
	w.dir = filepath.Dir(w.Filename)
	w.basename = strings.TrimSuffix(filepath.Base(w.Filename), filepath.Ext(w.Filename))
	w.ext = filepath.Ext(w.Filename)
	w.mu.Lock()
	defer w.mu.Unlock()
	file, err := w.openFile()
	if err != nil {
		return nil, err
	}
	w.file = file
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	w.currentSize = fi.Size()
	return w, nil
}

func (w *RotateWriter) openFile() (*os.File, error) {
	if w.AutoDirCreate {
		if err := os.MkdirAll(w.dir, 0755); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(w.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func (w *RotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentSize+int64(len(p)) >= int64(w.MaxSize*1024*1024) {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}
	w.currentSize += int64(n)
	return n, err
}

func (w *RotateWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}

	currentLog := filepath.Join(w.dir, fmt.Sprintf("%s%s", w.basename, w.ext))
	backupLog := filepath.Join(w.dir, fmt.Sprintf("%s-%d%s", w.basename, time.Now().UnixNano(), w.ext))
	if err := os.Rename(currentLog, backupLog); err != nil {
		return err
	}

	if w.MaxBackups > 0 {
		file, err := os.ReadDir(w.dir)
		if err != nil {
			return err
		}
		var ints []int64
		prefix := fmt.Sprintf("%s-", w.basename)
		suffix := w.ext
		for _, f := range file {
			if strings.HasPrefix(f.Name(), prefix) && strings.HasSuffix(f.Name(), suffix) {
				fname := strings.TrimSuffix(strings.TrimPrefix(f.Name(), prefix), suffix)
				n, err := strconv.ParseInt(fname, 10, 64)
				if err != nil {
					continue
				}
				ints = append(ints, n)
			}
		}

		slices.Sort(ints)

		if len(ints) > w.MaxBackups {
			toDelete := len(ints) - w.MaxBackups
			for i := 0; i < toDelete; i++ {
				path := filepath.Join(w.dir, fmt.Sprintf("%s-%d%s", w.basename, ints[i], w.ext))
				if err := os.Remove(path); err != nil {
					return err
				}
			}
		}
	}

	file, err := w.openFile()
	if err != nil {
		return err
	}
	w.file = file
	w.currentSize = 0
	return nil
}

func (w *RotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
