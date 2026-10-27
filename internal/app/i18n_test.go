package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

func TestListLanguages(t *testing.T) {
	rec := get(newTestApp(t), "/api/i18n")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var envelope struct {
		Data LanguageList `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	langs := envelope.Data.Languages

	if len(langs) < 30 {
		t.Fatalf("got %d languages, want all v7 languages", len(langs))
	}
	for _, want := range []string{"english", "german", "portuguese_br", "japanese", "arabic"} {
		if !contains(langs, want) {
			t.Errorf("missing language %q", want)
		}
	}
	// Sorted.
	for i := 1; i < len(langs); i++ {
		if langs[i-1] > langs[i] {
			t.Errorf("not sorted at %d: %q > %q", i, langs[i-1], langs[i])
		}
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func TestServeLanguage(t *testing.T) {
	rec := get(newTestApp(t), "/api/i18n/english")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body)
	}
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The administration area is English only, so its keys are gone; what is
	// left is the interface a user sees, which is well over a hundred strings.
	if len(envelope.Data) < 100 {
		t.Errorf("got %d messages", len(envelope.Data))
	}
	// A key the file browser shows, not an admin one.
	if envelope.Data["Download"] != "Download" {
		t.Errorf("Download = %q", envelope.Data["Download"])
	}
}

func TestServeLanguageEscapedQuote(t *testing.T) {
	rec := get(newTestApp(t), "/api/i18n/french")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// The fixture is an apostrophe inside a value, which must survive JSON
	// escaping. Dutch lost its last one when the administration keys were
	// removed, so this uses French "Upload failed", which the upload panel
	// shows.
	if !strings.Contains(rec.Body.String(), `d'envoi`) {
		t.Errorf("escaped apostrophe not preserved: %s", rec.Body)
	}
}

func TestServeLanguageUnicode(t *testing.T) {
	rec := get(newTestApp(t), "/api/i18n/japanese")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ファイル") {
		t.Errorf("UTF-8 not preserved: %s", rec.Body)
	}
}

func TestServeLanguageNotFound(t *testing.T) {
	rec := get(newTestApp(t), "/api/i18n/klingon")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestServeLanguageRejectsTraversal(t *testing.T) {
	for _, lang := range []string{"..%2f..%2findex", "ENGLISH", "english.json", "a-b", "a.b"} {
		rec := get(newTestApp(t), "/api/i18n/"+lang)
		if rec.Code == http.StatusOK {
			t.Errorf("%q was served", lang)
		}
	}
}

func TestValidLanguage(t *testing.T) {
	for _, ok := range []string{"english", "portuguese_br", "zh_cn", "a1"} {
		if !config.ValidLanguage(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "English", "../x", "a/b", "a b", strings.Repeat("x", 40)} {
		if config.ValidLanguage(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLanguageListReportsDefault(t *testing.T) {
	app := newTestApp(t)
	if err := app.config.Update(func(c *config.Config) error {
		c.Frontend.Language = "german"
		c.Frontend.LanguageSelection = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		Data LanguageList `json:"data"`
	}
	rec := get(app, "/api/i18n")
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.Default != "german" {
		t.Errorf("default = %q, want german", envelope.Data.Default)
	}
	if envelope.Data.Selectable {
		t.Error("selectable should follow the config")
	}
	if !contains(envelope.Data.Languages, "german") {
		t.Error("german missing from languages")
	}
}

func TestLanguageListFallsBackForMissingFile(t *testing.T) {
	app := newTestApp(t)
	if err := app.config.Update(func(c *config.Config) error {
		c.Frontend.Language = "klingon"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		Data LanguageList `json:"data"`
	}
	rec := get(app, "/api/i18n")
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Default == "klingon" {
		t.Error("default should fall back when no translation file exists")
	}
	if !contains(envelope.Data.Languages, envelope.Data.Default) {
		t.Errorf("fallback %q is not an available language", envelope.Data.Default)
	}
}

func TestLanguageSelectionDefault(t *testing.T) {
	if !config.Default().Frontend.LanguageSelection {
		t.Error("language_selection should default to true, matching v7")
	}
}
