// Package router provides a small, dependency-free HTTP router.
//
// Routes are registered with an HTTP method, a pattern and a handler. Patterns
// may contain named path segments such as "/updateuser/{username}". The router
// distinguishes "not found" (no pattern matched) from "method not allowed"
// (pattern matched, method did not) so handlers can return correct statuses.
package router

import (
	"fmt"
	"net/http"
	"strings"
)

// Params holds decoded path parameters for a request.
type Params map[string]string

// Handler is an HTTP handler that also receives path parameters.
type Handler func(w http.ResponseWriter, r *http.Request, p Params)

// Middleware wraps a Handler.
type Middleware func(Handler) Handler

// Route is a single registered route.
type Route struct {
	Method  string
	pattern []segment
	handler Handler
}

type segment struct {
	literal string
	param   string // non-empty when this is a named parameter
}

// Router matches requests against registered routes.
type Router struct {
	routes     []Route
	middleware []Middleware
	// basePath is a prefix such as "/browser" that every request carries and
	// every pattern omits. Stripping it here means routes are registered
	// without knowing where the application is mounted.
	basePath string
}

// New creates an empty router.
func New() *Router {
	return &Router{}
}

// SetBasePath mounts the router under prefix, e.g. "/browser".
//
// An empty prefix or "/" means the site root. The prefix is normalised to
// start with a slash and never end with one, so "/browser/", "browser" and
// "/browser" are all the same.
func (rt *Router) SetBasePath(prefix string) {
	p := strings.TrimSpace(prefix)
	if p == "" || p == "/" {
		rt.basePath = ""
		return
	}
	rt.basePath = "/" + strings.Trim(p, "/")
}

// BasePath returns the mounted prefix, or "" for the site root.
func (rt *Router) BasePath() string { return rt.basePath }

// Routes returns a copy of the registered routes in registration order, with
// each pattern written back out as text and the base path omitted — the form
// the routes are declared in.
//
// It exists so documentation can be generated from the router rather than
// maintained beside it, which is the list that would otherwise drift.
func (rt *Router) Routes() []Route {
	out := make([]Route, len(rt.routes))
	for i, r := range rt.routes {
		out[i] = Route{Method: r.Method, pattern: append([]segment(nil), r.pattern...)}
	}
	return out
}

// Pattern returns the route's pattern as registered, e.g. "/api/i18n/{lang}".
func (r Route) Pattern() string {
	if len(r.pattern) == 0 {
		return "/"
	}
	var b strings.Builder
	for _, s := range r.pattern {
		b.WriteByte('/')
		switch {
		case s.param != "":
			b.WriteByte('{')
			b.WriteString(s.param)
			b.WriteByte('}')
		default:
			b.WriteString(s.literal)
		}
	}
	return b.String()
}

// ValidateBasePath checks a URL prefix such as "/browser".
//
// A valid prefix starts with a slash, has no trailing slash and contains no
// empty, "." or ".." segments, so it can be mounted and stripped safely.
// The value is normalised first, so "browser" and "/a/b/" are accepted and
// mean the same as "/browser".
func ValidateBasePath(raw string) error {
	p := NormalizeBasePath(raw)
	if p == "" {
		return nil
	}
	for _, seg := range strings.Split(p[1:], "/") {
		switch seg {
		case "":
			return fmt.Errorf("base path %q contains an empty segment", p)
		case ".", "..":
			return fmt.Errorf("base path %q contains %q", p, seg)
		}
	}
	if strings.ContainsAny(p, "?#") {
		return fmt.Errorf("base path %q must be a plain path", p)
	}
	if strings.Contains(raw, "//") {
		return fmt.Errorf("base path %q contains an empty segment", raw)
	}
	return nil
}

// NormalizeBasePath returns the canonical form used at runtime: "" for the
// site root, otherwise a leading slash and no trailing one.
func NormalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || strings.Trim(p, "/") == "" {
		return ""
	}
	return "/" + strings.Trim(p, "/")
}

// Use appends middleware. Middleware runs in the order it was added.
func (rt *Router) Use(m ...Middleware) {
	rt.middleware = append(rt.middleware, m...)
}

// Handle registers a route. Panics on a malformed pattern.
func (rt *Router) Handle(method, pattern string, h Handler) {
	rt.routes = append(rt.routes, Route{
		Method:  strings.ToUpper(method),
		pattern: parsePattern(pattern),
		handler: h,
	})
}

// Get registers a GET route.
func (rt *Router) Get(pattern string, h Handler) { rt.Handle(http.MethodGet, pattern, h) }

// Post registers a POST route.
func (rt *Router) Post(pattern string, h Handler) { rt.Handle(http.MethodPost, pattern, h) }

// ServeHTTP implements http.Handler.
func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rt.basePath != "" {
		rest, ok := strings.CutPrefix(r.URL.Path, rt.basePath)
		if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
			// Outside the mount point. The caller (usually a proxy) sent us a
			// path we do not own, so answer plainly rather than guessing.
			http.NotFound(w, r)
			return
		}
		// Hand handlers a path relative to the mount so they never need to
		// know the prefix.
		r2 := r.Clone(r.Context())
		r2.URL.Path = rest
		if r2.URL.Path == "" {
			r2.URL.Path = "/"
		}
		r = r2
	}
	path := splitPath(r.URL.Path)
	var methodMismatch bool

	for _, route := range rt.routes {
		params, ok := match(route.pattern, path)
		if !ok {
			continue
		}
		if route.Method != r.Method {
			methodMismatch = true
			continue
		}
		rt.dispatch(w, r, route.handler, params)
		return
	}

	if methodMismatch {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	http.NotFound(w, r)
}

func (rt *Router) dispatch(w http.ResponseWriter, r *http.Request, h Handler, p Params) {
	for i := len(rt.middleware) - 1; i >= 0; i-- {
		h = rt.middleware[i](h)
	}
	h(w, r, p)
}

func parsePattern(pattern string) []segment {
	parts := splitPath(pattern)
	segs := make([]segment, 0, len(parts))
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := part[1 : len(part)-1]
			if name == "" {
				panic("router: empty parameter name in pattern " + pattern)
			}
			segs = append(segs, segment{param: name})
			continue
		}
		segs = append(segs, segment{literal: part})
	}
	return segs
}

// splitPath splits a URL path into non-empty segments. A trailing slash is
// ignored, so "/foo/" and "/foo" are equivalent.
func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func match(pattern []segment, path []string) (Params, bool) {
	if len(pattern) != len(path) {
		return nil, false
	}
	var params Params
	for i, seg := range pattern {
		if seg.param != "" {
			if params == nil {
				params = make(Params)
			}
			params[seg.param] = path[i]
			continue
		}
		if seg.literal != path[i] {
			return nil, false
		}
	}
	return params, true
}
