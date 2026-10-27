package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// docsExample is the "Complete example" block of docs/content/configuration.md,
// the reference an operator copies and edits by hand.
//
// There is deliberately no config.json.sample in the repository, the way there
// is a users.json.sample: the file is created on the first start with a
// generated session key, and a shipped sample would either carry a key that is
// identical on every install or become a second, silently stale description of
// the defaults. The docs block is the reference instead, so it has to stay
// true — which is what the tests below are for.
const docsExample = "../../docs/content/configuration.md"

// backendSections are the subtrees the example shows as {} on purpose.
//
// Their options are documented where they are chosen (storage.md,
// authentication.md, database.md), not on the configuration page: a value they
// duplicate is a value that goes stale in two places at once. The example still
// has to name each subtree, which TestDocsExampleShowsEveryBackend checks.
var backendSections = map[string][]string{
	"storage": {"sftp", "s3"},
	"auth":    {"ldap", "sql"},
}

// TestDocsExampleMatchesDefaults pins every documented value to Default().
//
// The page says the block is "the file after every setting has been saved at
// least once", so each value in it must be the one a fresh install uses.
// Without this test the two drift apart and the operator who copies the block
// is shown settings the binary no longer has.
func TestDocsExampleMatchesDefaults(t *testing.T) {
	documented := exampleValues(t)
	want := exampleValuesOf(t, Default())

	var diff []string
	for path, w := range want {
		// The page documents the session key as generated, so it writes "".
		if path == "security.session_key" {
			continue
		}
		g, ok := documented[path]
		if !ok {
			// Omissions are TestDocsExampleCoversEveryDefault's business.
			continue
		}
		if isEmpty(w) {
			// An empty default means "unset"; writing it out or leaving it
			// for the default to fill in reads the same.
			continue
		}
		if !reflect.DeepEqual(w, g) {
			diff = append(diff, path+": the page says "+jsonText(t, g)+
				", the default is "+jsonText(t, w))
		}
	}
	for _, d := range diff {
		t.Error(d)
	}
}

// TestDocsExampleCoversEveryDefault is the other direction: a key Default()
// sets that the page never shows cannot be changed by copying the block.
//
// The subtrees in backendSections are exempt, and so is a default of zero — it
// is the value the reader gets by leaving the key out, and the page documents
// it as "0 means the default".
func TestDocsExampleCoversEveryDefault(t *testing.T) {
	documented := exampleValues(t)
	for path, w := range exampleValuesOf(t, Default()) {
		if path == "security.session_key" || isEmpty(w) || inBackendSection(path) {
			continue
		}
		if _, ok := documented[path]; !ok {
			t.Errorf("default %s is not in the complete example; an operator copying the block never sees it", path)
		}
	}
}

// TestDocsExampleShowsEveryBackend keeps the exemption above from hiding a
// backend: the example must still show storage.sftp, auth.ldap and the rest as
// keys, even when their contents are left on the page that explains them.
func TestDocsExampleShowsEveryBackend(t *testing.T) {
	documented := exampleValues(t)
	for section, names := range backendSections {
		for _, name := range names {
			path := section + "." + name
			if _, ok := documented[path]; !ok {
				t.Errorf("the complete example does not show %s at all; it may be empty, but a backend missing from the block cannot be discovered", path)
			}
		}
	}
}

func inBackendSection(path string) bool {
	for section, names := range backendSections {
		for _, name := range names {
			if strings.HasPrefix(path, section+"."+name+".") {
				return true
			}
		}
	}
	return false
}

// exampleValues reads the ```json block under the page's "Complete example"
// heading and flattens it to "storage.root"-style paths.
func exampleValues(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(docsExample))
	if err != nil {
		t.Fatalf("read %s: %v", docsExample, err)
	}
	lines := strings.Split(string(data), "\n")

	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "## Complete example" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("%s: no \"## Complete example\" heading; the page this test guards was moved", docsExample)
	}

	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```json" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) != "```" {
				continue
			}
			var v map[string]any
			body := strings.Join(lines[i+1:j], "\n")
			if err := json.Unmarshal([]byte(body), &v); err != nil {
				t.Fatalf("%s: the complete example is not valid JSON: %v", docsExample, err)
			}
			return flatten(t, v)
		}
		t.Fatalf("%s: the complete example block is never closed", docsExample)
	}
	t.Fatalf("%s: \"Complete example\" has no json block", docsExample)
	return nil
}

// exampleValuesOf is the same flattening for a configuration, read through
// JSON so the field names and types are the ones an operator sees in the file.
//
// An object that flattens to nothing — "s3": {} — records its own path as a
// marker, so TestDocsExampleShowsEveryBackend can tell "written as {}" from
// "absent".
func exampleValuesOf(t *testing.T, c *Config) map[string]any {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal defaults: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal defaults: %v", err)
	}
	return flatten(t, v)
}

// flatten turns nested objects into dotted paths; arrays stay whole, because
// an entry of one is not a setting an operator sets by name.
func flatten(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	out := map[string]any{}
	var walk func(prefix string, v map[string]any)
	walk = func(prefix string, v map[string]any) {
		for k, child := range v {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			nested, ok := child.(map[string]any)
			if !ok {
				out[key] = child
				continue
			}
			before := len(out)
			walk(key, nested)
			if len(out) == before {
				// An empty object is still a key the reader has to know about.
				out[key] = nil
			}
		}
	}
	walk("", v)
	return out
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case float64:
		// Zero is what an absent key yields; the page documents it as
		// "0 means the default" rather than writing it out.
		return x == 0
	}
	return false
}

func jsonText(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %v: %v", v, err)
	}
	return string(raw)
}
