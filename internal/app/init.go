package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// First start
//
// A server started in an empty folder needs private/, a configuration file with
// a session key, a storage root and a users file. Creating them silently is what
// makes a wrong -config path look like a working install: the operator gets a
// configured instance they never asked for, on whatever the working directory
// happened to be.
//
// So the creation is a decision, not a side effect — and a refusal is final:
//
//   - InitAsk prompts on the terminal, listing every path first; answering no
//     writes nothing and the caller refuses to start.
//   - InitYes (-init) creates everything without asking, for scripts, containers
//     and service units, which have no terminal to answer on.
//
// InitAsk without a terminal refuses rather than guessing: a prompt nobody can
// see would block a service unit forever.
//
// The question is only asked when there is something to create. An installation
// with its config, users file and storage root in place never sees it.

// InitMode is how the first start is handled when files have to be created.
type InitMode int

const (
	// InitAsk is the default: ask on the terminal, refuse when there is none.
	InitAsk InitMode = iota
	// InitYes creates what is missing without asking.
	InitYes
)

// InitRequest is what a first start would create.
type InitRequest struct {
	// ConfigPath is the configuration file, written with a session key.
	ConfigPath string
	// UsersFile is created from its .sample template when that exists.
	UsersFile string
	// StorageRoot is created for the local backend only: a remote backend is
	// reached over the network, so nothing is written locally for it.
	StorageRoot string
}

// NeedsInit reports whether anything has to be created. A block of creation is
// about to happen when the configuration file is missing, or when the local
// storage root is missing.
func (r InitRequest) NeedsInit() bool {
	if _, err := os.Stat(r.ConfigPath); errors.Is(err, os.ErrNotExist) {
		return true
	}
	if r.StorageRoot != "" {
		if _, err := os.Stat(r.StorageRoot); errors.Is(err, os.ErrNotExist) {
			return true
		}
	}
	return false
}

// Describe lists what will be created, one per line, for the prompt and the log.
func (r InitRequest) Describe() []string {
	return r.lines(true)
}

// StartupNote turns the request into the note an administrator sees when the
// first start was declined. Nothing was created yet, so the note says what a
// later save will create; a declined start is not a broken one.
//
// A configured users file is one of the things /api/config must never name, so
// the note says "the users file" and leaves the path to the log.
func (r InitRequest) StartupNote() (key string, paths []string) {
	lines := r.lines(false)
	if len(lines) == 0 {
		lines = []string{"startup files"}
	}
	return "init.declined", lines
}

// lines lists what is missing. With detail it also names the configuration and
// users file paths, which belong in the prompt and the log but not in a note
// that is served to the browser.
func (r InitRequest) lines(detail bool) []string {
	var lines []string
	configPath := r.ConfigPath
	usersPath := r.UsersFile
	if !detail {
		configPath = "the configuration file"
		usersPath = "the users file"
	}
	if _, err := os.Stat(r.ConfigPath); errors.Is(err, os.ErrNotExist) {
		lines = append(lines, configPath+" (with a new session key)")
	}
	if _, err := os.Stat(filepath.Dir(r.ConfigPath)); errors.Is(err, os.ErrNotExist) {
		lines = append(lines, "the folder "+filepath.Dir(r.ConfigPath))
	}
	if _, err := os.Stat(r.UsersFile); errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat(r.UsersFile + ".sample"); err == nil {
			lines = append(lines, usersPath+" (copied from its .sample template)")
		} else if !containsPrefix(lines, "the folder "+filepath.Dir(r.UsersFile)) {
			lines = append(lines, "the folder "+filepath.Dir(r.UsersFile))
		}
	}
	if r.StorageRoot != "" {
		if _, err := os.Stat(r.StorageRoot); errors.Is(err, os.ErrNotExist) {
			if detail {
				lines = append(lines, "the storage folder "+r.StorageRoot)
			} else {
				lines = append(lines, "the storage folder")
			}
		}
	}
	if len(lines) == 0 {
		// Something in NeedsInit was true but Describe could not name it (for
		// example a permissions error where a missing file was expected): say
		// so rather than print an empty list.
		lines = append(lines, "the missing startup files")
	}
	return lines
}

func containsPrefix(lines []string, wanted string) bool {
	for _, line := range lines {
		if line == wanted {
			return true
		}
	}
	return false
}

// ResolveInit decides whether the first start may create files.
//
// mode comes from the command line, out is the terminal the question is printed
// on, and interactive reports whether there is one to ask on. The answer comes
// back with a human-readable reason when it is no, so the caller can say why it
// is refusing to start.
func ResolveInit(mode InitMode, req InitRequest, out io.Writer, interactive bool, ask func(prompt string) (bool, error)) (bool, string) {
	if !req.NeedsInit() {
		return true, ""
	}
	switch mode {
	case InitYes:
		return true, ""
	}
	if !interactive {
		return false, "this is a first start and there is no terminal to ask on: start with -init to create the files, " +
			"or prepare them yourself and start again"
	}

	var b strings.Builder
	b.WriteString("FileGator needs to create the following, which do not exist yet:\n")
	for _, line := range req.Describe() {
		fmt.Fprintf(&b, "  - %s\n", line)
	}
	b.WriteString("Create them now? [y/N] ")
	fmt.Fprint(out, b.String())

	answer, err := ask("")
	if err != nil {
		return false, "could not read the answer: " + err.Error()
	}
	if !answer {
		return false, "the first start was declined: nothing was created and the server will not start. " +
			"Start again to answer, or pass -init to create the files without asking"
	}
	return true, ""
}
