package app

import (
	"fmt"
	"log"
	"os"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/auth/ldap"
	sqlprovider "github.com/filegator/filegator/internal/auth/sql"
	"github.com/filegator/filegator/internal/config"
)

// openAuth builds the auth provider selected by the configuration.
func openAuth(cfg config.Auth, notes *StartupLog) (auth.Provider, error) {
	switch cfg.Type {
	case "", config.AuthTypeJSON:
		return openUsersFile(cfg, notes)
	case config.AuthTypeMySQL:
		provider, err := sqlprovider.New(cfg.SQL, sqlprovider.MySQL)
		if err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
		log.Printf("using MySQL authentication (%s)", provider.Describe())
		return provider, nil
	case config.AuthTypePostgres:
		provider, err := sqlprovider.New(cfg.SQL, sqlprovider.Postgres)
		if err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
		log.Printf("using PostgreSQL authentication (%s)", provider.Describe())
		return provider, nil
	case config.AuthTypeLDAP:
		provider, err := ldap.New(cfg.LDAP)
		if err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
		log.Printf("using LDAP authentication at %s", provider.Describe())
		return provider, nil
	default:
		return nil, fmt.Errorf("auth: unknown type %q", cfg.Type)
	}
}

// openUsersFile prepares the JSON users provider: it materialises the file
// from the shipped sample on first run and refuses a read-only store.
//
// notes records what was created, so the administrator sees it in the
// interface rather than only in a startup log nobody was watching.
func openUsersFile(cfg config.Auth, notes *StartupLog) (auth.Provider, error) {
	users := jsonfile.NewUsersFile(cfg.UsersFile)

	if created, err := users.EnsureFromTemplate(cfg.UsersFile + ".sample"); err != nil {
		if os.IsNotExist(err) {
			log.Printf("warning: no %s.sample found; using built-in default accounts", cfg.UsersFile)
			notes.add(noteWarning, "users.builtin",
				"No users file could be created from the sample: the built-in default accounts are in use and are not saved to disk.")
		} else {
			// A read-only volume must not stop the server: fall back to the
			// built-in defaults so administrators can still sign in.
			log.Printf("warning: cannot create %s: %v", cfg.UsersFile, err)
			notes.add(noteWarning, "users.unwritable",
				"The users file could not be created ("+err.Error()+"); the built-in default accounts are in use for this run only.")
		}
	} else if created {
		log.Printf("created %s from the sample template (default login: admin / %s)",
			cfg.UsersFile, jsonfile.DefaultAdminPassword)
		// The configured users file is one of the things the public
		// configuration must never name, so the path is only mentioned when
		// it is the default the docs already print.
		where := cfg.UsersFile
		if where != config.DefaultUsersFile {
			where = config.DefaultUsersFile
		}
		notes.add(noteInfo, "users.created",
			"Created "+where+" with the default accounts (admin and guest). "+
				"The default password is published: change it on Users.")
	}

	// A read-only users file is fatal: password and permission changes would
	// appear to succeed and then vanish, which is worse than not starting.
	if err := users.CheckWritable(); err != nil {
		return nil, err
	}
	// FILEGATOR_ADMIN_PASSWORD sets the first administrator's password before
	// anyone signs in, so an internet-facing instance is never briefly
	// protected by the published default.
	if _, err := applyAdminPasswordSeed(users); err != nil {
		return nil, err
	}

	if users.DefaultAdminPassword() {
		log.Printf("WARNING: the admin account still uses the default password %q; change it in Admin -> Users, "+
			"or set %s before the next start (see docs/content/install.md)",
			jsonfile.DefaultAdminPassword, adminPasswordEnv)
	}
	return users, nil
}
