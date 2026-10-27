---
title: "HTTP API"
weight: 20
---

All API routes live under `/api/`, except the single-sign-on hand-off at
`/auth/sso`. Requests and responses are JSON.

The endpoint tables on this page are **generated from the server's own route
table** with `make docs`; do not edit them by hand. An
endpoint added to the code and not to this page is a feature nobody can find,
so the generator refuses to run when a route has no description.

## Response envelope

Success — the payload is always under `data`:

```json
{ "data": { "path": "/", "name": "files" } }
```

Failure:

```json
{ "error": { "message": "invalid path", "code": "bad_request" } }
```

Stable error codes: `bad_request`, `unauthorized`, `forbidden`, `not_found`,
`conflict`, `internal_error`, `too_many_attempts`, `csrf_failed`.

## Authentication

Sessions use a single signed cookie (`filegator_session`). State-changing
requests must also send the `X-CSRF-Token` header, whose value is the
`filegator_csrf` cookie.

<!-- BEGIN apidoc:auth -->
| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/auth/login` | `{"username","password"}` → user |
| `POST` | `/api/auth/logout` | Ends the session |
| `GET` | `/api/auth/me` | The signed-in user, or `401` |
| `POST` | `/api/auth/password` | Change your own password |
| `GET` | `/api/ping` | Health check |
<!-- END apidoc:auth -->

```json
{ "current_password": "old", "new_password": "new" }
```

The current password must match, so a stolen session cannot take over the
account. A new session cookie is returned because the session fingerprint
covers the password hash.

## Files
All paths are relative to the user's home folder and always start with `/`.
A `..` segment that would leave the home folder is rejected with `403`.

<!-- BEGIN apidoc:files -->
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/files` | List a directory |
| `GET` | `/api/files/batch` | Stream a zip of selected paths |
| `POST` | `/api/files/chmod` | Set permission bits |
| `GET` | `/api/files/content` | Read a text file for the editor |
| `POST` | `/api/files/copy` | Copy entries into a folder |
| `POST` | `/api/files/create` | Create an empty file |
| `POST` | `/api/files/delete` | Delete permanently |
| `GET` | `/api/files/download` | Download a file |
| `POST` | `/api/files/mkdir` | Create a directory |
| `POST` | `/api/files/move` | Move entries into a folder |
| `POST` | `/api/files/rename` | Rename, or move in one step |
| `POST` | `/api/files/save` | Write text content |
| `GET` | `/api/files/search` | Search names below a folder |
| `GET` | `/api/files/stat` | Describe one entry |
| `POST` | `/api/files/unzip` | Extract an archive into a new folder |
| `GET` | `/api/files/upload` | Which chunks are already stored (resume) |
| `POST` | `/api/files/upload` | Send one chunk |
| `POST` | `/api/files/upload/cancel` | Discard a staged upload and free its chunks |
| `POST` | `/api/files/zip` | Create an archive |
<!-- END apidoc:files -->

All `POST` routes require the `filegator_csrf` cookie value in the
`X-CSRF-Token` header.

### Create

```json
{ "parent": "/", "name": "notes.txt" }
```

