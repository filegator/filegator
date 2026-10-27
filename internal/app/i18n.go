package app

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/filegator/filegator/internal/api"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/router"
)

// translationsDir is where the language files live inside the embedded
// frontend.
const translationsDir = "translations"

// listLanguages returns the available language codes together with the
// administrator's configured default, so the frontend can pick a language
// without an extra request.
func (a *App) listLanguages(w http.ResponseWriter, r *http.Request, _ router.Params) {
	entries, err := fs.ReadDir(a.frontend, translationsDir)
	var langs []string
	if err == nil {
		langs = make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			langs = append(langs, strings.TrimSuffix(entry.Name(), ".json"))
		}
		sort.Strings(langs)
	}

	cfg := a.config.Get().Frontend
	defaultLang := cfg.Language
	if !containsString(langs, defaultLang) && len(langs) > 0 {
		// A configured language with no translation file falls back to the
		// first available one rather than breaking the UI.
		defaultLang = langs[0]
	}

	api.WriteJSON(w, http.StatusOK, LanguageList{
		Languages:  langs,
		Default:    defaultLang,
		Selectable: cfg.LanguageSelection,
	})
}

// LanguageList is the response for GET /api/i18n.
type LanguageList struct {
	// Languages are the available language codes, sorted.
	Languages []string `json:"languages"`
	// Default is the administrator-configured language.
	Default string `json:"default"`
	// Selectable reports whether users may choose their own language.
	Selectable bool `json:"selectable"`
}

// containsString reports whether list contains want.
func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// serveLanguage returns one language file. The language name is restricted to
// a safe character set so it cannot be used to read arbitrary embedded files.
func (a *App) serveLanguage(w http.ResponseWriter, r *http.Request, p router.Params) {
	lang := p["lang"]
	if !config.ValidLanguage(lang) {
		api.WriteError(w, api.Errorf(http.StatusBadRequest, api.CodeBadRequest, "invalid language"))
		return
	}
	data, err := fs.ReadFile(a.frontend, path.Join(translationsDir, lang+".json"))
	if err != nil {
		api.WriteError(w, api.Errorf(http.StatusNotFound, api.CodeNotFound, "language not found"))
		return
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		api.WriteError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, messages)
}
