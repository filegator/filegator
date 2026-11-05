package api

import (
	"io"

	"github.com/filegator/filegator/internal/storage"
)

// maxInt64 is the largest value an int64 can hold.
const maxInt64 = int64(^uint64(0) >> 1)

// checkFreeSpace applies the configured minimum-free-space guard to a write
// that needs need bytes. It returns the remaining byte allowance and whether a
// budget is active.
//
// The guard is local-storage only: a backend that does not report free space
// is allowed through, as is a failed probe. Refusing on a probe failure would
// turn an unsupported filesystem into a read-only file manager.
func (s *Server) checkFreeSpace(need int64) (allowance int64, guarded bool, err error) {
	percent := s.Config.Get().Storage.MinFreePercent
	if percent <= 0 {
		return 0, false, nil
	}
	root := s.ActiveStorage()
	if root == nil {
		return 0, false, nil
	}
	reporter, ok := root.(storage.SpaceReporter)
	if !ok {
		return 0, false, nil
	}
	free, total, probeErr := reporter.FreeSpace()
	if probeErr != nil || total == 0 {
		return 0, false, nil
	}

	// Split the multiplication so a large total cannot overflow uint64.
	reserve := total/100*uint64(percent) + total%100*uint64(percent)/100
	if free <= reserve {
		return 0, true, mapStorageError(storage.ErrInsufficientStorage)
	}
	allowance = int64(free - reserve)
	if need > 0 && uint64(need) > uint64(allowance) {
		return 0, true, mapStorageError(storage.ErrInsufficientStorage)
	}
	return allowance, true, nil
}

// writeBudget stops a stream once it has written the allowance calculated
// before the operation started. It is the backstop for an unknown or untrusted
// size, such as an archive whose header understates its contents.
type writeBudget struct {
	remaining int64
}

// wrap returns w unchanged when no budget is active.
func (b *writeBudget) wrap(w io.WriteCloser) io.WriteCloser {
	if b == nil {
		return w
	}
	return &budgetWriter{WriteCloser: w, budget: b}
}

type budgetWriter struct {
	io.WriteCloser
	budget *writeBudget
}

func (w *budgetWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.budget.remaining {
		return 0, storage.ErrInsufficientStorage
	}
	n, err := w.WriteCloser.Write(p)
	w.budget.remaining -= int64(n)
	return n, err
}

// budgetFS wraps a storage view so every file it creates is limited by budget.
// archiver.Extract uses it so an archive cannot write past the reserve even if
// its central directory lies about the uncompressed sizes.
type budgetFS struct {
	storage.FS
	budget *writeBudget
}

func (b budgetFS) Create(p string) (io.WriteCloser, error) {
	w, err := b.FS.Create(p)
	if err != nil {
		return nil, err
	}
	return b.budget.wrap(w), nil
}

func (b budgetFS) OpenWrite(p string) (io.WriteCloser, error) {
	w, err := b.FS.OpenWrite(p)
	if err != nil {
		return nil, err
	}
	return b.budget.wrap(w), nil
}
