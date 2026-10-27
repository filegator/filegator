package app

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
)

// adminPasswordEnv is the environment variable that sets the first
// administrator's password.
//
// The shipped users file has always had a published default password
// ("admin123"), which an internet-facing instance exposes to any scanner
// between the first start and the first sign-in. Rather than change how the
// first admin is created, the password can be supplied before the first start:
//
//	FILEGATOR_ADMIN_PASSWORD='a long passphrase' ./filegator
//
// It is **create-time only**: it applies while an administrator still has the
// default password, exactly like Grafana's `admin_password` and Moor's
// `MOOR_INITIAL_PASSWORD`. Once the password has been changed (in the admin
// area, or by an earlier start with this variable set) the value is ignored, so
// leaving it in a compose file or a systemd unit does not lock anyone out and
// does not silently reset a password somebody chose. A warning says so, because
// "my env var does nothing" is otherwise a silent mystery.
const adminPasswordEnv = "FILEGATOR_ADMIN_PASSWORD"

// minimumAdminPassword is the shortest accepted seed. It is deliberately a
// plain length check and nothing more: a composition rule ("one digit, one
// symbol") is a guess about what makes a password good, while length is the one
// thing that reliably helps. The admin area has no policy for the same reason.
const minimumAdminPassword = 8

// applyAdminPasswordSeed sets the first administrator's password from the
// environment when there is still an administrator using the default one.
//
// It reports whether it applied a password. Problems are fatal: a password the
// administrator expects but the server rejected would leave them locked out of
// an account they think they have configured.
func applyAdminPasswordSeed(users *jsonfile.UsersFile) (bool, error) {
	raw, ok := os.LookupEnv(adminPasswordEnv)
	if !ok || strings.TrimSpace(raw) == "" {
		return false, nil
	}

	if !users.DefaultAdminPassword() {
		log.Printf("%s is set but ignored: no administrator is using the default password, so the stored password is kept",
			adminPasswordEnv)
		return false, nil
	}
	if len(raw) < minimumAdminPassword {
		return false, fmt.Errorf("%s must be at least %d characters (it is %d)",
			adminPasswordEnv, minimumAdminPassword, len(raw))
	}
	if strings.TrimSpace(raw) != raw {
		return false, fmt.Errorf("%s has leading or trailing whitespace, which is almost always a quoting mistake", adminPasswordEnv)
	}

	recs, err := users.List()
	if err != nil {
		return false, fmt.Errorf("%s: read users: %w", adminPasswordEnv, err)
	}
	applied := 0
	for _, u := range recs {
		if u.Role != auth.RoleAdmin || u.Username == "guest" {
			continue
		}
		// Update rewrites only what changes when the password is the only
		// field set, and it re-hashes with the PHP-compatible bcrypt prefix.
		if _, err := users.Update(u.Username, u, raw); err != nil {
			return false, fmt.Errorf("%s: set password for %q: %w", adminPasswordEnv, u.Username, err)
		}
		applied++
	}
	if applied == 0 {
		return false, nil
	}
	// Never log the password, and never log how many administrators there are
	// beyond this count: it is the same information the admin area shows.
	log.Printf("administrator password set from %s; it now applies and will be ignored on the next start", adminPasswordEnv)
	return true, nil
}
