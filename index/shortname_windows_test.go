package index

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// A dot folder reached by its 8.3 short name (.git as GIT~1) is still a dot
// path: the containment check refuses it, whatever name it is given.
func TestAShortNameOfADotFolderIsRefused(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	long := filepath.Join(root, ".git", "config")
	if err := os.WriteFile(long, []byte("[core]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := syscall.UTF16PtrFromString(long)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Skipf("no short name: %v", err)
	}
	short := syscall.UTF16ToString(buf[:n])
	if filepath.Base(filepath.Dir(short)) == ".git" {
		t.Skip("8.3 names are off on this volume")
	}
	if _, err := inBundle(root, short); !errors.Is(err, ErrOutsideBundle) {
		t.Errorf("inBundle(%s) = %v, want it refused", short, err)
	}
}
