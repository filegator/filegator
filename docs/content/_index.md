---
title: "FileGator"
---

FileGator is a self-hosted web file manager: one binary, no runtime
dependencies, and a browser interface for browsing, uploading and editing files
on your own server.

- **Single binary.** Go, standard library plus four vendored dependencies, one
  file to deploy and one process to run.
- **No build step for the frontend.** Plain HTML, CSS and ES modules served from
  the binary itself.
- **Your files stay ordinary files.** Any existing folder can be managed; the
  app writes no index, no metadata and no hidden files into it.
- **Accounts and files are ordinary data.** `private/users.json` holds the
  accounts, any folder on the server can be the storage root, and nothing
  FileGator-specific is written into it.
- **Mobile-friendly.** The listing becomes one-line rows on a phone, dialogs
  become bottom sheets, and every control is at least 44px tall.

## Start here

- [Upgrading and what's new]({{< ref "upgrading.md" >}}) — coming from
  FileGator 7: how to upgrade, what carries over, what is new and what is
  missing
- [First five minutes]({{< ref "first-steps.md" >}}) — password, storage root,
  a user, and HTTPS, in that order
- [Installation]({{< ref "install.md" >}}) — build, run, the folder layout, and
  running it as a service
- [Docker]({{< ref "docker.md" >}}) — the container image: volumes, ownership,
  a proxy in front, and upgrading it
- [Configuration]({{< ref "configuration.md" >}}) — every setting in
  `config.json`, including the ones with no admin screen
- [Users, roles and permissions]({{< ref "users.md" >}}) — accounts, home
  folders and confinement

## Setting it up

- [Storage backends]({{< ref "storage.md" >}}) — local, SFTP or S3
- [Authentication]({{< ref "authentication.md" >}}) — users file, LDAP,
  MySQL, PostgreSQL
- [Database users]({{< ref "database.md" >}}) — the SQL schema
- [Guest access]({{< ref "guest-access.md" >}}) — publish a folder without a
  login
- [Running under a subpath]({{< ref "subpath.md" >}}) — serving from
  `example.com/browser/`
- [Single sign-on from other apps]({{< ref "sso.md" >}}) — let a CMS or panel
  log users in
- [Customising the interface]({{< ref "customising.md" >}}) — logo, title,
  colours, and serving an edited copy without rebuilding

## Operating it

- [Event log]({{< ref "logging.md" >}}) — sign-ins, changes and refusals, in
  JSON Lines
- [HTTP API]({{< ref "api.md" >}}) — the JSON endpoints
- [Development]({{< ref "development.md" >}}) — layout, tests and conventions

## Project

- [License and contributing]({{< ref "license.md" >}}) — MIT, and signing off
  your commits

## What it does

| | |
|---|---|
| Browsing | Folders, breadcrumbs, folder tree, search across the whole home folder, column sorting |
| Files | Upload (resumable, pausable), download, rename, move, copy, delete, permissions |
| Archives | Zip a selection, extract an archive, download many files as one zip |
| Preview | Image gallery with keyboard and swipe navigation; PDFs inline |
| Editing | Text editor for the extensions you allow, with Ctrl/Cmd+S |
| Admin | Users, interface, file handling, security, repository and auth, all from the browser |
| Security | Signed-cookie sessions, CSRF protection, IP allow/deny lists, lockout, event log |

Deletion is permanent — there is no trash — and nothing is ever overwritten
without being asked first.

## Access without accounts

Two ways to reach files without signing in, both **off until you turn them on**:

- [Guest access]({{< ref "guest-access.md" >}}) — anonymous visitors browse and
  download as the `guest` account, read-only.
- [Single sign-on]({{< ref "sso.md" >}}) — another application signs a ticket
  and FileGator opens a session for that user.

Public share links are **not implemented**.
