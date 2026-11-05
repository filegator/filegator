//go:build !windows

package local

import "syscall"

// FreeSpace reports the bytes available to unprivileged writers and the total
// size of the filesystem holding the root. Bavail is used rather than Bfree so
// the reserve the operating system keeps for root is not counted as usable.
func (r *Root) FreeSpace() (free, total uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(r.dir, &stat); err != nil {
		return 0, 0, err
	}
	blockSize := uint64(stat.Bsize)
	return uint64(stat.Bavail) * blockSize, uint64(stat.Blocks) * blockSize, nil
}