`name` must not contain `/`, `\` or be `.`/`..`. Returns the new entry with
`201`.

### Rename

```json
{ "from": "/notes.txt", "to": "/archive/notes.txt" }
```

The request fails with `409` if the destination exists. Renaming into a
different folder moves the entry.

### Move / Copy

```json
{ "paths": ["/a.txt", "/b.txt"], "target": "/archive" }
```

Entries keep their own names inside `target`. `copy` recurse into
directories.

### Delete

```json
{ "paths": ["/a.txt", "/old"] }
```

**Deletion is permanent.** Deleting `/` is refused. Returns `204`.

### Chmod

```json
{ "paths": ["/a.txt"], "mode": "644" }
```

`mode` is three octal digits (a leading `0` is accepted). Requires the
`chmod` permission in addition to `read` and `write`. Changing the home folder
itself is refused. Returns `204`.

### Upload

<!-- The upload endpoints are part of the generated table in the Files
section above; this part explains the protocol. -->

Uploads use the Resumable.js field names, so an existing upload client keeps
working:

```
resumableFilename, resumableRelativePath, resumableIdentifier,
resumableChunkNumber, resumableTotalChunks, resumableTotalSize, file
```

Each response reports progress:

```json
{ "data": { "status": "uploaded", "received": 2, "total": 4 } }
```

The final chunk returns `201` with the stored entry:

```json
{
  "data": {
    "status": "stored",
    "entry": { "name": "a.bin", "path": "/a.bin" },
    "received": 4,
    "total": 4
  }
}
```

- Chunks are staged in `private/tmp/uploads/` and assembled before the file
  appears in the user's folder, so an interrupted upload never leaves a
  partial file behind. Each upload gets its own staging folder, keyed by the
  user, the destination and the file name as well as `resumableIdentifier`, so
  two clients that send the same identifier — which happens whenever they upload
  same-sized files — cannot mix their chunks.
- Sending the same file again resumes: only missing chunks are needed. The
  resume check reports chunks it holds for that identifier, so a client must
  keep sending the same destination and file name to find them.
- `POST /api/files/upload/cancel` with `{"identifier"}` discards the staged
  chunks. Adding `"filename"` discards only that file's upload.
- Files larger than `frontend_config.upload_max_size` are refused with `413`.
- Existing files are never overwritten (`409`).

### Read for editing

```http
GET /api/files/content?path=/notes.md
```

```json
{
  "data": {
    "path": "/notes.md",
    "name": "notes.md",
    "content": "line one\nline two\n",
    "modified": "2026-10-23T21:19:52Z"
  }
}
```

Requires `read`. Only extensions in `frontend_config.editable` are served.
Binary files (a NUL byte in the first kilobyte) are refused with `400`, and
files over 2 MB with `413`.

### Save
```json
{ "path": "/notes.md", "content": "text" }
```

Only extensions listed in `frontend_config.editable` may be written.

### Directory listing

```json
{
  "data": {
    "path": "/docs",
    "parent": "/",
    "entries": [
      {
        "name": "sub",
        "path": "/docs/sub",
        "type": "dir",
        "size": 0,
        "modified": "2026-10-23T17:30:00Z",
        "mode": "755"
      }
    ]
  }
}
```

- `parent` is `null` at the home folder root, so the UI shows an "up" row only
  when it is non-null.
- `type` is `dir` or `file`.
- `modified` is RFC 3339 / UTC.
- `mode` is the octal permission string as used by `chmod`.
- Entries are sorted directories first, then by name ignoring case.

### Downloads

Files are sent as `attachment` unless their extension is listed in
`download_inline` (default: `pdf`, `jpg`, `jpeg`, `png`, `gif`, `webp`, `bmp`,
`avif`), in which case they are rendered inline with their real content type.
Responses always include `X-Content-Type-Options: nosniff`.

Everything else stays an attachment, which is what keeps an uploaded `.html`
or `.svg` from executing on this origin. That protection is the **default**, not
a ceiling: an administrator can list `svg` or `html` on **Admin → Files →
Inline download extensions**, and then those files are rendered — which means
script in them runs on this origin. A listed extension is rendered with its real
content type; the admin page shows a warning while a risky one is in the list.
`*` renders everything and is the most dangerous setting there is.

Entries are matched as exact extensions: listing `html` does not render
`notes.mhtml`. A leading dot is accepted (`pdf` and `.pdf` are the same), and
the list is stored in dotless lowercase form.

The endpoint decides only how the browser is *told* to present the file. The
interface then opens that URL in a new tab, so a download never replaces the
file manager — see `api.openDownload` in the frontend. Any client is free to
fetch the URL directly.

### Name collisions

Creating, renaming, moving, copying or uploading onto a name that already
exists returns `409` and writes nothing. The caller may instead pick a policy
with `on_collision`:

| Value | Behaviour |
|-------|-----------|
| `refuse` (default) | `409 Conflict`, nothing written |
| `upcount` | next free name: `report.pdf` → `report (1).pdf` → `report (2).pdf` |
| `replace` | the existing entry is deleted first — **destructive** |

```http
POST /api/files/create
{ "parent": "/", "name": "report.pdf", "on_collision": "upcount" }
```

- Accepted by `create`, `mkdir`, `rename`, `move`, `copy` and by uploads as the
  multipart field `resumableOnCollision`.
- `replace` is refused for `mkdir`: deleting a whole folder to create one in
  its place is never intended.
- `archive.tar.gz` becomes `archive (1).tar.gz`: the whole compound extension is
  kept, rather than upcounting the last part alone.
- Uploads decide the final name **after** assembly, because the target can
  appear while a long upload runs. A refusal is still reported on chunk 1, so a
  doomed upload costs one request rather than the whole file.
- Creating never overwrites through a race: the storage layer uses an exclusive
  create, so two simultaneous uploads of one name cannot both succeed.
- The upload protocol itself is described under [Upload](#upload).

### Search

```http
GET /api/files/search?path=/docs&q=report
```

```json
{
  "data": {
    "term": "report",
    "path": "/docs",
    "limited": false,
    "results": [
      {
        "name": "report.txt",
        "path": "/docs/sub/report.txt",
        "type": "file",
        "size": 12,
        "modified": "2026-10-23T17:30:00Z",
        "mode": "644",
        "parent": "/docs/sub"
      }
    ]
  }
}
```

- Walks the tree below `path` (default `/`) on the server. Matching is a
  case-insensitive substring test on the name; both files and directories are
  returned. Content is never searched.
- `parent` is the folder containing the match, so a client can jump straight to
  it: a directory result opens itself, a file result opens its parent.
- Requires the `read` permission. Results can never leave the user's home
  folder, because the walk uses the same confined storage view as listings.
- At most 500 results are returned; `limited: true` means the cap was hit and
  the term should be narrowed.
- A search that runs longer than 30 seconds returns `408`. Closing the client
  request aborts the walk.
- Folders that cannot be read are skipped rather than failing the whole search.

### Archives

Archives use the three generated endpoints `POST /api/files/zip`,
`POST /api/files/unzip` and `GET /api/files/batch`, each described above.

```http
POST /api/files/zip
{ "paths": ["/docs"], "destination": "/", "name": "docs.zip" }
```

The archive is written in place and returned as a file entry. An existing name
is refused with `409` — nothing is ever overwritten. Without `name` the
configured `default_archive_name` is used. Requires `read`, `write` and `zip`.

```http
POST /api/files/unzip
{ "path": "/docs.zip", "destination": "/", "folder": "" }
```

Extraction always happens **inside a new subfolder**, named after the archive
(`docs.zip` → `docs`) unless `folder` is given, so loose files never spill into
the current directory. An existing folder is refused with `409`. Entry names
containing `..`, absolute paths or anything resolving outside the destination
reject the whole archive with `400` (zip-slip protection), as does a corrupt
file. Requires `read`, `write` and `zip`.

```http
GET /api/files/batch?path=/a.txt&path=/docs&name=pick.zip
```

Streams `application/zip` directly; no temporary archive is created and no
archive id is issued. Requires `read`, `download` and `batchdownload`.

## Configuration and languages

These routes are public: a browser needs them before anyone signs in.

<!-- BEGIN apidoc:config -->
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/capabilities` | Which features this deployment supports |
| `GET` | `/api/config` | Public settings the browser needs before login |
| `GET` | `/api/i18n` | Available languages, and the configured default |
| `GET` | `/api/i18n/{lang}` | Messages for one language |
<!-- END apidoc:config -->

