---
title: "Upgrading and what's new"
weight: 5
---

This is the page for people who ran **FileGator 7** and are moving to
**FileGator 8**, and for anyone asking what version 8 changes. It covers four
things: how the upgrade works, what carries over untouched, what is new, and
what is missing or different, with the reason.

## The short version

Version 8 is a **rewrite**, not an update. The PHP/Vue application was replaced
by a single Go binary serving plain HTML, CSS and JavaScript. Your **accounts,
your files and your folder layout are unchanged**; the interface, the
configuration file and the way storage and authentication are set up are new.

| | Version 7 | Version 8 |
|---|---|---|
| Runtime | PHP 7/8 behind Apache or nginx | one Go binary, no runtime to install |
| Container | `filegator/filegator` on PHP/Apache, user `www-data` | `filegator/filegator` on a small Alpine base, user 65532, installation at `/data` — see [Docker]({{< ref "docker.md" >}}) |
| Frontend | Vue, compiled with webpack into `dist/` | plain HTML, CSS and ES modules, served from the binary |
| Install | copy the files into a web root | run the binary |
| Configuration | `private/config.php` plus `configuration.php` | `private/config.json`, editable in the browser |
| Storage and auth | PHP adapters in `private/repos/`, configured in PHP | built in — local, SFTP, S3; users file, LDAP, MySQL, PostgreSQL — configured on the **Setup** screens |
| Extending it | PHP service adapters in `private/repos/`, bound in `configuration.php` | built-in backends configured on the **Setup** screens; the interface can be replaced with `frontend_dir` |
| Event log | `private/logs/app.log`, via Monolog | JSON Lines, to standard output by default |

## How to upgrade

1. **Stop the old process.** Do not run both against the same folder: they
   write to the same `private/users.json`.
2. **Back up `private/users.json`** and `repository/`. The users file is the only
   copy of your accounts.
3. **Build or obtain the version 8 binary** and put it in the same directory as
   the old installation — see [Installation]({{< ref "install.md" >}}).
4. **Start it.** The folder layout is unchanged, so an existing installation is
   read as it is. Nothing is migrated, moved or rewritten on the first start.
5. **Sign in.** Every account from version 7 works, with the same password, the
   same role, the same home folder and the same permissions.
6. **Check the storage root** on **Setup → Repository**. It defaults to
   `repository/`; point it at the folder your files are really in if that is
   somewhere else.
7. **Configure auth and storage in the browser.** An installation that used
   LDAP or a database in version 7 needs its connection settings entered on
   **Setup → Auth**; see [Authentication]({{< ref "authentication.md" >}}).
8. **Read the "[what is missing](#what-is-missing)" section below** before you
   consider the upgrade done. Custom `configuration.php` settings and the old
   frontend customisations do not carry over.

### What carries over

