package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/storage"
	"github.com/filegator/filegator/internal/storage/local"
	"github.com/filegator/filegator/internal/storage/s3"
	"github.com/filegator/filegator/internal/storage/sftp"
)

// LocalRoot returns the folder the local backend would create, or "" for a
// remote backend: only the local one writes to this machine. Exported so the
// process can ask the operator about the exact paths before the app is built.
func LocalRoot(cfg config.Storage) string {
	switch cfg.Type {
	case "", config.StorageTypeLocal:
		return cfg.Root
	}
	return ""
}

// openStorage builds the storage backend selected by the configuration.
//
// Only the local backend is writable through the admin UI at runtime; a remote
// backend is chosen in the configuration file, so its connection settings live
// there rather than in a path picker.
//
// notes records a root that had to be created, since an unexpected storage
// folder is the first thing that looks wrong on a fresh install.
func openStorage(cfg config.Storage, notes *StartupLog) (storage.Storage, error) {
	switch cfg.Type {
	case "", config.StorageTypeLocal:
		abs, err := filepath.Abs(cfg.Root)
		if err != nil {
			return nil, fmt.Errorf("storage: resolve root: %w", err)
		}
		// Checked before opening: local.NewRoot creates the folder, so
		// afterwards there is no way to tell whether it was there already.
		_, statErr := os.Stat(abs)
		if os.IsNotExist(statErr) {
			notes.add(noteInfo, "storage.created",
				"Created the storage folder "+abs+". If that is not where your files are, "+
					"set the right one on Repository.")
		}
		return local.NewRoot(cfg.Root)
	case config.StorageTypeS3:
		backend, err := s3.New(cfg.S3)
		if err != nil {
			return nil, fmt.Errorf("storage: %w", err)
		}
		log.Printf("using S3 storage at %s", backend.Description())
		return backend, nil
	case config.StorageTypeSFTP:
		backend, err := sftp.New(cfg.SFTP, cfg.Root)
		if err != nil {
			return nil, fmt.Errorf("storage: %w", err)
		}
		log.Printf("using SFTP storage at %s", backend.Description())
		return backend, nil
	default:
		return nil, fmt.Errorf("storage: unknown type %q", cfg.Type)
	}
}
