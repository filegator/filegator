package local

import (
	"errors"
	"os"
	"testing"

	"github.com/filegator/filegator/internal/storage"
)

func TestFreeSpaceReportsTheRootFilesystem(t *testing.T) {
	r := newRoot(t)
	if !r.Capabilities().FreeSpace {
		t.Fatal("local storage does not advertise free space")
	}
	free, total, err := r.FreeSpace()
	if err != nil {
		t.Fatalf("FreeSpace: %v", err)
	}
	if total == 0 {
		t.Fatal("total size is zero")
	}
	if free > total {
		t.Errorf("free %d is larger than total %d", free, total)
	}
}

func TestFreeSpaceFailsForAMissingRoot(t *testing.T) {
	r := newRoot(t)
	if err := os.RemoveAll(r.Dir()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.FreeSpace(); err == nil {
		t.Fatal("missing root reported free space")
	}
}

func TestCopyWithLimitStopsBeforeTheBudgetIsCrossed(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "src/a.txt", "hello")  // 5 bytes
	writeFile(t, r.Dir(), "src/b.txt", "world!") // 6 bytes

	written, err := r.CopyWithLimit("/src", "/dst", 5)
	if !errors.Is(err, storage.ErrInsufficientStorage) {
		t.Fatalf("err = %v, want ErrInsufficientStorage", err)
	}
	if written != 5 {
		t.Errorf("written = %d, want 5", written)
	}
	// The first file was copied; the second was refused before it was created.
	if _, err := r.Stat("/dst/a.txt"); err != nil {
		t.Errorf("first file missing: %v", err)
	}
	if r.Exists("/dst/b.txt") {
		t.Error("second file was created past the budget")
	}
}

func TestCopyWithLimitSuccess(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "src/a.txt", "hello")
	writeFile(t, r.Dir(), "src/b.txt", "world!")

	written, err := r.CopyWithLimit("/src", "/dst", 11)
	if err != nil {
		t.Fatalf("CopyWithLimit: %v", err)
	}
	if written != 11 {
		t.Errorf("written = %d, want 11", written)
	}
	if got, err := r.Stat("/dst/b.txt"); err != nil || got.Size != 6 {
		t.Errorf("copied file = %+v, %v", got, err)
	}
}

func TestCopyWithLimitRejectsAFileBeforeCreatingIt(t *testing.T) {
	r := newRoot(t)
	writeFile(t, r.Dir(), "src/a.txt", "hello")

	written, err := r.CopyWithLimit("/src/a.txt", "/too-big", 4)
	if !errors.Is(err, storage.ErrInsufficientStorage) {
		t.Fatalf("err = %v, want ErrInsufficientStorage", err)
	}
	if written != 0 {
		t.Errorf("written = %d, want 0", written)
	}
	if r.Exists("/too-big") {
		t.Error("a file larger than the budget was created")
	}
}