| Path | Purpose | Upgraded? |
|------|---------|-----------|
| `private/users.json` | Users, roles, home folders, permissions, bcrypt hashes | **Yes, unchanged.** Version 8 uses the same file and the same format |
| `private/config.json` | Version 8 configuration | Created on the first start of version 8 (with its session key) after the first-start question is answered "yes" or `-init` is passed; settings then survive restarts |
| `repository/` (or any other storage root) | Your files | **Yes, untouched.** Ordinary files and folders, read in place |
| `private/logs/` | Event log, if you keep a file log | Kept; see the log difference below |
| `private/tmp/` | Upload staging | Same purpose; uploads abandoned by version 7 are not resumed |
| Version 7 Docker volumes | The `private/` and `repository/` mounts | **The files, not the mounts.** Version 8 reads them from `/data/private` and `/data/repository` and runs as UID 65532, so the mount points and the folder owner must be changed once; see [Coming from a version 7 image]({{< ref "docker.md" >}}#coming-from-a-version-7-image) |

Because `users.json` is shared, the **same `users.json` file works in both
directions** during a trial: stop one, start the other. Version 7 files keep
their pipe-separated permissions (`read|write|upload|download|batchdownload|zip|chmod`)
and their bcrypt password hashes (`$2y$`, `$2a$`, `$2b$`).

### Files that are no longer read

These version 7 files are ignored by version 8 and can be deleted once you are
sure everything works:

- **`configuration.php`** — replaced by `config.json` and the **Setup** screens.
  Their settings are not imported:
  set the logo, app name, language and upload limits again on **Setup → UI** and
  **Setup → Files**.
- **The version 7 frontend** (`dist/`, or `frontend/` if you patched the
  sources) — nothing serves it. See
  [customising the interface]({{< ref "customising.md" >}}) for how the same
  changes are made in version 8.

### Upgrade checklist

- [ ] `private/users.json` backed up, and still readable by the user running
      the server (`chmod 600 private/users.json`, `chmod 700 private`)
- [ ] Storage root confirmed on **Setup → Repository**
- [ ] Admin password changed if it is still the default; the notice bar says so
- [ ] Guest access re-checked — it is **off** unless enabled.
      See [Guest access]({{< ref "guest-access.md" >}})
- [ ] LDAP or database authentication re-entered on **Setup → Auth**
- [ ] HTTPS, secure cookies and the trusted IP header set behind a proxy, as
      described under [Behind a proxy]({{< ref "install.md" >}}#behind-a-proxy)
- [ ] Log destination chosen: standard output, or a file as before
- [ ] `configuration.php` snippets and `add_to_head` / `add_to_body` looked at
      — neither is ported

## What is new

### Deployment

- **One binary.** No PHP, no Apache requirement, no `composer`, no
  `npm`. Dependencies are vendored, so it builds and runs offline.
- **Any folder can be the storage root**, chosen from a folder picker in the
  browser and applied without a restart. The files are managed in place:
  nothing is written into them.
- **Two more storage backends besides the local folder**: **SFTP** (password or
  key, served from a remote directory) and **S3** (AWS and S3-compatible
  services such as MinIO). See [Storage backends]({{< ref "storage.md" >}}).
- **An official container image**, `filegator/filegator`, on a small Alpine base
  with an unprivileged user. A fresh volume needs no first-start
  question answered. See [Docker]({{< ref "docker.md" >}}).
- **Storage and authentication are configured in the browser.** No PHP files to
  edit, no adapter files to drop in.
- **`FILEGATOR_ADMIN_PASSWORD`** sets the first administrator's password from
  the environment, so an exposed instance never runs on the published default.
- **`-reset-admin-password` and `-list-admins`** recover an installation whose
  administrator password was lost or whose username is unknown. See
  [Forgotten admin password]({{< ref "install.md" >}}#forgotten-admin-password).
- **The first start asks before it writes anything**: `private/`, `config.json`,
  the users file and the storage folder are created only after a yes. `-init`
  creates them without asking, for containers and service units. See
  [Installing]({{< ref "install.md" >}}).
- **A readable frontend.** The interface is HTML, CSS and ES modules you can
  open and edit, served from the binary; `-export-frontend` hands you exactly
  the copy the binary contains.

### Authentication

- **Three new providers**: LDAP, MySQL and PostgreSQL, alongside the users
  file. See [Authentication]({{< ref "authentication.md" >}}) and
  [Database users]({{< ref "database.md" >}}).
- **Single sign-on hand-off** (`/auth/sso`): another application signs a
  short-lived ticket and FileGator opens a session for that user, with
  `existing`, `policy` or `create` behaviour. Version 7 could not do this. See
  [Single sign-on]({{< ref "sso.md" >}}).
- **The last administrator cannot be deleted or demoted**, so an installation
  cannot be left with nobody who can manage it.

### Files and browsing

- **Server-side search** across the whole home folder, from the header, with
  the same row actions on a result as on a listing row. See
  [HTTP API]({{< ref "api.md" >}}#search).
- **Resumable uploads with pause, resume, per-file cancel and retry**, a speed
  readout and a summary line — and a running upload survives a page reload.
- **A collision prompt instead of a fixed policy**: when a name is taken, the
  upload stops and asks **Keep both**, **Replace**, **Skip** or **Cancel**, with
  an "apply to the remaining files" option for a batch. See
  [Uploading over an existing file](#uploading-over-an-existing-file).
- **Copy and move** with a folder-tree picker, from the row menu and for the
  whole selection.
- **An image gallery** with keyboard and swipe navigation and a thumbnail
  strip; PDFs and images open in the browser rather than downloading.
- **Column sorting** on Name, Size and Modified, with folders always first.
- **Light and dark themes**, following the operating system unless the user
  chooses.
- **A genuinely mobile interface**: one-line rows on a phone, bottom-sheet
  dialogs, touch targets of at least 44 px, no zooming into inputs.
- **An account menu** holding the theme, the language, **Change password** and
  **Log out**.
- **Every language file now carries the same messages as English**, so no
  screen falls back to English mid-sentence.
- **The Setup area is English only**: user-facing screens (login, file browser,
  dialogs) are translated into every language the installation offers; the
  administration screens are in English for everyone.

### Security and operations

- **Path confinement is enforced inside the storage layer**, not by the HTTP
  handler, so no endpoint can be written that forgets it.
- **Only bcrypt hashes authenticate.** A record with a plain-text or MD5
  password fails to sign in; it is never upgraded on the fly.
- **Sessions are one signed cookie** carrying a fingerprint of role,
  permissions, home folder and password: changing any of them ends that
  account's sessions immediately.
- **Zip-slip protection** on extraction, and batch downloads streamed with no
  temporary archive on disk.
- **The event log covers more than sign-ins**: file changes and admin actions
  are recorded too, as JSON Lines with a stable `event` id. See
  [Event log]({{< ref "logging.md" >}}).
- **Guest access is read-only and off by default**, enforced in code rather
  than by the stored permission string. See
  [Guest access]({{< ref "guest-access.md" >}}).
- **A clean JSON API** under `/api/` with a stable error envelope, documented
  and generated from the router itself. See [HTTP API]({{< ref "api.md" >}}).

### Running it

- **`-base-path`** serves the whole application under a prefix such as
  `/browser`. See [Running under a subpath]({{< ref "subpath.md" >}}).
- **`-frontend-dir` and `-export-frontend`** serve an editable copy of the
  interface, and hand you exactly the copy the binary contains. See
  [Customising the interface]({{< ref "customising.md" >}}).
- **A documentation website built in Go** from the Markdown pages, with a
  search index — the site can be rebuilt and reviewed by anyone.

## What is missing

Each of these was in version 7 or was expected from it, and is **not** in
version 8. The reason and the alternative follow, so none of it is a surprise
after the upgrade.

### Storage and auth adapters

**Not ported.** Version 7 bound storage and authentication through PHP service
adapters — classes, registered in `configuration.php`.
Version 8 has no PHP runtime and no place to drop a class into. Every storage
and auth backend it supports is built in:

- Storage: local, SFTP and S3, configured on **Setup → Repository**.
- Authentication: users file, LDAP, MySQL and PostgreSQL, configured on
  **Setup → Auth**.

### Frontend customisation from `configuration.php`

**Not ported.** `add_to_head` and `add_to_body` injected arbitrary markup into
the generated page. In version 8 the page is a file: put the tag in
`index.html` in a `frontend_dir` folder. This is explained, with examples, in
[Customising the interface]({{< ref "customising.md" >}}#adding-to-head-or-body).

The **compiled** frontend of version 7 (webpack output in `dist/`, Sass theme
variables, translations that had to be registered in `shared.js`) is gone. In
version 8 the interface is readable files, colours are CSS custom properties,
and a translation is a `.json` file that is discovered automatically.

### `overwrite_on_upload`

**Not ported — there is no global default.** Version 7 had one switch deciding
up front whether an upload replaced an existing file or failed. Version 8 asks
at the moment of the collision, every time, and nothing is replaced unless you
say so. API clients state their intent with `on_collision`. Full explanation:
[Uploading over an existing file](#uploading-over-an-existing-file).

### Pagination

**Not ported.** Version 7 had a page-size selector (5, 10 or 15 rows). Version 8
renders a folder in full and relies on search to find something in a large one.
The `pagination` key is gone from the configuration and from the public
`/api/config`; an old `config.json` carrying it simply ignores it.

### `search_simultaneous`

**Kept in the configuration, but unused.** It throttled version 7's browser-side
search requests, which no longer exist: search is a server-side walk now. The key
is still stored and still saved by the settings screen so an upgraded file keeps
working, but nothing reads it.

### Public share links

**Not implemented, and not in version 7 either.** A per-row "Copy link" is
deliberately absent: a link someone can paste for another person is a sharing
feature, and sharing needs tokens, expiry and revocation designed first. Guest
access publishes a folder; it does not create links. See
[Guest access]({{< ref "guest-access.md" >}}).

### The trash

**Nothing was lost.** Version 7 had no trash either: deletion is permanent in
both, after a confirmation.

### Smaller differences

| In version 7 | In version 8 |
|---|---|
| The event log always went to `private/logs/app.log`. | JSON Lines to **standard output** by default, so the container's log driver collects it. To keep the old file, set `logging.file` to `private/logs/app.log` and `logging.max_size` to `10485760`. |
| A **permissions column** showed the octal mode, toggling to `-rw-r--r--` on click. | No column. The mode is shown in the row's **⋯** menu as `Mode (644)`, next to the action that changes it. See [Users, roles and permissions]({{< ref "users.md" >}}). |
| A **profile icon** with a separate theme button and logout button. | One **account menu** with the user's name: theme, language, change password, log out. |
| Uploading onto an existing name **auto-renamed** the file (`name (1).ext`). | Version 8 asks. **Keep both** does the same renaming, as an explicit choice. |
| Batch download created a temporary archive and handed out a link from an archive list. | The zip is **streamed** straight to the browser. Nothing is left behind, and there is nothing to expire. |
| Navigating with the **browser's Back button skipped folders**. | Back and Forward walk the folder history. |
| The **timezone** setting. | The browser's own clock is used; a `timezone` key in `config.json` is ignored. Times are formatted with `date_format`, in the same moment.js token syntax as version 7, so an old pattern keeps working. |
| **`config.php`** was the place to set the logo, title and language, and a hand edit was how it was done. | The same things are on **Setup → UI**, so they survive an upgrade. Do not patch files for them. |
| The frontend arrived **compiled**. | The frontend is readable source, served from the binary, and can be replaced with a folder. |
| A PHP **web server setup**: the files in a web root, with rewrite rules. | The binary listens on a socket; a reverse proxy in front of it is optional but recommended for HTTPS. |

### What "not ported" means for a bug report

If you used something in version 7 that this page does not mention and version 8
does not have, that is a gap in the porting notes: report it. What is listed
above was left out on purpose, with the reason.

## Uploading over an existing file

Version 7 had one switch, `overwrite_on_upload`, applied to every upload:
either replacing was allowed silently, or every upload with a clashing name
failed. Version 8 has **no such setting**, on purpose: the answer depends on the
file, and answering once for everything is how the wrong file gets overwritten.

Every upload whose name is already taken stops and asks what to do. With the
default policy the refusal arrives on the **first chunk**, before anything is
stored, so a doomed upload costs one request rather than the whole file:

| Choice | Result |
|---|---|
| **Keep both** | The new file is stored under the next free name: `report.pdf` becomes `report (1).pdf`, and `archive.tar.gz` becomes `archive (1).tar.gz`. Version 7 produced `archive.tar (1).gz`, which most people consider a bug: the whole compound extension is kept now. |
| **Replace** | The existing file is overwritten. Destructive, so it is never the default and never offered for folders. |
| **Skip** | This file is not uploaded; the rest of the batch carries on. |
| **Cancel** | The upload is abandoned. |

For a batch, the dialog also offers **apply to the remaining files**, so fifty
clashing files do not mean fifty prompts. Folders never offer **Replace**:
deleting a whole tree to create a directory of the same name is never what
anyone meant.

With **Keep both** or **Replace** the final name is settled when the last chunk
arrives, not on the first: the conflicting file can appear or disappear while a
long upload is still running, so the decision belongs to the moment the file is
actually put in place.

The default, when nobody is asked — an API client that sends no policy at all —
is to **refuse** with `409 Conflict`. Nothing is overwritten without being
asked, and an automated uploader must state its intent:

```json
{ "parent": "/", "name": "report.pdf", "on_collision": "upcount" }
```

`on_collision` accepts `refuse` (default), `upcount` or `replace` — the same
policy applies to create, mkdir, rename, move and copy. See
[HTTP API]({{< ref "api.md" >}}#name-collisions). If an installation genuinely
needs the old behaviour — always replace, never ask — that is an explicit
`on_collision: "replace"` in the client, not a setting on the server.

## Migrating a customised interface

A version 7 installation with a patched frontend, custom colours or an injected
snippet keeps none of it. Do this instead:

1. Keep your `private/users.json` and `repository/` — both are unchanged.
2. **Colours**: CSS custom properties in `css/site.css` served from a
   `frontend_dir` folder, not Sass variables in `App.vue`.
3. **Snippets** (`add_to_head`, `add_to_body`): tags in `index.html` in the same
   folder.
4. **Translations**: a `.json` file in `translations/`. No registration list.
5. **Logo, app name and default language**: set them on **Setup → UI** — a
   setting survives an upgrade, a patched file does not.
6. A custom storage or auth adapter has no equivalent, as described under
   [Storage and auth adapters](#storage-and-auth-adapters).

Full detail: [Customising the interface]({{< ref "customising.md" >}}).
