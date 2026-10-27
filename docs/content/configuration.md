---
title: "Configuration"
weight: 15
---

FileGator reads `private/config.json`. A missing file means built-in defaults;
on a writable folder the file is created on the first start, with a new session
key, and rewritten on every save. Every key can also be edited by hand;
**changes need a restart** unless the admin interface says otherwise.

Most of it is editable in the admin area, which is reached from the **Setup**
button and split into **Users**, **UI**, **Files**, **Security**,
**Repository** and **Auth**. This page documents the file itself, including the
keys that have no screen.

The **UI**, **Files** and **Security** pages write the same `settings` block
through `POST /api/admin/settings`, and each sends only the fields it shows: a
field absent from the request keeps its stored value. That is why saving the
Files page cannot reset the app name, and saving Security cannot reset the
upload limits. An explicitly empty value (`""`, `[]`) does clear the field, and
a JSON `null` is treated as absent.

## Complete example

```json
{
  "download_inline": ["pdf", "jpg", "jpeg", "png", "gif", "webp", "bmp", "avif"],
  "lockout_attempts": 5,
  "lockout_timeout": 15,
  "frontend_dir": "",

  "frontend_config": {
    "app_name": "FileGator",
    "logo": "/img/filegator-logo.svg",
    "logo_dark": "/img/filegator-logo-dark.svg",
    "language": "english",
    "language_selection": true,
    "date_format": "YYYY/MM/DD HH:mm:ss",
    "upload_max_size": 104857600,
    "upload_chunk_size": 1048576,
    "upload_simultaneous": 3,
    "default_archive_name": "archive.zip",
    "editable": [".txt", ".css", ".js", ".ts", ".html", ".json", ".md"],
    "guest_redirection": "",
    "search_simultaneous": 5,
    "filter_entries": [],
    "max_width": 960
  },

  "security": {
    "csrf_protection": true,
    "session_key": "",
    "session_ttl": 28800,
    "secure_cookies": false,
    "ip_allowlist": [],
    "ip_denylist": [],
    "trusted_ip_header": "",
    "headers": {},
    "allow_password_change": true,
    "allow_logout": true
  },

  "storage": { "type": "local", "root": "repository", "sftp": { }, "s3": { } },
  "auth": { "type": "json", "users_file": "private/users.json", "ldap": { }, "sql": { } },
  "logging": { "enabled": true, "file": "-", "level": "info", "max_size": 0 },
  "sso": { "applications": [] }
}
```

This is the file after every setting has been saved at least once. A key that
is absent keeps its default, so a shorter file is normal.

**This block is the reference, not a template to install.** There is no
`config.json.sample` to copy, and none is wanted: the file is written on the
first start with a session key unique to the installation, so a shipped sample
would either carry a key shared by every install or be a second description of
the defaults that goes stale unnoticed. Copy the block above only to start from
an explicit file; leave `session_key` empty and the first start generates one.

`config.json` is a strict JSON document, so the block carries no comments. The
options inside `sftp`, `s3`, `ldap` and `sql` are shown as `{}` here and
documented where they are chosen: [Storage backends]({{< ref "storage.md" >}})
and [Authentication]({{< ref "authentication.md" >}}).

`internal/config/example_test.go` fails when a value here stops matching the
built-in defaults, or when a default is missing from it. A key added to
`Default()` therefore has to appear in this block in the same change.

## Top level

| Key | Default | Meaning |
|-----|---------|---------|
| `frontend_dir` | `""` | Serve the interface from this folder instead of the copy built into the binary. See [Customising the interface]({{< ref "customising.md" >}}) |

The remaining top-level keys:

| Key | Default | Meaning |
|-----|---------|---------|
| `download_inline` | PDF and image types | Extensions rendered in the browser instead of downloaded. Editable on **Admin → Files**. `*` means everything, which lets uploaded HTML/SVG run on this origin — avoid it |
| `lockout_attempts` | `5` | Failed logins per address before a lockout. `0` disables the lockout |
| `lockout_timeout` | `15` | Seconds before the counter resets |

## `frontend_config`

