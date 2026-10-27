package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
)

// ListAdmins prints the administrator accounts of a users file.
//
// It exists because the username is the part people forget: the reset command
// needs the exact login name, and "no account named admin" is a dead end when
// the account was created under a different name, by hand in users.json, or
// imported from somewhere else. Reading the users file to find out is possible
// but uninviting, so the same shell access that resets a password can list what
// there is to reset.
//
// It prints no password material — not the hash, not its length, not a prefix.
// The only thing it says about a password is whether it is still the published
// default, which the admin area and the startup warning already say, because
// that is the thing an operator needs to know right now.
func ListAdmins(configPath string, out io.Writer) error {
	store, err := loadExistingConfig(configPath)
	if err != nil {
		return err
	}
	cfg := store.Get()
	if cfg.Auth.Type != config.AuthTypeJSON {
		return fmt.Errorf("auth type is %q: accounts live in the directory or database, not in a users file, "+
			"so list them with that system's tooling", cfg.Auth.Type)
	}

	users := jsonfile.NewUsersFile(cfg.Auth.UsersFile)
	all, err := users.List()
	if err != nil {
		return fmt.Errorf("read %s: %w", cfg.Auth.UsersFile, err)
	}

	var admins []*auth.User
	for _, u := range all {
		if u.Role == auth.RoleAdmin {
			admins = append(admins, u)
		}
	}
	// Sorted so the output is stable whatever order the file happens to have.
	sort.Slice(admins, func(i, j int) bool { return admins[i].Username < admins[j].Username })

	if len(admins) == 0 {
		fmt.Fprintf(out, "%s has no administrator account.\n", cfg.Auth.UsersFile)
		fmt.Fprintln(out, "Nothing here can sign in to the admin area. Promote an account by setting its")
		fmt.Fprintln(out, `"role" to "admin" in that file, then sign in with its password.`)
		return nil
	}

	fmt.Fprintf(out, "Administrators in %s:\n\n", cfg.Auth.UsersFile)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  USERNAME\tHOME\tPASSWORD")
	for _, u := range admins {
		state := "set"
		if jsonfile.VerifyPassword(jsonfile.DefaultAdminPassword, u.Credential()) {
			state = "STILL THE DEFAULT"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", u.Username, u.HomeDir, state)
	}
	tw.Flush()

	fmt.Fprintln(out, "\nTo reset one: filegator -reset-admin-password <username>")
	return nil
}

// errNoSuchUser is the "not found" path of a reset, with the hint that actually
// helps: the names that do exist.
func errNoSuchUser(username, usersFile string, users *jsonfile.UsersFile) error {
	all, err := users.List()
	if err != nil || len(all) == 0 {
		return fmt.Errorf("no account named %q in %s", username, usersFile)
	}
	var names []string
	for _, u := range all {
		names = append(names, u.Username)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "no account named %q in %s; the accounts are: %s",
		username, usersFile, strings.Join(names, ", "))
	b.WriteString("\nlist the administrators with -list-admins")
	return errors.New(b.String())
}

// loadExistingConfig reads a config file that must already exist.
//
// config.Load treats a missing file as a fresh install and returns the built-in
// defaults, which is right for the server — it has to start on an empty volume.
// A command that reads an installation to report on it is the opposite: a wrong
// -config path must not quietly read whatever happens to be in
// private/users.json relative to the working directory, which is what happened
// before this check existed.
func loadExistingConfig(path string) (*config.Store, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%s does not exist; point -config at the file the server uses (default private/config.json)", path)
		}
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	store, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return store, nil
}
