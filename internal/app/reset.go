package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
)

// ResetAdminPassword sets an account's password without the current one, for an
// operator who has lost access. It runs in the same process as the server
// binary but does not start it: no socket, no storage, no session.
//
// What it exists for: with one administrator and a forgotten password the
// instance is unreachable. Editing the password hash in private/users.json by
// hand is not something to ask of anyone, and deleting the file to get the
// shipped defaults back also deletes every other account. This is the escape
// hatch, and it is deliberately a command that needs shell access to the server
// — the same boundary as editing the file, without the risk of getting it
// wrong.
//
// Every session of the account ends with the change, because the session
// fingerprint covers the password hash.
//
// It reads the auth section of the config to find the users file, so
// -config has to point at the same file the server uses.
func ResetAdminPassword(configPath, username string, in io.Reader, out io.Writer) error {
	if username == "" {
		return errors.New("no username given")
	}

	store, err := loadExistingConfig(configPath)
	if err != nil {
		return err
	}
	if store.Get().Auth.Type != config.AuthTypeJSON {
		// There is nothing here to rewrite. Saying which provider is in use
		// saves an operator from hunting for a users file that does not exist.
		return fmt.Errorf("auth type is %q: passwords for this installation live in the directory or database, "+
			"not in a users file, so reset it there", store.Get().Auth.Type)
	}

	users := jsonfile.NewUsersFile(store.Get().Auth.UsersFile)
	u, err := users.Find(username)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			// Name the accounts that exist: the username is the part people
			// forget, and "not found" on its own is a dead end.
			return errNoSuchUser(username, store.Get().Auth.UsersFile, users)
		}
		return err
	}
	if u.Role == auth.RoleGuest {
		return fmt.Errorf("%q is the guest account and has no password", username)
	}
	if !users.Writable() {
		return fmt.Errorf("%s cannot be written; fix its permissions first", store.Get().Auth.UsersFile)
	}

	password, err := readNewPassword(in, out)
	if err != nil {
		return err
	}

	// Update writes only the password when the rest of the account is passed
	// back unchanged.
	if _, err := users.Update(u.Username, u, password); err != nil {
		return fmt.Errorf("write password: %w", err)
	}
	fmt.Fprintf(out, "password for %q in %s was changed\n", username, store.Get().Auth.UsersFile)
	fmt.Fprintln(out, "every session for that account has ended; sign in with the new password")
	return nil
}

// readNewPassword asks for the password twice.
func readNewPassword(in io.Reader, out io.Writer) (string, error) {
	first, err := promptHidden(in, out, "New password: ")
	if err != nil {
		return "", err
	}
	if first == "" {
		return "", errors.New("the password is empty")
	}
	second, err := promptHidden(in, out, "Repeat password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("the passwords do not match")
	}
	return first, nil
}

// promptHidden prints a prompt and reads a line with the terminal echo turned
// off.
//
// Hiding the input is done with `stty`, the only way available without adding a
// dependency (the project vendors no terminal package, and ioctl through
// golang.org/x/sys would pull one in for this). When stdin is not a terminal —
// a pipe, or a platform without stty — the line is read in the clear and the
// caller is told, rather than the password being echoed without a word about it.
func promptHidden(in io.Reader, out io.Writer, prompt string) (string, error) {
	if file, ok := in.(*os.File); ok && isTerminal(file) && commandExists("stty") {
		fmt.Fprint(out, prompt)
		restore := setEcho(file, false)
		defer func() {
			restore()
			fmt.Fprintln(out)
		}()
	} else {
		fmt.Fprintln(out, "warning: this input cannot be hidden, so the password will be visible as you type it")
		fmt.Fprint(out, prompt)
	}
	line, err := readLine(in)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// readLine reads one line from in, one byte at a time.
//
// It does not wrap the reader in a bufio.Reader, which is the obvious way to
// read a line: bufio reads ahead into its own buffer, so asking twice for a
// password — "New password:" then "Repeat password:" — left the second prompt
// with nothing to read and reported "the passwords do not match". Reading one
// byte at a time leaves the reader untouched for the next prompt, and a
// password is short enough that the cost does not matter.
func readLine(in io.Reader) (string, error) {
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			b.WriteByte(buf[0])
			if buf[0] == '\n' {
				return b.String(), nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return b.String(), nil
			}
			return "", err
		}
	}
}

// isTerminal reports whether f is a character device, which is how a terminal
// looks on the platforms this runs on.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// setEcho turns terminal echo off and returns a function that turns it back on.
// It is best effort: a failure leaves the password visible, which the caller has
// already warned about by only taking this path on a terminal.
func setEcho(f *os.File, on bool) func() {
	cmd := exec.Command("stty", "-echo")
	cmd.Stdin = f
	_ = cmd.Run()
	return func() {
		restore := exec.Command("stty", "echo")
		restore.Stdin = f
		_ = restore.Run()
	}
}