| Key | Default | Meaning |
|-----|---------|---------|
| `app_name` | `FileGator` | Page title. |
| `logo` | the built-in logo | The logo in light mode, shown in the header and on the login page. Empty means the FileGator artwork built into the binary. See [The logo](#the-logo) |
| `logo_dark` | the built-in dark logo | The logo in dark mode. Empty means the built-in dark artwork |
| `language` | `english` | Default interface language |
| `language_selection` | `true` | Whether users may switch languages. When `true`, the account menu offers **Language** (a dialog with a dropdown of every available language); when `false` the item is absent and the interface stays in `language`. **Admin → UI** |
| `date_format` | `YYYY/MM/DD HH:mm:ss` | How the **Modified** column is written. See below |
| `upload_max_size` | 100 MB | Largest accepted file, in bytes. **Admin → Files** |
| `upload_chunk_size` | 1 MB | Chunk size; larger means fewer requests, more memory per upload. **Admin → Files** |
| `upload_simultaneous` | `3` | Files uploaded at once. **Admin → Files** |
| `default_archive_name` | `archive.zip` | Name offered when zipping. **Admin → Files** |
| `editable` | `[".txt", ".css", …]` | Extensions the editor may open. **Admin → Files** |
| `guest_redirection` | `""` | See below |
| `search_simultaneous` | `5` | **Unused.** It throttles concurrent browser-side search requests, which the server-side search does not make |
| `filter_entries` | `[]` | Glob patterns hidden from listings until the user ticks **Show hidden** |
| `max_width` | `960` | Cap on the interface width in pixels. **0 means no cap — the card then fills the window**. **Admin → UI** |

### The logo

The interface shows one logo in two themes: `logo` in light mode and
`logo_dark` in dark mode. It appears in the header of the file browser, in the
header of the administration area, and on the login page — one pair covers all
three, so there is nothing separate to set for the login screen. The browser
picks the variant from the theme, before the page is painted.

**The defaults are built into the binary.** `logo` defaults to
`/img/filegator-logo.svg` and `logo_dark` to `/img/filegator-logo-dark.svg`,
both of which are served by the application itself. A fresh install therefore
shows the FileGator logo without reaching out to another server, and keeps
working on a machine with no internet access.

Both fields are edited on **Admin → UI**, with a preview showing the two
variants side by side. Each accepts:

- an absolute `http(s)` URL, for a logo hosted elsewhere;
- a path on this server, such as `/img/mine.svg`. A file in a custom interface
  folder (`frontend_dir`) is served from there, so `/img/mine.svg` works when
  the folder has an `img/mine.svg`;
- an empty value, meaning the built-in logo.

Anything else — a bare relative path, `javascript:`, `data:` — is refused when
saved, because the value is written into an `<img src>` where those either break
the page or run script in this origin. (A hand-edited `config.json` is checked
too: the server refuses to save an invalid value.)

**One picture, two themes.** An empty `logo_dark` shows the same picture as
`logo` when one was set: an administrator who supplied a single logo meant that
logo, and the FileGator drawing reappearing in half of the themes would be a
surprise. Dark lettering on a dark page is hard to read, which is the reason
`logo_dark` exists.

**Removing a logo** is done by emptying both fields, which restores the built-in
artwork. There is no "no logo at all" setting: the header's home button and the
login page both lean on the branding.

Under `-base-path` the built-in paths are returned with that prefix
(`/browser/img/filegator-logo.svg`), so a deployment under a subpath loads
them like any other asset.

### `date_format`

How the **Modified** column is written, in the moment.js token syntax — so a
pattern written for any earlier FileGator keeps working. The tokens are not
reinterpreted: `YY/MM/DD hh:mm:ss` means what it always meant.

| Token | Meaning | Example |
|-------|---------|---------|
| `YYYY` / `YY` | year, 4 or 2 digits | `2026` / `26` |
| `MM` / `M` | month, padded or not | `03` / `3` |
| `MMM` / `MMMM` | month name | `Oct` / `October` |
| `DD` / `D` | day of month | `03` / `3` |
| `Do` | day with ordinal | `3rd` |
| `dddd` / `ddd` | weekday name | `Saturday` / `Sat` |
| `HH` / `H` | hour, 24-hour | `09` / `9` |
| `hh` / `h` | hour, 12-hour | `09` / `9` |
| `mm` / `ss` | minutes / seconds | `04` / `05` |
| `A` / `a` | `AM` / `pm` | |
| `SSS` | milliseconds | `007` |
| `Z` / `ZZ` | UTC offset | `+0200` / `+02:00` |

Text in square brackets prints as written, and a backslash escapes the next
character, so a pattern can carry words:

```json
{ "frontend_config": { "date_format": "dddd, D. MMMM YYYY [at] HH:mm" } }
```

```
Saturday, 3. October 2026 at 09:04
```

Times are shown in the **browser's** local time — each user sees their own
clock. There is no timezone setting; the old `timezone` key is gone
and is ignored if present. An empty value means the default,
`YYYY/MM/DD HH:mm:ss`. The field sits on **Admin → UI** with a live
preview under it, so the result is visible before saving.

### `filter_entries`

Patterns that hide entries from a listing. The files are still there and can be
shown with the **Show hidden** control, which appears only when a pattern
actually hides something.

```json
{ "frontend_config": { "filter_entries": [".git/", "Thumbs.db", "*.tmp"] } }
```

| Pattern | Hides |
|---------|-------|
| `Thumbs.db` | that name, in any folder |
| `.git/` | a **folder** of that name; the trailing `/` means directory |
| `*.log` | names ending in `.log` |
| `/archive/*` | anything directly inside `/archive`; a leading `/` means a full path |

`*` matches any run of characters, `?` one character, and everything else is
literal. Filtering is a convenience for tidiness, **not** a security control:
hidden files remain readable through the API and by direct download.

### `guest_redirection`

When set, a visitor who is **not logged in** is sent to this URL instead of
seeing the login form. It exists for external login pages — a WordPress
single-sign-on, an SSO portal — where the form here is pointless.

```json
{
  "frontend_config": {
    "guest_redirection": "https://example.com/wp-admin/"
  }
}
```

Only `http://` and `https://` URLs are accepted; anything else is rejected when
saving, and the login form is shown as usual. A relative value such as `/login`
is refused too, because it would redirect the login page to itself.

This is independent of [guest access]({{< ref "guest-access.md" >}}): if guests
may browse, they never see the login screen at all.

## `security`

| Key | Default | Meaning |
|-----|---------|---------|
| `csrf_protection` | `true` | Double-submit token on state-changing requests. Turn off only for debugging |
| `session_key` | generated | HMAC key for session cookies. **Generated on first start**; changing it logs everyone out |
| `session_ttl` | `28800` (8 h) | Session lifetime in seconds |
| `secure_cookies` | `false` | Mark cookies `Secure`. Set to `true` behind HTTPS. **Needs a restart** — the cookie flags are built at startup |
| `ip_allowlist` | `[]` | If non-empty, only these addresses may connect. IPs or CIDR |
| `ip_denylist` | `[]` | Always refused. **Deny wins over allow** |
| `trusted_ip_header` | `""` | Header carrying the real client IP behind a proxy (`CF-Connecting-IP`, `X-Real-IP`, `X-Forwarded-For`). Empty uses the socket address |
| `headers` | `{}` | Extra response headers. An empty value removes a built-in one |
| `allow_password_change` | `true` | Whether users may change their own password. When `false` the **Change password** item is absent from the account menu **and** the endpoint answers `403`. Administrators can still set any password from **Admin → Users**. **Admin → UI** |
| `allow_logout` | `true` | Whether **Log out** is offered in the account menu and `POST /api/auth/logout` is accepted. Turn it off for an SSO deployment, where the identity provider holds the session and a local logout only looks broken. The session still expires at `session_ttl`. **Admin → UI** |

Sessions are a single signed cookie; there is no server-side session store. Any
change to a user's role, permissions, home folder or password invalidates their
existing sessions, because the cookie carries a fingerprint of the record.

## `storage`

```json
{ "storage": { "type": "local", "root": "repository" } }
```

`type` is `local`, `sftp` or `s3`. The full option set for each is documented in
[Storage backends]({{< ref "storage.md" >}}). The path is relative to the
working directory unless absolute.

## `auth`

```json
{ "auth": { "type": "json", "users_file": "private/users.json" } }
```

`type` is `json`, `ldap`, `mysql` or `postgres`. The connection fields for
each live under `auth.ldap` and `auth.sql` — every key, with its default, is in
[Authentication]({{< ref "authentication.md" >}}).

Users live in a separate file in the same format every FileGator installation
has used, so an existing `users.json` works unchanged:

```json
{
  "1": {
    "username": "admin",
    "name": "Admin",
    "role": "admin",
    "homedir": "/",
    "permissions": "read|write|upload|download|batchdownload|zip|chmod",
    "password": "$2y$10$…"
  }
}
```

Passwords are **bcrypt only** (`$2y$`, `$2a$`, `$2b$`). A record with a
plain-text password can never sign in.

## `logging`

| Key | Default | Meaning |
|-----|---------|---------|
| `enabled` | `true` | Write the event log |
| `file` | `-` | `-` is standard output; a path writes a file (mode `0600`) |
| `level` | `info` | `debug`, `info`, `warn`, `error`. Applies immediately, no restart |
| `max_size` | `0` | Rotate a file past this size, keeping one previous file. **Bytes**; the admin field takes megabytes. Must be `0` with `-` |

See [Event log]({{< ref "logging.md" >}}).

## Read-only configuration

If `config.json` cannot be written — a read-only volume, wrong ownership — the
server **still starts** on in-memory defaults with an ephemeral session key, and
says so at startup. Saving from the admin interface then answers `409` with the
code `read_only_config`: the change applies for the current run, but the manager
that asked for it is told it was not saved. The message names the cause, for
example `private is not writable` or `private is not a folder`, rather than only
saying "read-only". The notice bar shows the same condition. Changing the
storage root or the auth provider behaves the same way.

A folder that does not exist yet counts as writable when it can be created: a
fresh install starts in an empty directory, and `private/` is created on the
first start with the configuration inside it.

## What a first start creates

A binary started in an empty folder needs nothing prepared: it creates what it
needs — **after asking** — and **tells the administrator what it did**. The
notes appear in the notice bar on the next sign-in and name every path involved:

| What | When | Note |
|------|------|------|
| `config.json` | the first start was allowed and the file is missing, written with a new session key | "Created … with a new session key" |
| the storage root (`repository/`) | it does not exist | "Created the storage folder …" |
| the users file | it is missing and `users.json.sample` is next to it | "Created private/users.json with the default accounts" |
| — | the users file cannot be created | *warning*: the built-in accounts are in use and nothing is saved to disk |
| — | the configuration cannot be written | *warning*: settings are lost on restart |

Nothing is created without consent: the first start in an empty folder asks
first, and `-init` answers in advance. Answering no creates nothing and stops the
start — see [Installing]({{< ref "install.md" >}}). A missing `config.json` on a
**read-only** folder is not an error: the server runs on the built-in defaults
and the note says why the file is absent. On a folder that has run before, no
notes appear — they describe the current start, not the history of the instance.

One file is enough. The configuration is written **once, on the first start
that was allowed**, so the second start finds everything in place, and a missing
file is never required for the server to run. The folder holding it (`private/`)
is created on the way; that counts as writable, so a fresh install can save
settings immediately.

The same facts are written to the startup log on `stderr`, so they are visible
without signing in.

The **users file** is different: if it cannot be written, startup **fails**. An
administrator who changes a password and finds it forgotten after a restart is
worse off than one who is told immediately.

## `sso`

Applications allowed to hand users in with a signed URL. See
[Single sign-on]({{< ref "sso.md" >}}).

```json
{
  "sso": {
    "applications": [
      {
        "name": "panel",
        "secret": "…",
        "mode": "create",
        "home": "/clients/%s",
        "permissions": ["read", "download"],
        "admin_usernames": [],
        "expiry": 60
      }
    ]
  }
}
```

There is no admin screen for this: secrets belong in the file, not in a form
that every administrator session can read.

## Not in the file

The URL prefix the app is served under (`-base-path`) is a start-up flag, not a
setting — see [Running under a subpath]({{< ref "subpath.md" >}}).
