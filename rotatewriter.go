package rotatewriter

import (
	"fmt"
	"log"
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
	err := w.openFile()
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (w *RotateWriter) openFile() error {
	if w.AutoDirCreate {
		if err := os.MkdirAll(w.dir, 0755); err != nil {
			return err
		}
	}
	// Return an error if the target path is a symlink.
	if fi, err := os.Lstat(w.Filename); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("file is a symlink: %s", w.Filename)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	file, err := openFile(w.Filename)
	if err != nil {
		return err
	}

	fi, err := file.Stat()
	if err != nil {
		if errClose := file.Close(); errClose != nil {
			return fmt.Errorf("stat failed: %w (close failed: %w)", err, errClose)
		}
		return err
	}

	w.file = file
	w.currentSize = fi.Size()
	return nil
}

func (w *RotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, fmt.Errorf("file is not open")
	}
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

func (w *RotateWriter) rotateBackups() error {
	if w.MaxBackups <= 0 {
		return nil
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	var backupTSs []int64
	prefix := fmt.Sprintf("%s-", w.basename)
	suffix := w.ext
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), suffix) {
			fname := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), prefix), suffix)
			n, err := strconv.ParseInt(fname, 10, 64)
			if err != nil {
				log.Printf("rotatewriter: skipping backup file with invalid timestamp %q: %v", entry.Name(), err)
				continue
			}
			backupTSs = append(backupTSs, n)
		}
	}

	slices.Sort(backupTSs)

	if w.MaxBackups >= len(backupTSs) {
		return nil
	}

	toDelete := len(backupTSs) - w.MaxBackups
	for _, ts := range backupTSs[:toDelete] {
		path := filepath.Join(w.dir, fmt.Sprintf("%s-%d%s", w.basename, ts, w.ext))
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (w *RotateWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		w.file = nil
		w.currentSize = 0
		return err
	}
	w.file = nil
	w.currentSize = 0
	currentLog := filepath.Join(w.dir, fmt.Sprintf("%s%s", w.basename, w.ext))
	backupLog := filepath.Join(w.dir, fmt.Sprintf("%s-%d%s", w.basename, time.Now().UnixNano(), w.ext))
	if err := os.Rename(currentLog, backupLog); err != nil {
		return err
	}

	if err := w.rotateBackups(); err != nil {
		// If backup rotation fails, we log the error but continue to open a new file.
		// This is to ensure that logging can continue even if backup cleanup fails.
		log.Printf("failed to rotate backups: %v", err)
	}

	if err := w.openFile(); err != nil {
		return err
	}
	return nil
}

func (w *RotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		w.currentSize = 0
		return err
	}
	return nil
}