`GET /api/config` returns the settings the browser needs before login. Like
every response, it is wrapped in `{"data": …}`:

```json
{
  "data": {
    "app_name": "FileGator",
    "language": "english",
    "language_selection": true,
    "logo": "",
    "upload_max_size": 104857600,
    "upload_chunk_size": 1048576,
    "upload_simultaneous": 3,
    "default_archive_name": "archive.zip",
    "editable": [".txt", ".md"],
    "date_format": "YYYY/MM/DD HH:mm:ss",
    "guest_redirection": ""
  }
}
```

Sensitive settings (session key, IP lists, users file) are never included.

`config_read_only` is `true` when the configuration file cannot be written
(for example on a read-only volume). The server still runs on in-memory
settings, but they are lost on restart: a save from the admin area then answers
`409` with the code `read_only_config`, and the notice bar explains why.
Changing the storage root or the auth provider behaves the same way — the switch
applies to the running server, the call is refused.

`startup_notes` describes what **this start** of the server created or could not
resolve, for example:

```json
{
  "startup_notes": [
    {
      "kind": "info",
      "key": "users.created",
      "text": "Created private/users.json with the default accounts (admin and guest). The default password is published: change it on Users."
    }
  ]
}
```

They are shown in the notice bar to administrators, and are empty on a server
that has been running for a while. The text is written by the server, so it is
in English and names the paths involved; a configured users file is never named
here.

## Administration

These routes require an administrator session. `POST` also requires the CSRF
token.

