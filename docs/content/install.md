---
title: "Installation"
weight: 10
---

FileGator is a single Go binary. There are two ways to run it: as a **container**
(`filegator/filegator`, see [Docker]({{< ref "docker.md" >}})), or as the
**binary**, built from the repository — there is no prebuilt release download at
this stage.

## Build from source

You need **Go 1.22 or newer** and nothing else — no Node, no PHP, no frontend
build step. Dependencies are vendored, so the build works offline.

```sh
git clone https://github.com/filegator/filegator
cd filegator
make build
```

The binary is written to `bin/filegator`.

The Makefile keeps Go's own state (`GOCACHE`, `GOMODCACHE`, `GOPATH`,
telemetry) under `/tmp/filegator-*` instead of `$HOME`, so a build never writes
into the checkout. Override `GOCACHE`, `GOMODCACHE`, `GOPATH`,
`XDG_CACHE_HOME` or `XDG_CONFIG_HOME` if you want a shared cache.

If you would rather not use make, `go build -mod=vendor -o bin/filegator .`
does the same thing.

## Run it

```sh
./bin/filegator -addr 127.0.0.1:8080
```

`./bin/filegator` with no flags does the same thing — serving is the default
action. There are no subcommands: `./bin/filegator -h` prints the usage, the
flag list and a few examples, and exits 0. A misspelled flag or a stray word
(`filegator addr` instead of `filegator -addr …`) is refused rather than
ignored, so the server never starts listening somewhere you did not ask for.

Open <http://127.0.0.1:8080> and sign in as `admin` with the password
`admin123`. **Change it immediately** — until you do, the server logs a warning
at every start and the interface shows a banner.

### Set the admin password before the first start

The default password is published, so an instance reachable from the internet is
only protected between the first start and your first sign-in — a window a
scanner can find. Set the password in the environment instead:

```sh
FILEGATOR_ADMIN_PASSWORD='a long passphrase of your choosing' ./bin/filegator
```

It applies **only while an administrator still has the default password**. That
makes it safe to leave in a `docker-compose.yml` or a systemd unit: once the
password has been changed — by this variable or by hand — the value is ignored
and a warning in the log says so. It never resets a password you chose later.

- At least 8 characters. A shorter value, or one with leading or trailing
  spaces (almost always a quoting mistake), **stops startup** with the reason;
  it is never silently discarded.
- It applies to **every administrator still using the default password**, not
  just the account named `admin`.
- Only the password changes: name, role, home folder and permissions are left
  as they were, and a guest account is never touched.
- It needs the users file to be writable, which it is on a fresh install.

If you would rather not put the password in the environment, read it from a
file with your shell instead:

```sh
FILEGATOR_ADMIN_PASSWORD="$(cat /run/secrets/filegator_admin)" ./bin/filegator
```

Running it with `-h` prints the version and the flags:

```
$ ./bin/filegator -h
FileGator v8.0.0-beta.1 - a web file manager.
...
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `127.0.0.1:8080` | HTTP listen address. Use `:8080` to listen on every interface, or put a proxy in front |
| `-config` | `private/config.json` | Path to the configuration file |
| `-base-path` | *(site root)* | Serve under a URL prefix, e.g. `/browser` — see [Running under a subpath]({{< ref "subpath.md" >}}) |
| `-frontend-dir` | *(built-in)* | Serve the interface from a folder instead of the built-in copy — see [Customising the interface]({{< ref "customising.md" >}}) |
| `-export-frontend` | — | Write the built-in interface to a folder and exit |
| `-list-admins` | — | Print the administrator accounts and exit — see below |
| `-reset-admin-password` | — | Set an account's password and exit — see below |
| `-init` | — | Create what a first start needs without asking — see below |

### The first start asks first

A binary started in an empty folder has to create things: `private/`, a
configuration file with a session key, the users file and the storage folder. It
**asks before it does**, and lists every path:

```
FileGator needs to create the following, which do not exist yet:
  - private/config.json (with a new session key)
  - the folder private
  - the storage folder repository
