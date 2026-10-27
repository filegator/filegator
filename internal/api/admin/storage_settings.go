package admin

import (
	"errors"
	"net/http"

	"github.com/filegator/filegator/internal/api/httpx"
	"strings"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/storage"
	"github.com/filegator/filegator/internal/storage/local"
	"github.com/filegator/filegator/internal/storage/s3"
	"github.com/filegator/filegator/internal/storage/sftp"
)

// StorageConfig is the admin view of the storage backend settings. The SFTP
// password is never returned; HasPassword reports whether one is stored so the
// form can show a placeholder instead.
type StorageConfig struct {
	Type string `json:"type"`
	Root string `json:"root"`

	SFTPHost       string `json:"sftp_host"`
	SFTPPort       int    `json:"sftp_port"`
	SFTPUser       string `json:"sftp_user"`
	SFTPPrivateKey string `json:"sftp_private_key"`
	SFTPKnownHosts string `json:"sftp_known_hosts"`
	SFTPTimeout    int    `json:"sftp_timeout"`
	// SFTPPassword is write-only. An empty value keeps the stored password.
	SFTPPassword    string `json:"sftp_password,omitempty"`
	SFTPHasPassword bool   `json:"sftp_has_password"`

	S3Bucket       string `json:"s3_bucket"`
	S3Region       string `json:"s3_region"`
	S3Endpoint     string `json:"s3_endpoint"`
	S3AccessKey    string `json:"s3_access_key"`
	S3SecretKey    string `json:"s3_secret_key,omitempty"`
	S3HasSecret    bool   `json:"s3_has_secret"`
	S3UsePathStyle bool   `json:"s3_use_path_style"`
	S3Prefix       string `json:"s3_prefix"`
	S3Timeout      int    `json:"s3_timeout"`
}

// GetStorage handles GET /api/admin/storage.
func (h *Handler) GetStorage(w http.ResponseWriter, r *http.Request, _ router.Params) {
	cfg := h.Core.Config.Get().Storage
	httpx.WriteJSON(w, http.StatusOK, StorageConfig{
		Type:            cfg.Type,
		Root:            cfg.Root,
		SFTPHost:        cfg.SFTP.Host,
		SFTPPort:        cfg.SFTP.Port,
		SFTPUser:        cfg.SFTP.User,
		SFTPPrivateKey:  cfg.SFTP.PrivateKey,
		SFTPKnownHosts:  cfg.SFTP.KnownHosts,
		SFTPTimeout:     cfg.SFTP.Timeout,
		SFTPHasPassword: cfg.SFTP.Password != "",

		S3Bucket:       cfg.S3.Bucket,
		S3Region:       cfg.S3.Region,
		S3Endpoint:     cfg.S3.Endpoint,
		S3AccessKey:    cfg.S3.AccessKey,
		S3HasSecret:    cfg.S3.SecretKey != "",
		S3UsePathStyle: cfg.S3.UsePathStyle,
		S3Prefix:       cfg.S3.Prefix,
		S3Timeout:      cfg.S3.Timeout,
	})
}

