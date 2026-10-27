// Package search walks a storage tree looking for entries whose name contains
// a term.
//
// It is a plain depth-first walk over storage.FS rather than an index, because
// the storage backends range from a local disk to object stores with no real
// directory structure. An index would have to be kept in sync with every
// backend and every writer outside the application, so searching live data is
// both simpler and more trustworthy.
package search

import (
	"context"
	"errors"
	"strings"

	"github.com/filegator/filegator/internal/storage"
)

// Errors returned by this package.
var (
	// ErrEmptyTerm is returned when no search term was supplied.
	ErrEmptyTerm = errors.New("search: the term must not be empty")
	// ErrLimited is returned when the walk stopped at the configured limits.
	// Results collected up to that point are still returned.
	ErrLimited = errors.New("search: the result limit was reached")
)

// Options tune a search.
type Options struct {
	// Limit caps how many results are returned. Zero means Unlimited.
	Limit int
	// MaxDepth stops the walk after this many directory levels below the
	// root. Zero means no limit.
	MaxDepth int
}

// Unlimited may be used as Options.Limit for no cap at all. It exists so a
// caller can be explicit rather than relying on the magic zero.
const Unlimited = -1

// Result is one matching entry.
type Result struct {
	// Entry is the matched file or directory.
	Entry *storage.Entry
	// Parent is the folder containing it, useful for jumping to the match.
	Parent string
}

// Names returns every entry under root whose name contains term, ignoring
// case.
//
// The walk is depth-first and aborts as soon as ctx is cancelled, which is how
// an HTTP handler stops working when the client goes away. Hidden entries are
// included: a file the user can see in a listing should be findable.
func Names(ctx context.Context, fsys storage.FS, root, term string, opts Options) ([]Result, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, ErrEmptyTerm
	}
	needle := strings.ToLower(term)

	matched, err := walk(ctx, fsys, root, needle, 0, opts)
	if err != nil && !errors.Is(err, ErrLimited) {
		return nil, err
	}
	return matched, err
}

// walk visits one directory and recurses into its subdirectories.
func walk(ctx context.Context, fsys storage.FS, dir, needle string, depth int, opts Options) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := fsys.ReadDir(dir)
	if err != nil {
		// A folder that vanished mid-walk or cannot be read (permissions) is
		// skipped rather than failing the whole search: a partial result is
		// more useful than an error, and the walk would otherwise be at the
		// mercy of one unreadable folder.
		if errors.Is(err, storage.ErrPermission) || errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var results []Result
	for _, entry := range entries {
		if opts.Limit > 0 && len(results) >= opts.Limit {
			return results, ErrLimited
		}
		if err := ctx.Err(); err != nil {
			return results, err
		}

		if strings.Contains(strings.ToLower(entry.Name), needle) {
			results = append(results, Result{Entry: entry, Parent: dir})
		}

		if !entry.IsDir {
			continue
		}
		if opts.MaxDepth > 0 && depth+1 >= opts.MaxDepth {
			continue
		}
		sub, err := walk(ctx, fsys, entry.Path, needle, depth+1, opts)
		results = append(results, sub...)
		if errors.Is(err, ErrLimited) {
			return results, err
		}
		if err != nil && !errors.Is(err, storage.ErrPermission) {
			return results, err
		}
	}
	return results, nil
}

// IsDir reports whether the entry is a directory. It exists so callers do not
// reach into the entry for a one-liner and keeps the package's intent readable.
func (r Result) IsDir() bool {
	return r.Entry != nil && r.Entry.IsDir
}
