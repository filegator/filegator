---
title: "Development"
weight: 60
---

## Requirements

- **Go 1.26.2 or newer** — the version in `go.mod`, which CI reads directly.
  Nothing else: no Node, no PHP, no build step for the frontend bundle.
- Dependencies are **vendored** (about 9 MB), so the build works offline.

## Layout

```
main.go                     flags, config load, start
internal/
  router/                   custom HTTP router with a base path
  config/                   config.json load/save/validate
  security/                 sessions, CSRF, IP lists, headers
  auth/                     shared user types; jsonfile/, ldap/, sql/
  storage/                  shared FS types; local/, sftp/, s3/
  api/                      user endpoints
    httpx/                  response envelope, error codes, session helper
    admin/                  /api/admin endpoints
  archiver/                 zip create/extract, zip-slip protection
  search/                   depth-first name search
  logger/                   JSON Lines event log
  app/                      wiring, routing, embedded frontend
    frontend/               HTML, CSS, ES modules, translations
  apidoc/                   generates the endpoint tables in api.md
  docsite/                  builds this documentation website
docs/
  content/                  the pages (Markdown)
  static/                   the bundled Liberation Sans font
repository/                 default storage root (runtime data)
```

The page directory follows the `internal/<feature>/<feature>.go` convention,
with a `_test.go` beside it. Backends that can be swapped live in a subpackage and
implement a shared interface, so handlers never import a concrete one.

## Commands

| Command | Does |
|---------|------|
| `make build` | Build `bin/filegator` |
| `make run` | Build and run (`ADDR=`, `CONFIG=`, `BASE_PATH=`) |
| `make watch` | Rebuild and restart on change |
| `make test` | `go test ./...` |
| `make lint` | `gofmt` check plus `go vet` |
| `make fmt` | Format everything |
| `make docs` | Regenerate the endpoint tables, then build the documentation website into `docs-site/` |
| `make docs-check` | Fail on a stale endpoint table or a broken docs link |
| `make clean` | Remove `bin/` |

The Makefile pins Go's cache, module cache, GOPATH and telemetry under
`/tmp/filegator-*`, so a build never writes into the checkout even when `HOME`
points at it. Running `go` **directly** can still create `./.cache`,
`./.config` and `./go`; all three are git-ignored.

## Running locally

```sh
make run
```

Then open <http://127.0.0.1:8080> and sign in as `admin` / `admin123`.

The first start creates `private/users.json` from the sample. Change the
password, or the server logs a warning at every start.

Under a subpath:

```sh
make run BASE_PATH=/browser
```

## Working on the container

The image is built with `docker buildx` directly — there is no `make` target for
it:

```sh
docker buildx build --load -t filegator/filegator:v8.0.0-beta.1 .
docker run --rm -p 127.0.0.1:8080:8080 -v filegator-data:/data filegator/filegator:v8.0.0-beta.1
```

The `Dockerfile` is deliberately small: a `golang` builder with the **vendored**
modules and `GOPROXY=off`, then a pinned `alpine` that runs as an unprivileged
user (UID 65532). Things worth knowing while writing code:

- **The image has a shell**, so nothing is forbidden by it any more. Keep the
  server free of shelling out anyway: a command that is missing on a slim image
  fails only in production, and the binary should not depend on the base having
  it. Password reset runs `stty` when there is a terminal and reads stdin when
  there is not.
- **The installation is `/data`**, so its paths are `/data/private/...`.
- **The image runs as UID/GID 65532** — a bare numeric ID, no `passwd` entry, and
  cannot chown a mount from inside.
- **Sources are copied by name, never `COPY . .`**, so runtime state cannot reach a
  layer. A new top-level build input has to be added to that list.
- **The builder is `FROM --platform=$BUILDPLATFORM`** and cross-compiles with
  `GOOS`/`GOARCH` from `TARGETOS`/`TARGETARCH`, and the runtime stage has no
  `RUN` at all — only `COPY`. Either of those wrong means buildx runs a
  foreign-architecture command and fails with `exec /bin/sh: exec format error`
  on a host without qemu. No emulation is needed: Go cross-compiles, and the
  runtime just unpacks files.

`docs/content/docker.md` is the operator's page: volumes, ownership, a proxy in
front, and upgrading. Change it in the same commit as the `Dockerfile`.

## Tests

```sh
make test
```

Every package has `_test.go` files. API tests build a real server with
`httptest` and a temporary storage root, so they exercise routing, permissions,
CSRF and confinement together rather than in isolation.

The frontend has **no test runner and no Node**. It is covered by Go tests
only — the asset and module checks in `internal/app/app_test.go` and
`internal/app/modules_test.go`:

- Tests that fetch the page and assert every asset it references is actually
  served
- Assertions on the **served** JavaScript: each module responds, and the text it
  must contain (an `export function`, a class name) is present
- `TestModuleImportsResolve`, which walks every `.js` import and fails on a
  missing target or a bare specifier — the two mistakes no bundler is present
  to catch
- Checks that missing assets answer with the right content type and that `HEAD`
  works

Logic that deserves real tests is kept out of the DOM (see below) so it stays
reviewable, but it is exercised by reading the module rather than executing it.
Nothing in the toolchain runs JavaScript; the browser is the only runtime.

If a piece of logic genuinely needs to be executed in a test, the answer is to
make the Go side responsible for it, or to assert on its observable output over
HTTP — not to introduce a JavaScript runtime.

## Frontend conventions

- **ES modules only**, no framework, no bundler. `js/app.js` is the entry point.
- `js/modules/` holds reusable logic, `js/views/` one file per screen.
- **Keep logic out of the DOM** where you can. `upload.js` is a queue with no
  DOM access and `upload_panel.js` renders it; that split is why upload
  behaviour can be tested at all.