// SetStorage handles POST /api/admin/storage and switches the
// storage backend at runtime.
//
// The new backend is validated before anything is saved: the remote server must
// connect and the root must be readable. A failed switch leaves the previous
// backend active, so an admin cannot lock everyone out with a typo.
func (h *Handler) SetStorage(w http.ResponseWriter, r *http.Request, _ router.Params) {
	var req StorageConfig
	if err := httpx.DecodeJSON(w, r, &req, httpx.MaxJSONBody); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if req.Type == "" {
		req.Type = config.StorageTypeLocal
	}

	// Build the candidate configuration, keeping the stored password when the
	// form left it blank.
	current := h.Core.Config.Get().Storage
	if req.SFTPPassword == "" && req.SFTPHasPassword && req.Type == config.StorageTypeSFTP {
		req.SFTPPassword = current.SFTP.Password
	}
	if req.S3SecretKey == "" && req.S3HasSecret && req.Type == config.StorageTypeS3 {
		req.S3SecretKey = current.S3.SecretKey
	}

	candidate := config.Storage{
		Type: req.Type,
		Root: strings.TrimSpace(req.Root),
		SFTP: config.SFTP{
			Host:       strings.TrimSpace(req.SFTPHost),
			Port:       req.SFTPPort,
			User:       strings.TrimSpace(req.SFTPUser),
			Password:   req.SFTPPassword,
			PrivateKey: strings.TrimSpace(req.SFTPPrivateKey),
			KnownHosts: strings.TrimSpace(req.SFTPKnownHosts),
			Timeout:    req.SFTPTimeout,
		},
		S3: config.S3{
			Bucket:       strings.TrimSpace(req.S3Bucket),
			Region:       strings.TrimSpace(req.S3Region),
			Endpoint:     strings.TrimSpace(req.S3Endpoint),
			AccessKey:    strings.TrimSpace(req.S3AccessKey),
			SecretKey:    req.S3SecretKey,
			UsePathStyle: req.S3UsePathStyle,
			Prefix:       strings.TrimSpace(req.S3Prefix),
			Timeout:      req.S3Timeout,
		},
	}

	// Validate against a copy of the config so a bad value cannot slip through.
	probe := h.Core.Config.Get()
	probe.Storage = candidate
	if err := probe.Validate(); err != nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, err.Error()))
		return
	}

	backend, err := openBackend(candidate)
	if err != nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, err.Error()))
		return
	}
	if _, err := backend.Scope("/"); err != nil {
		httpx.WriteError(w, httpx.Errorf(http.StatusBadRequest, httpx.CodeBadRequest, "cannot read the storage root: "+err.Error()))
		return
	}

	// Persist, then swap. A read-only config still switches for this run, but
	// the caller is told the choice will be lost on restart: answering 200
	// made a switch that cannot survive a restart look saved.
	if err := h.Core.Config.Update(func(c *config.Config) error {
		c.Storage = candidate
		return nil
	}); err != nil {
		if !errors.Is(err, config.ErrReadOnly) {
			httpx.WriteError(w, err)
			return
		}
		h.Core.SetRoot(backend, backend.Description())
		h.Core.LogAdmin(r, logger.EventAdminStorageChanged, "storage backend changed but was not saved",
			logger.Fields{"type": candidate.Type, "description": backend.Description(), "read_only": true})
		httpx.WriteError(w, httpx.Errorf(http.StatusConflict, httpx.CodeReadOnlyConfig,
			"the storage backend was switched for this session but could not be saved: "+readOnlyReason(h.Core.Config)))
		return
	}

	h.Core.SetRoot(backend, backend.Description())
	h.Core.LogAdmin(r, logger.EventAdminStorageChanged, "storage backend changed",
		logger.Fields{"type": candidate.Type, "description": backend.Description()})
	httpx.WriteJSON(w, http.StatusOK, StorageConfig{
		Type:            candidate.Type,
		Root:            candidate.Root,
		SFTPHost:        candidate.SFTP.Host,
		SFTPPort:        candidate.SFTP.Port,
		SFTPUser:        candidate.SFTP.User,
		SFTPPrivateKey:  candidate.SFTP.PrivateKey,
		SFTPKnownHosts:  candidate.SFTP.KnownHosts,
		SFTPTimeout:     candidate.SFTP.Timeout,
		SFTPHasPassword: candidate.SFTP.Password != "",

		S3Bucket:       candidate.S3.Bucket,
		S3Region:       candidate.S3.Region,
		S3Endpoint:     candidate.S3.Endpoint,
		S3AccessKey:    candidate.S3.AccessKey,
		S3HasSecret:    candidate.S3.SecretKey != "",
		S3UsePathStyle: candidate.S3.UsePathStyle,
		S3Prefix:       candidate.S3.Prefix,
		S3Timeout:      candidate.S3.Timeout,
	})
}

// openBackend builds a storage backend from a configuration.
func openBackend(cfg config.Storage) (storage.Storage, error) {
	switch cfg.Type {
	case "", config.StorageTypeLocal:
		return local.NewRoot(cfg.Root)
	case config.StorageTypeSFTP:
		return sftp.New(cfg.SFTP, cfg.Root)
	case config.StorageTypeS3:
		return s3.New(cfg.S3)
	default:
		return nil, errors.New("unknown storage type " + cfg.Type)
	}
}
