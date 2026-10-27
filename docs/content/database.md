---
title: "Database users"
weight: 30
---

FileGator can read users from a MySQL/MariaDB or PostgreSQL table instead of
`private/users.json`. This is useful when the accounts already live in another
application's database.

## The table is not created for you

FileGator **never creates or migrates the table**. Create it first, then point
the configuration at it. On startup FileGator checks that the table is readable
and refuses to start with an actionable message if it is not.

The column layout is identical to `users.json`:

| Column | Type | Notes |
|--------|------|-------|
| `username` | string, primary key | login name |
| `name` | string | display name |
| `role` | string | `guest`, `user` or `admin` |
| `permissions` | string | pipe-separated, e.g. `read|write|zip` |
| `homedir` | string | home folder, `/` for the storage root |
| `password` | string | bcrypt hash (`$2y$…`) |

### MySQL / MariaDB

```sql
CREATE TABLE users (
  username    VARCHAR(190) NOT NULL PRIMARY KEY,
  name        VARCHAR(190) NOT NULL DEFAULT '',
  role        VARCHAR(16)  NOT NULL DEFAULT 'user',
  permissions VARCHAR(255) NOT NULL DEFAULT '',
  homedir     VARCHAR(255) NOT NULL DEFAULT '/',
  password    VARCHAR(255) NOT NULL DEFAULT ''
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO users (username, name, role, permissions, homedir, password)
VALUES ('admin', 'Admin', 'admin',
        'read|write|upload|download|batchdownload|zip|chmod', '/',
        '$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq');
```

The hash above is the default `admin123` password. **Change it after the first
login.** `FILEGATOR_ADMIN_PASSWORD` does not apply here: it seeds the users
file, and a SQL-backed installation never has one.

### PostgreSQL

```sql
CREATE TABLE users (
  username    TEXT PRIMARY KEY,
  name        TEXT NOT NULL DEFAULT '',
  role        TEXT NOT NULL DEFAULT 'user',
  permissions TEXT NOT NULL DEFAULT '',
  homedir     TEXT NOT NULL DEFAULT '/',
  password    TEXT NOT NULL DEFAULT ''
);

INSERT INTO users (username, name, role, permissions, homedir, password)
VALUES ('admin', 'Admin', 'admin',
        'read|write|upload|download|batchdownload|zip|chmod', '/',
        '$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq');
```

## Configuration

In `private/config.json`:

```json
{
  "auth": {
    "type": "mysql",
    "sql": {
      "dsn": "filegator:secret@tcp(127.0.0.1:3306)/filegator",
      "table": "users",
      "user_cache_ttl": 60,
      "max_open_conns": 10,
      "max_idle_conns": 5,
      "timeout": 10
    }
  }
}
```

Use `"type": "postgres"` with a PostgreSQL DSN:

```json
{
  "auth": {
    "type": "postgres",
    "sql": {
      "dsn": "postgres://filegator:secret@127.0.0.1:5432/filegator?sslmode=require",
      "table": "users",
      "user_cache_ttl": 60,
      "max_open_conns": 10,
      "max_idle_conns": 5,
      "timeout": 10
    }
  }
}
```

| Option | Default | Meaning |
|--------|---------|---------|
| `dsn` | — | driver connection string (required) |
| `table` | `users` | table name |
| `user_cache_ttl` | 60 | seconds a resolved user is cached. `0` means the default, not "no cache" |
| `max_open_conns` | driver default | connection pool limit. `0` leaves the driver's own limit |
| `max_idle_conns` | driver default | idle connections kept |
| `timeout` | 10 | seconds per query |

The same settings are available in **Admin → Auth**.

## What changes with a database provider

- Users are **managed in FileGator** (create, edit, delete, password change)
  because the provider is writable. This is different from LDAP, where accounts
  live in the directory.
- The **DSN is never returned by the API**. Leave the field blank when saving to
  keep the stored value.
- Passwords are bcrypt hashes. Plain-text passwords are rejected, and an
  existing FileGator database therefore keeps working unchanged.