- **Every URL goes through `api.url()`.** Never write `/api/...` directly: the
  app can be mounted under a prefix, and `url()` is the only place that knows.
- Dialogs are built from `dialogRoot()` / `openDialog()` so they behave the same
  way everywhere: × button, Escape, and backdrop tap all dismiss.
- The layout is **mobile-first**. The file table becomes cards below 560px, touch
  targets are at least 44px, and inputs are 16px so iOS does not zoom.
- **The administration area is English only** (decision 2026-10-27): its labels
  are plain English strings in `views/admin.js`, not `t()` keys, and it has no
  entries in `translations/`. The file browser and the login screen are
  translated.
- **Translations are words, not layout.** Every visible string a user sees goes
  through `t("...")`, and `english.json` is the key set: a new string is added
  there and translated in every language file in the same change. Do not put arrows or
  other directional punctuation in a translated string — the back arrow in the
  administration area is a `<span>` beside `t("Back")`, because a left arrow is
  wrong in a right-to-left language. `internal/app/translations_test.go` fails
  when a file's keys differ from English, when a `t("...")` call names a key
  English does not have, or when a translation drops a `{0}` placeholder.

## Configuration and first start

`config.json` is optional: a missing file means built-in defaults. On a
writable folder it is created on the first start, because it carries the session
key, and it is replaced atomically on every save. A folder that does not exist
yet counts as writable when it can be created, so a binary started in an empty
directory works on the first run. A save that cannot be persisted is **refused**
(`409`, code `read_only_config`): the change applies to the running server, but
the caller is told it will not survive a restart, and `Store.Why()` names the
cause instead of only saying "read-only".

**There is no `config.json.sample`**, though there is a `users.json.sample`.
A sample would carry a session key identical on every install, or become a
second description of the defaults that drifts. The reference is the
**Complete example** block in `docs/content/configuration.md`, and
`internal/config/example_test.go` keeps it equal to `config.Default()` in both
directions — a new default must be added there in the same change. The options
inside `sftp`, `s3`, `ldap` and `sql` are exempt from that check because their
own pages document them; the example still has to name each backend.

A first start records what it created as **startup notes** — the configuration
file, the storage folder, the users file copied from the sample — and the
cases it could not resolve: a read-only configuration, a users file that cannot
be created. They appear in the notice bar for administrators and are empty on a
warm start. Because they travel in `/api/config`, a note must never name a
configured users file.

The first start is a **decision**: the binary asks before creating anything, and
`-init` creates without asking. Answering no, or having no terminal and no
`-init`, means it does not start at all. `app.ResolveInit` holds the rule and is
tested without a terminal.

## Adding a storage backend

Implement `storage.Storage` (`Scope`, `EnsureDir`, `Description`,
`Capabilities`) and the `storage.FS` it returns. Map the backend's errors onto
the shared sentinels (`ErrNotFound`, `ErrPermission`, `ErrExists`, …) so
handlers respond identically. Confinement must be enforced **in the backend**,
not in the HTTP layer.

`Capabilities` decides what the UI hides: no `Chmod` means no permission action,
no `ServerBrowse` means no host folder picker.

## Adding an auth provider

Implement `auth.Provider`. Unsupported operations return `auth.ErrUnsupported`,
and `Capabilities` tells the admin UI which actions to hide — that is how LDAP
disables user management without a special case in the handlers.

`SessionFingerprint` must change whenever something that matters to
authorisation changes, so existing sessions are invalidated.

## Working rules and history

The rules for working on the code — guidelines, conventions and the reasons
behind them — live in `AGENTS.md` for contributors. It holds the rules in force
only: a decision is folded into the section it governs rather than kept in a
log, so there is nothing to read past.

**Everything that differs from FileGator 7 — what carries over, what is new and
what is deliberately missing — lives on one page,
[Upgrading and what's new]({{< ref "upgrading.md" >}}).** A behaviour that
changes in a later release updates that page too, in the same change, so there
is exactly one place where a reader can compare versions.

The feature pages elsewhere in these docs describe FileGator 8 as it is, without
history: how a setting works today, not what it replaced.

## Documentation

Docs are Markdown in `docs/content/`; `.md` files are the source of truth and
the site is generated, never edited by hand.

| Command | Does |
|---------|------|
| `make docs` | Regenerate the endpoint tables in `api.md`, then build the site into `docs-site/` |
| `make docs-serve` | Build the site and serve it on <http://127.0.0.1:8099> |
| `make docs-check` | Fail on a stale endpoint table, a broken link or a missing anchor |

These are three intentions, not four: **preview** (`docs-serve`), **publish**
(`docs`), and **verify** (`docs-check`, which is what CI runs). `docs` is the
one that regenerates the endpoint tables first, so a built site never shows a
table belonging to a previous version of the router.

`docs-serve` builds once and then serves `docs-site/`; it does not watch. After
editing a page, stop it and run it again.

The site is built by **`internal/docsite`**, in Go: no Hugo, no Node, no theme
to install. A page starts with a small front matter block (`title`, `weight`)
and the body is the Markdown subset in `internal/docsite/markdown.go` —
headings, paragraphs, fenced code, tables, quotes, nested lists, links, bold and
italics. A construct it does not know is printed as written rather than
dropped, and `docs-check` is what catches that before a reader does.

Two rules keep the docs honest:

- **the endpoint tables in `api.md` are generated** from the router
  (`internal/apidoc`), so a new endpoint cannot be missing from the page;
- **every link and anchor is checked** by `docs-check`, this includes links
  between pages and links to a section on the same page. The checker is
  `internal/docsite`'s `verify`, and it has its own tests: an earlier version
  resolved same-page anchors against the home page and reported nothing.