<!-- BEGIN apidoc:admin -->
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/admin/auth` | Read the auth provider settings |
| `POST` | `/api/admin/auth` | Switch or configure the auth provider |
| `GET` | `/api/admin/browse` | Browse the host filesystem |
| `GET` | `/api/admin/root` | Read the storage root |
| `POST` | `/api/admin/root` | Change the storage root |
| `GET` | `/api/admin/settings` | Read the editable settings |
| `POST` | `/api/admin/settings` | Save the editable settings |
| `GET` | `/api/admin/status` | Configuration warnings |
| `GET` | `/api/admin/storage` | Read the storage backend settings |
| `POST` | `/api/admin/storage` | Switch or configure the storage backend |
| `GET` | `/api/admin/storage/browse` | Browse inside the storage root |
| `GET` | `/api/admin/users` | List users |
| `POST` | `/api/admin/users` | Create a user |
| `POST` | `/api/admin/users/{username}` | Update a user |
| `POST` | `/api/admin/users/{username}/delete` | Delete a user |
| `POST` | `/api/admin/users/{username}/password` | Set a password without knowing the old one |
<!-- END apidoc:admin -->

The browser UI is reached from the **Setup** button, which only appears for
administrators.

### Status warnings

`GET /api/admin/status` reports conditions that are otherwise only visible in
the server log:

```json
{
  "data": {
    "config_read_only": false,
    "users_file_read_only": false,
    "default_admin_password": true,
    "storage_root": "/srv/filegator/repository"
  }
}
```

The admin view shows these as banners. While `default_admin_password` is true,
a **Change password** button opens the administrator's edit form directly.

### Choosing a home folder

Home folders must stay inside the storage root. Use the scoped endpoint:

```http
GET /api/admin/storage/browse?path=/team
```

Paths are storage-relative (`/team/alice`), and anything that would climb above
the root is rejected with `403`. The picker in the Users form starts at the
storage root and has no `..` entry there.

### Choosing a storage root

The root may be **any directory on the server**, not just `repository/`:

```http
GET /api/admin/browse?path=/srv/data
```

```json
{
  "data": {
    "path": "/srv/data",
    "directories": [
      { "name": "projects", "path": "/srv/data/projects" }
    ]
  }
}
```

Only directories are listed — never file names or contents — so the picker
cannot be used to read data. Broken symlinks are skipped; symlinks that point
at a directory are offered.

Select a directory with:

```http
POST /api/admin/root
{ "path": "/srv/data/projects" }
```

The change is validated, saved to `storage.root` and applied immediately, with
no restart. If the path does not exist, is a file or is unreadable, the request
fails and the previous root stays active.

### Users

```http
POST /api/admin/users
{
  "username": "alice",
  "name": "Alice",
  "role": "user",
  "homedir": "/team/alice",
  "permissions": ["read", "write"],
  "password": "secret",
  "create_home": true
}
```

- `role` is `user`, `admin` or `guest`.
- `permissions` are any of `read`, `write`, `upload`, `download`,
  `batchdownload`, `zip`, `chmod`.
- `create_home: true` creates `homedir` (including parents) when missing.
  Without it, a missing folder is rejected with a clear message rather than
  leaving the user locked out.
- On update, an empty `password` keeps the current one.
- Deleting the **last administrator** is refused with `409`.
- Responses never include password material.

## Languages

Language codes are lowercase letters, digits and underscores (for example
`english`, `portuguese_br`). The two endpoints are listed in the generated
table under [Configuration and languages](#configuration-and-languages).

`GET /api/i18n` answers:

```json
{
  "data": {
    "languages": ["arabic", "english", "german"],
    "default": "english",
    "selectable": true
  }
}
```

### Configuring the default language

The administrator sets the default in `frontend_config`:

```json
{
  "frontend_config": {
    "language": "german",
    "language_selection": true
  }
}
```

- `language` — the interface language used when a user has not chosen one.
  If the configured value has no translation file, the API falls back to the
  first available language so the UI still works.
- `language_selection` — when `true` (default), users may override the default
  from their profile. Set it to `false` to lock everyone to the configured
  language.

## Single sign-on

<!-- BEGIN apidoc:sso -->
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/auth/sso` | Signed hand-off from another application |
<!-- END apidoc:sso -->

`GET /auth/sso` is the endpoint another application redirects a browser to. The
URL is signed with a per-application secret; every failure answers the same
`403`. See [Single sign-on]({{< ref "sso.md" >}}) for the ticket format.
