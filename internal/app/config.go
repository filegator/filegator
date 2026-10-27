package app

import (
	"net/http"

	"github.com/filegator/filegator/internal/api"
	"github.com/filegator/filegator/internal/router"
)

// PublicConfig is the subset of configuration the browser needs before
// logging in. It deliberately excludes anything sensitive.
type PublicConfig struct {
	AppName            string   `json:"app_name"`
	Language           string   `json:"language"`
	LanguageSelection  bool     `json:"language_selection"`
	Logo               string   `json:"logo"`
	LogoDark           string   `json:"logo_dark"`
	UploadMaxSize      int64    `json:"upload_max_size"`
	UploadChunkSize    int64    `json:"upload_chunk_size"`
	UploadSimultaneous int      `json:"upload_simultaneous"`
	DefaultArchiveName string   `json:"default_archive_name"`
	Editable           []string `json:"editable"`
	DateFormat         string   `json:"date_format"`
	GuestRedirection   string   `json:"guest_redirection"`
	// FilterEntries hides matching names until the user asks to see them.
	FilterEntries []string `json:"filter_entries"`
	// MaxWidth caps the interface width in pixels; 0 means no cap.
	MaxWidth int `json:"max_width"`
	// ConfigReadOnly warns the admin UI that changes will not be saved.
	ConfigReadOnly bool `json:"config_read_only"`
	// StartupNotes describes what this start created or could not resolve.
	// Only administrators are meant to act on them; they are part of this
	// payload rather than an admin-only request because the page is already
	// loading it, and they mention paths, never secrets.
	StartupNotes []StartupNote `json:"startup_notes,omitempty"`
}

// serveConfig returns the public frontend configuration.
//
// The logo pair is resolved here rather than sent as stored, so a fresh install
// and a customised one are the same shape to the browser: both slots have
// something to show.
func (a *App) serveConfig(w http.ResponseWriter, r *http.Request, _ router.Params) {
	f := a.config.Get().Frontend
	logo := resolveLogo(f).underBasePath(a.router.BasePath())
	api.WriteJSON(w, http.StatusOK, PublicConfig{
		AppName:            f.AppName,
		Language:           f.Language,
		LanguageSelection:  f.LanguageSelection,
		Logo:               logo.Logo,
		LogoDark:           logo.LogoDark,
		UploadMaxSize:      f.UploadMaxSize,
		UploadChunkSize:    f.UploadChunkSize,
		UploadSimultaneous: f.UploadSimultaneous,
		DefaultArchiveName: f.DefaultArchiveName,
		Editable:           f.Editable,
		DateFormat:         f.DateFormat,
		GuestRedirection:   f.GuestRedirection,
		FilterEntries:      f.FilterEntries,
		MaxWidth:           f.MaxWidth,
		ConfigReadOnly:     a.config.ReadOnly(),
		StartupNotes:       a.startup.Notes(),
	})
}
