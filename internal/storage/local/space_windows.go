//go:build windows

package local

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// FreeSpace reports the bytes available to the calling user and the total size
// of the volume holding the root. The standard library has no wrapper for
// GetDiskFreeSpaceEx, so the one call is bound directly rather than adding a
// dependency for it.
func (r *Root) FreeSpace() (free, total uint64, err error) {
	path, err := syscall.UTF16PtrFromString(r.dir)
	if err != nil {
		return 0, 0, err
	}
	var available, totalBytes, freeBytes uint64
	ok, _, callErr := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&freeBytes)),
	)
	if ok == 0 {
		return 0, 0, callErr
	}
	return available, totalBytes, nil
}