Create them now? [y/N]
```

The default is **no**: answering no creates nothing **and the server does not
start**, saying so. Nothing is half-configured and nothing is silently written.

In a container, a service unit or a script there is no terminal to ask on, so
the binary does not start and asks for **`-init`**, which creates what the prompt
would have created. That is what a systemd unit passes, and what the published
image passes in its default command — see [Docker]({{< ref "docker.md" >}}).

There is no flag for "no": the answer is the terminal's, and an installation
whose folders are prepared by hand (or mounted from elsewhere) simply never
reaches the question, so nothing is created and nothing is asked.

An installation that already has its configuration and storage folder never
sees the question, so `-init` is only needed for the very first start.

The users file is copied from `private/users.json.sample` when that file sits
next to it — the answer covers both, so nobody is asked twice. Without a sample
there is nothing to copy: a warning says the built-in accounts are in use for
the run and that they are not saved to disk.

### Behind a proxy

Run the binary on loopback and let nginx, Caddy or Apache terminate TLS. Then:

- set `security.secure_cookies` to `true` so the session cookie is only sent
  over HTTPS (`-addr` alone does not know about TLS), and **restart** — the
  session and CSRF cookie flags are built at startup, unlike the settings that
  apply on the next request,
- set `security.trusted_ip_header` to the header your proxy fills in
  (`X-Real-IP`, `CF-Connecting-IP`, `X-Forwarded-For`), or the IP allow/deny
  lists and the login lockout will see the proxy's address for every visitor,
- pass the prefix through with `-base-path` if the app is not at the site root.

All of these are on **Admin → Security**, and described in
[Configuration]({{< ref "configuration.md" >}}).

## Forgotten admin password

If you are the only administrator and the password is lost, the instance is
otherwise unreachable — there is no reset email and no recovery question. Reset
it from a shell on the server.

### First, find the username

The account may not be called `admin`: it can be renamed, or created by hand in
`users.json`. List the administrators before resetting one:

```sh
./bin/filegator -config private/config.json -list-admins
```

```
Administrators in private/config.json:

  USERNAME  HOME   PASSWORD
  admin     /      STILL THE DEFAULT
  milena    /team  set

To reset one: filegator -reset-admin-password <username>
```

It prints the username, home folder and whether an account still has the
published default password — never a password or a hash. It reads `-config` to
find the users file, and refuses an install whose accounts live in LDAP or a
database, since there is nothing here to list.

### Then reset it

```sh
./bin/filegator -config private/config.json -reset-admin-password milena
```

It asks for the new password twice, without showing it as you type.

- **Any account** can be reset this way, not only an administrator — the point
  is to get back in. The guest is refused: it has no password.
- **Only the named account changes.** Nothing else about it is touched, and no
  other account is affected.
- **Every session for that account ends**, because the session fingerprint covers
  the password hash. Anyone already signed in is signed out.
- **Point `-config` at the file the server uses.** The command reads `auth` from
  it to find `users.json`; with `-config` unset it uses `private/config.json`.
- **The server does not need to be running, and does not need restarting**: the
  users file is read on each request, so the new password works immediately.
- **LDAP and SQL installations are refused** with a message saying so:
  passwords live in the directory or the database, so reset them there.

If the command is not available to you — for example a container you cannot get
a shell into — the account cannot be recovered, so keep a second administrator
account, or a copy of `private/users.json`. Do not delete the file to "start
over": with `-init` (or a "y" at the first-start question) the server recreates
it from `users.json.sample`, which also discards every other account and their
home folders. Without `-init` a missing users file is not recreated at all: the
built-in accounts are used for the run and nothing is written.

## Upgrading

If you are moving from FileGator 7, read
[Upgrading and what's new]({{< ref "upgrading.md" >}}) first: it covers the
upgrade steps, what carries over untouched, what is new, and what is missing
from version 8 with the reason for each.

- **Stop the old process before starting the new one.** Both write
  `private/users.json`.
- **Back up `private/users.json` and your storage root.**
- **Start the binary in the same directory.** The folder layout is unchanged,
  so an existing installation is read as it is.
- **Existing Docker volumes** keep their files, but version 8 mounts them at
  `/data/private` and `/data/repository` and runs as UID 65532 — see
  [Coming from a version 7 image]({{< ref "docker.md" >}}#coming-from-a-version-7-image).

The paths, once running:

| Path | Purpose | Committed? |
|------|---------|------------|
| `private/users.json` | Users, roles, permissions and bcrypt hashes | no |
| `private/config.json` | Configuration: created on the first start (with the session key) and rewritten on every save | no |
| `private/users.json.sample` | Template, copied to `users.json` only when that file is absent | yes |
| `private/logs/` | Optional event log, when `logging.file` names a path | no |
| `private/tmp/` | Upload staging | no |
| `repository/` | Default storage root | only the `.gitkeep` placeholder |

To keep the log file an old installation wrote to `private/logs/app.log`, set
`logging.file` to that path and `logging.max_size` to `10485760`; the default is
JSON Lines on standard output. See [Event log]({{< ref "logging.md" >}}).

## systemd

```ini
[Unit]
Description=FileGator
After=network.target

[Service]
User=filegator
WorkingDirectory=/srv/filegator
ExecStart=/srv/filegator/bin/filegator -addr 127.0.0.1:8080
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Run as a dedicated user that owns `repository/` and `private/`. `private/`
holds password hashes and the session key: keep it readable by that user only
(`chmod 700 private`, `chmod 600 private/users.json`).

## A fresh install

The repository ships with `repository/` containing only a `.gitkeep`
placeholder, an `admin` account with the default password, and a `guest`
account with **no permissions** — so nothing is published until you grant
access (see [Guest access]({{< ref "guest-access.md" >}})). The `.gitkeep` is an
ordinary file: users can see and delete it.
