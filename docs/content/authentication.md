---
title: "Authentication"
weight: 28
---

FileGator can take accounts from four places. One is active at a time, chosen in
**Admin → Auth**; switching is validated before it happens, and a failed
switch leaves the previous provider running.

| Provider | `auth.type` | Writable from the admin UI |
|----------|-------------|---------------------------|
| Users file | `json` (default) | yes |
| LDAP / Active Directory | `ldap` | no |
| MySQL / MariaDB | `mysql` | yes |
| PostgreSQL | `postgres` | yes |

Whatever the provider, users get the same **roles** (`guest`, `user`, `admin`)
and **permissions** (`read`, `write`, `upload`, `download`, `batchdownload`,
`zip`, `chmod`).

## Users file

```json
{ "auth": { "type": "json", "users_file": "private/users.json" } }
```

The default, and the simplest. On first start `private/users.json` is copied
from `private/users.json.sample`, which contains `admin` / `admin123` and a
disabled `guest`. **Change the admin password immediately**; the server logs a
warning at every start until you do. Setting `FILEGATOR_ADMIN_PASSWORD` before
the first start replaces that default without anyone signing in — see
[Installation]({{< ref "install.md" >}}).

The file uses the same format FileGator has always used, so an existing one
works unchanged. Passwords are bcrypt only — `$2y$`, `$2a$` or `$2b$`. A
plain-text password never authenticates.

Full key reference: [Configuration]({{< ref "configuration.md" >}}).

## LDAP

```json
{
  "auth": {
    "type": "ldap",
    "ldap": {
      "server": "ldaps://dc1.example.com:636",
      "bind_dn": "cn=filegator,ou=services,dc=example,dc=com",
      "bind_pass": "…",
      "base_dn": "ou=people,dc=example,dc=com",
      "filter": "(uid=%s)",
      "attributes": ["uid", "cn", "mail"],
      "username_attribute": "uid",
      "name_attribute": "cn",
      "default_permissions": ["read", "write", "upload", "download"],
      "admin_usernames": ["jane.doe"],
      "home_template": "/users/%s",
      "user_cache_ttl": 60,
      "timeout": 10
    }
  }
}
```

| Key | Default | Meaning |
|-----|---------|---------|
| `server` | — | `ldap://`, `ldaps://` or `ldapi://` URL. Required |
| `bind_dn`, `bind_pass` | — | Service account used to search. It must be able to read `base_dn`. Searching anonymously is refused |
| `base_dn` | — | Where to search. Required |
| `filter` | `""` | Extra search filter, e.g. `(objectClass=person)`. The username condition `(<username_attribute>=<login>)` is ANDed onto it, and the login is filter-escaped so a typed `*` cannot enumerate accounts. Empty means "any entry with a username attribute" |
| `attributes` | `username_attribute`, the name attribute, `dn` | Attributes to request |
| `username_attribute` | — | Which attribute is the login name. **Required** |
| `name_attribute` | `""` (falls back to the username) | Display name attribute |
| `add_domain` | `""` | Appended to a typed username that has no domain, e.g. `@example.com` |
| `remove_domains` | `[]` | Stripped from a typed username before searching, e.g. `["EXAMPLE\\"]` |
| `default_permissions` | `[]` | Granted to every resolved user |
| `admin_usernames` | `[]` | Promoted to administrators, with all permissions and home `/` |
| `home_template` | `/` | Home folder for each user; `%s` becomes the login name. Without it every LDAP user shares the storage root. Use `/%s` to give every user a private folder |
| `user_cache_ttl` | `60` | Seconds a resolved user is cached. `0` means the default, not "no cache" |
| `timeout` | `10` | Seconds per LDAP operation |

### How a login works

1. Bind as the **service account** and search for the filter.
2. Bind **as the found DN** with the supplied password. If that succeeds, the
   password is correct — the directory decides, FileGator never stores it.
3. Map the entry to permissions and home folder, then cache the result.

**An empty password is refused before any bind**, because directories commonly
treat an empty password as an anonymous bind that succeeds.

### What differs from the users file

- **LDAP users are read-only here.** The admin UI hides create, edit and delete,
  and password changes return `403` — change them in the directory.
- Users are **cached** for `user_cache_ttl` seconds instead of querying the
  directory per request. A directory outage therefore keeps existing sessions
  working until the cache expires.
- Startup performs one search, so a wrong server, bind account or base DN fails
  immediately with the reason rather than at the first login.

## MySQL / PostgreSQL

```json
{
  "auth": {
    "type": "mysql",
    "sql": {
      "dsn": "filegator:secret@tcp(127.0.0.1:3306)/files?parseTime=true",
      "table": "users",
      "max_open_conns": 10,
      "max_idle_conns": 2,
      "timeout": 30
    }
  }
}
```

`type` is `mysql` or `postgres`; `postgres` takes a libpq-style DSN
(`postgres://user:pass@host/db`).

| Key | Default | Meaning |
|-----|---------|---------|
| `dsn` | — | Connection string. Write-only: the API never returns it, and an empty value in the admin form keeps the stored one |
| `table` | `users` | Users table name |
| `user_cache_ttl` | `0` (no cache) | Seconds a resolved user is cached |
| `max_open_conns`, `max_idle_conns` | the driver's own | Connection pool bounds |
| `timeout` | — | Seconds for individual queries |

The table layout is the same as `users.json` — see
[Database users]({{< ref "database.md" >}}), which covers the schema, creating
rows and the password format.

**FileGator never creates or migrates the table.** Startup verifies it is
readable and refuses to start with a message pointing at that page.

The DSN is write-only: the API never returns it, and an empty value in the admin
form keeps the stored one.

## Choosing one

| | Users file | LDAP | MySQL / Postgres |
|---|---|---|---|
| Set up in the admin UI | yes | yes | yes |
| Managed from FileGator | yes | **no** | yes |
| Password changes here | yes | **no** | yes |
| Best for | small installs | an existing directory | accounts already in an app database |

## Security notes

- All providers use the same signed-cookie sessions. Any change to a user's
  role, permissions, home folder or password invalidates their sessions
  immediately.
- Passwords are never logged, and API responses never include password material
  or connection strings.
- LDAP and SQL credentials sit in `config.json` in plain text: `chmod 600` it.
- Enable `security.secure_cookies` when serving over HTTPS, so session cookies
  are not sent in clear.
