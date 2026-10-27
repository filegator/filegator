package app

import (
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// translationsDirName is where language files live inside the embedded
// frontend, matching translationsDir in i18n.go.
const translationsDirName = "translations"

// englishFile is the reference every other language is checked against.
const englishFile = translationsDirName + "/english.json"

// placeholderPattern matches the {0}, {1} ... substitution markers a
// translation may contain.
var placeholderPattern = regexp.MustCompile(`\{\d+\}`)

// readTranslations decodes every language file of the embedded frontend.
func readTranslations(t *testing.T) map[string]map[string]string {
	t.Helper()
	root := frontendRoot()
	entries, err := fs.ReadDir(root, translationsDirName)
	if err != nil {
		t.Fatalf("read %s: %v", translationsDirName, err)
	}
	out := make(map[string]map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		data, err := fs.ReadFile(root, path.Join(translationsDirName, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			t.Errorf("%s is not valid JSON: %v", entry.Name(), err)
			continue
		}
		out[name] = messages
	}
	if len(out) < 2 {
		t.Fatalf("found %d language files; expected the translations to be embedded", len(out))
	}
	return out
}

// TestLanguagesMatchEnglish fails when a language file has a key English does
// not, or is missing one it does. A missing key falls back to English in the
// browser, so without this test a language silently drifts behind and nobody
// notices until someone reads that screen in it.
func TestLanguagesMatchEnglish(t *testing.T) {
	files := readTranslations(t)
	english, ok := files["english"]
	if !ok {
		t.Fatal("english.json is missing")
	}

	for lang, messages := range files {
		if lang == "english" {
			continue
		}
		var missing, extra []string
		for key := range english {
			if _, ok := messages[key]; !ok {
				missing = append(missing, key)
			}
		}
		for key := range messages {
			if _, ok := english[key]; !ok {
				extra = append(extra, key)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			t.Errorf("%s: %d keys missing from English, e.g. %q", lang, len(missing), missing[:min(3, len(missing))])
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			t.Errorf("%s: %d keys not in English, e.g. %q", lang, len(extra), extra[:min(3, len(extra))])
		}
	}
}

// TestTranslationPlaceholders keeps the {0}, {1} markers a message needs. A
// translation that drops one prints a sentence with a missing value; one that
// adds one is allowed, because upload_panel.js fills "Upload failed" with a
// file name through the placeholder v7 already had.
func TestTranslationPlaceholders(t *testing.T) {
	files := readTranslations(t)
	english := files["english"]

	for lang, messages := range files {
		for key, translated := range messages {
			source, ok := english[key]
			if !ok {
				continue
			}
			for _, marker := range placeholderPattern.FindAllString(source, -1) {
				if !strings.Contains(translated, marker) {
					t.Errorf("%s: %q keeps %s in English but drops it in the translation", lang, key, marker)
				}
			}
		}
	}
}

// TestTranslationFilesAreNotEmpty guards against a truncated or half-written
// file: an empty or near-empty language would silently show English keys.
func TestTranslationFilesAreNotEmpty(t *testing.T) {
	files := readTranslations(t)
	english := files["english"]
	if len(english) == 0 {
		t.Fatal("english.json has no keys")
	}
	for lang, messages := range files {
		if len(messages) == 0 {
			t.Errorf("%s.json is empty", lang)
		}
		empty := 0
		for _, value := range messages {
			if strings.TrimSpace(value) == "" {
				empty++
			}
		}
		if empty > 0 {
			t.Errorf("%s.json has %d blank translations", lang, empty)
		}
	}
}

// translatedKeysPattern matches t("...") and t('...') calls, the form the
// frontend uses for every visible string.
var translatedKeysPattern = regexp.MustCompile(`\bt\(\s*(?:"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)')`)

// TestTranslatedKeysExist checks that every key the JavaScript asks for is in
// english.json. This is the direction that was missing when English itself fell
// 92 keys behind the code: the page still looked right, because a missing key
// prints as the key, so nothing failed.
func TestTranslatedKeysExist(t *testing.T) {
	files := readTranslations(t)
	english := files["english"]

	var used []string
	err := fs.WalkDir(frontendRoot(), "js", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".js") {
			return err
		}
		data, readErr := fs.ReadFile(frontendRoot(), p)
		if readErr != nil {
			return readErr
		}
		for _, match := range translatedKeysPattern.FindAllStringSubmatch(string(data), -1) {
			key := match[1]
			if key == "" {
				key = match[2]
			}
			key = strings.ReplaceAll(key, `\"`, `"`)
			key = strings.ReplaceAll(key, `\'`, `'`)
			used = append(used, key)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk js: %v", err)
	}
	if len(used) < 100 {
		t.Fatalf("found only %d t() calls; the walk is broken", len(used))
	}

	for _, key := range used {
		if _, ok := english[key]; !ok {
			t.Errorf("the JavaScript calls t(%q), which english.json does not have", key)
		}
	}
}
