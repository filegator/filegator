package app

import (
	"os"
	"sync"

	"github.com/filegator/filegator/internal/config"
)

// Startup notes
//
// Installing FileGator is mostly "start the binary somewhere". The first start
// therefore creates things the operator never asked for: private/, a users file
// with published default passwords, a storage root, and later a config file
// carrying the session key. The log says so once, at a moment nobody is
// watching, and the symptom of a wrong -config path (an instance running on
// built-in defaults, with everything resolved relative to the working
// directory) looks like a bug rather than a wrong invocation.
//
// These notes record what happened during this start so an administrator can
// see it in the interface, where the decision to keep or fix it is made. They
// describe the current start only: they are built while the process starts and
// are never written anywhere.
type StartupNote struct {
	// Kind is "info" for something that was created for the operator, and
	// "warning" for something a fresh or misconfigured install should look at.
	Kind string `json:"kind"`
	// Key is a stable name for the condition, so a client could act on it
	// without matching English text.
	Key string `json:"key"`
	// Text is the operator-facing sentence, including the paths involved.
	Text string `json:"text"`
}

const (
	noteInfo    = "info"
	noteWarning = "warning"
)

// StartupLog collects notes while the application is being built.
//
// A nil *StartupLog is usable and records nothing, so a caller that builds part
// of the application outside New (tests, the admin API fixtures) needs no setup.
type StartupLog struct {
	mu    sync.Mutex
	notes []StartupNote
}

// Add records a note. Later callers cannot modify it.
func (l *StartupLog) Add(note StartupNote) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.notes = append(l.notes, note)
}

func (l *StartupLog) add(kind, key, text string) {
	l.Add(StartupNote{Kind: kind, Key: key, Text: text})
}

// Notes returns a copy of the notes recorded so far.
func (l *StartupLog) Notes() []StartupNote {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]StartupNote, len(l.notes))
	copy(out, l.notes)
	return out
}

// removeKey drops every note with the given key.
//
// Used when a later step resolves something an earlier one reported: a missing
// configuration file that is then created should be shown as "created", not as
// "missing" followed by "created".
func (l *StartupLog) removeKey(key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.notes[:0]
	for _, note := range l.notes {
		if note.Key != key {
			kept = append(kept, note)
		}
	}
	l.notes = kept
}

// fileExists reports whether path names anything at all.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readOnlyReason explains why the configuration cannot be persisted.
//
// The store's own error only says "read-only"; that sent operators looking for
// a read-only mount when the cause was a folder that could not be created.
func readOnlyReason(store *config.Store) string {
	if why := store.Why(); why != "" {
		return why
	}
	return config.ErrReadOnly.Error()
}

// RecordConfigSource notes how the configuration file was found.
//
// A missing file is not an error, but it is worth saying out loud: it is what
// a wrong -config path looks like, and it explains why the instance shows the
// built-in app name and the default repository. The read-only note says why,
// not just that it is read-only.
func (l *StartupLog) RecordConfigSource(store *config.Store) {
	path := store.Path()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			l.add(noteInfo, "config.missing",
				"No configuration file at "+path+"; the built-in defaults are in use. "+
					"It is created as soon as a setting is saved, or on the first start when the folder can be written.")
		} else {
			l.add(noteWarning, "config.unreadable",
				"The configuration file "+path+" cannot be read ("+err.Error()+"); the built-in defaults are in use.")
		}
	}
	if why := store.Why(); why != "" {
		l.add(noteWarning, "config.read_only",
			"The configuration in "+path+" cannot be written ("+why+"): settings apply to this run only and are lost on restart.")
	}
}
