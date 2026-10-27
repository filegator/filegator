package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/router"
	"github.com/filegator/filegator/internal/search"
)

// SearchResult is one entry returned by a search.
type SearchResult struct {
	Entry
	// Parent is the folder containing the match, so the UI can jump to it.
	Parent string `json:"parent"`
}

// SearchResponse is the body of GET /api/files/search.
type SearchResponse struct {
	Term    string         `json:"term"`
	Path    string         `json:"path"`
	Results []SearchResult `json:"results"`
	// Limited reports that the search stopped at the result cap, so the UI can
	// ask for a narrower term instead of silently hiding matches.
	Limited bool `json:"limited"`
}

// searchLimit bounds how many matches a single search returns.
//
// v7 walked the tree from the browser five folders at a time and had no cap at
// all, which stalled on large repositories. Walking on the server needs a
// bound so one request cannot hold a worker and the client for minutes.
const searchLimit = 500

// searchTimeout bounds how long a search may run.
const searchTimeout = 30 * time.Second

// Search handles GET /api/files/search?path=/&q=term.
//
// The walk runs on the server against live storage, rather than in the browser
// against cached folder listings the way v7 did.
func (s *Server) Search(w http.ResponseWriter, r *http.Request, u *auth.User) {
	fsys, err := s.requirePermission(u, auth.PermRead)
	if err != nil {
		WriteError(w, err)
		return
	}

	term := strings.TrimSpace(r.URL.Query().Get("q"))
	if term == "" {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "q is required"))
		return
	}

	root := r.URL.Query().Get("path")
	if root == "" {
		root = "/"
	}
	if entry, err := fsys.Stat(root); err != nil {
		WriteError(w, mapStorageError(err))
		return
	} else if !entry.IsDir {
		WriteError(w, Errorf(http.StatusBadRequest, CodeBadRequest, "path must be a directory"))
		return
	}

	// The search must not outlive the request: a cancelled context stops the
	// walk where it is instead of walking the whole tree for nobody.
	ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
	defer cancel()

	matches, err := search.Names(ctx, fsys, root, term, search.Options{Limit: searchLimit})
	switch {
	case err == nil, errors.Is(err, search.ErrLimited):
		// A capped search still returns what it found.
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		WriteError(w, Errorf(http.StatusRequestTimeout, CodeBadRequest,
			"the search took too long, try a narrower term"))
		return
	default:
		WriteError(w, mapStorageError(err))
		return
	}

	results := make([]SearchResult, 0, len(matches))
	for _, m := range matches {
		results = append(results, SearchResult{Entry: toEntry(m.Entry), Parent: m.Parent})
	}

	WriteJSON(w, http.StatusOK, SearchResponse{
		Term:    term,
		Path:    root,
		Results: results,
		Limited: errors.Is(err, search.ErrLimited),
	})
}

// SearchRoutes registers the search endpoint.
func (s *Server) SearchRoutes(rt *router.Router) {
	rt.Get("/api/files/search", s.withUser(s.Search))
}
