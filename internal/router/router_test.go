package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func do(rt *Router, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	return rec
}

func TestExactMatch(t *testing.T) {
	rt := New()
	rt.Get("/hello", func(w http.ResponseWriter, r *http.Request, p Params) {
		w.Write([]byte("hi"))
	})
	rec := do(rt, "GET", "/hello")
	if rec.Code != 200 || rec.Body.String() != "hi" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestTrailingSlashIsEquivalent(t *testing.T) {
	rt := New()
	rt.Get("/hello", func(w http.ResponseWriter, r *http.Request, p Params) {})
	if rec := do(rt, "GET", "/hello/"); rec.Code != 200 {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}

func TestRootRoute(t *testing.T) {
	rt := New()
	rt.Get("/", func(w http.ResponseWriter, r *http.Request, p Params) {})
	if rec := do(rt, "GET", "/"); rec.Code != 200 {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}

func TestPathParam(t *testing.T) {
	rt := New()
	var got Params
	rt.Post("/updateuser/{username}", func(w http.ResponseWriter, r *http.Request, p Params) {
		got = p
	})
	rec := do(rt, "POST", "/updateuser/alice")
	if rec.Code != 200 {
		t.Fatalf("got %d", rec.Code)
	}
	if got["username"] != "alice" {
		t.Fatalf("username = %q", got["username"])
	}
}

func TestNotFound(t *testing.T) {
	rt := New()
	rt.Get("/hello", func(w http.ResponseWriter, r *http.Request, p Params) {})
	if rec := do(rt, "GET", "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rt := New()
	rt.Get("/hello", func(w http.ResponseWriter, r *http.Request, p Params) {})
	if rec := do(rt, "POST", "/hello"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rec.Code)
	}
}

func TestMiddlewareOrder(t *testing.T) {
	rt := New()
	var order []string
	rt.Use(func(next Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request, p Params) {
			order = append(order, "outer-before")
			next(w, r, p)
			order = append(order, "outer-after")
		}
	})
	rt.Use(func(next Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request, p Params) {
			order = append(order, "inner")
			next(w, r, p)
		}
	})
	rt.Get("/x", func(w http.ResponseWriter, r *http.Request, p Params) {
		order = append(order, "handler")
	})
	do(rt, "GET", "/x")

	want := []string{"outer-before", "inner", "handler", "outer-after"}
	if len(order) != len(want) {
		t.Fatalf("order = %v", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestEmptyParamNamePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	New().Get("/x/{}", func(w http.ResponseWriter, r *http.Request, p Params) {})
}

func TestBasePathStripsThePrefix(t *testing.T) {
	rt := New()
	rt.SetBasePath("/browser")
	var got string
	rt.Get("/api/ping", func(w http.ResponseWriter, r *http.Request, p Params) {
		// Handlers see the path without the mount point.
		got = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/browser/api/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if got != "/api/ping" {
		t.Errorf("handler saw %q, want /api/ping", got)
	}
}

func TestBasePathRejectsPathsOutsideTheMount(t *testing.T) {
	rt := New()
	rt.SetBasePath("/browser")
	rt.Get("/api/ping", func(w http.ResponseWriter, r *http.Request, p Params) {})

	for _, path := range []string{"/api/ping", "/", "/browserX/api/ping"} {
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}

func TestBasePathNormalisation(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"/":         "",
		"browser":   "/browser",
		"/browser":  "/browser",
		"/browser/": "/browser",
		"/a/b/":     "/a/b",
	}
	for in, want := range cases {
		rt := New()
		rt.SetBasePath(in)
		if got := rt.BasePath(); got != want {
			t.Errorf("SetBasePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRootBasePathIsUnchanged(t *testing.T) {
	rt := New()
	rt.SetBasePath("/")
	rt.Get("/api/ping", func(w http.ResponseWriter, r *http.Request, p Params) {})
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestValidateBasePath(t *testing.T) {
	for _, in := range []string{"", "/", "/browser", "browser", "/a/b/"} {
		if err := ValidateBasePath(in); err != nil {
			t.Errorf("ValidateBasePath(%q) rejected: %v", in, err)
		}
	}
	// "browser" and "/a/b/" are normalised rather than rejected: they mean the
	// same prefix someone would type on a command line.
	if got := NormalizeBasePath("browser/"); got != "/browser" {
		t.Errorf("NormalizeBasePath = %q", got)
	}
	if got := NormalizeBasePath("/a/b/"); got != "/a/b" {
		t.Errorf("NormalizeBasePath = %q", got)
	}
	for _, bad := range []string{"/a//b", "/a/../b", "/a?b"} {
		if err := ValidateBasePath(bad); err == nil {
			t.Errorf("ValidateBasePath(%q) accepted an unusable prefix", bad)
		}
	}
}
