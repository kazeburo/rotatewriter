package rotatelogger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotateWriter_BasicWriteAndRotate(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "test.log")
	w, err := New(Filename(filename), MaxSize(1), MaxBackups(2), AutoDirCreate(true))
	if err != nil {
		t.Fatalf("failed to create RotateWriter: %v", err)
	}
	defer w.Close()

	// 1MB = 1024*1024 bytes, so write just under 1MB, then over
	data := make([]byte, 1024*1024-10)
	copy(data, []byte("A"))
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// Should not rotate yet
	if _, err := os.Stat(filename); err != nil {
		t.Errorf("log file missing: %v", err)
	}

	// Write more to trigger rotation
	if _, err := w.Write([]byte("0123456789")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// After rotation, new file should exist
	if _, err := os.Stat(filename); err != nil {
		t.Errorf("log file missing after rotate: %v", err)
	}
	// There should be a rotated file
	files, _ := os.ReadDir(dir)
	rotated := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test-") && strings.HasSuffix(f.Name(), ".log") {
			rotated = true
		}
	}
	if !rotated {
		t.Error("rotated file not found")
	}
}

func TestRotateWriter_MaxBackups(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "test.log")
	w, err := New(Filename(filename), MaxSize(1), MaxBackups(2), AutoDirCreate(true))
	if err != nil {
		t.Fatalf("failed to create RotateWriter: %v", err)
	}
	defer w.Close()
	data := make([]byte, 1024*1024)
	for i := 0; i < 4; i++ {
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
	files, _ := os.ReadDir(dir)
	count := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "test-") && strings.HasSuffix(f.Name(), ".log") {
			count++
		}
	}
	if count > 2 {
		t.Errorf("too many backup files: got %d, want <= 2", count)
	}
}

func TestRotateWriter_AutoDirCreate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir")
	filename := filepath.Join(dir, "test.log")
	_, err := New(Filename(filename), MaxSize(1), MaxBackups(1), AutoDirCreate(true))
	if err != nil {
		t.Fatalf("failed to create RotateWriter with AutoDirCreate: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}
