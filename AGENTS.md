# AGENTS.md — working on FileGator v8

This file is for anyone — human or AI agent — contributing to FileGator. It
records the project's working guidelines, the decisions behind them, and the
house style the code already follows. When a decision changes, update this file
in the same change. `docs/content/development.md` is the public-facing summary
and must stay in sync with it.

## What this project is

FileGator is a self-hosted, open-source web file manager. Version 8 is a
**rewrite**: a single Go binary serving plain HTML, CSS and modern JavaScript,
with no build step for the frontend. The rewritten project optimises for
**security, low maintenance and simplicity in development**, in that order.

Read `README.md` and `docs/content/` first. The docs are the source of truth
for behaviour; this file is the source of truth for how to work on the code.

## Non-negotiable guidelines

These are the project rules. Treat them as constraints, not preferences.

### Backend

- **Go and the standard library only.** No new external dependency unless it is
  impossible to do without. Answer "do we really need it?" before adding one.
- **Every new dependency needs approval.** The five direct dependencies
  (`go-ldap/ldap/v3`, `go-sql-driver/mysql`, `jackc/pgx/v5`, `pkg/sftp`,
  `golang.org/x/crypto`) exist because each ships a protocol or primitive the
  standard library does not: LDAP, MySQL wire protocol, PostgreSQL wire
  protocol, SSH/SFTP, and bcrypt. Any addition must clear the same bar and be
  added with `go mod vendor` (dependencies are **vendored**; builds run
  offline with `-mod=vendor`).
- **Every feature is tested** with `_test.go` files beside the code. No feature
  is done without tests.
- **Path confinement is enforced inside the storage layer**, never in the HTTP
  handler. That is why no endpoint can be written that forgets it.

### Frontend

- **Pure HTML, CSS and modern JavaScript only.** No framework, no bundler, no
  transpiler, no external dependency.
- **Split JavaScript into separate modules** so a developer can find things
  quickly: `js/modules/` for reusable logic, `js/views/` for one screen each.
  See [Frontend conventions](#frontend-conventions).
- **Frontend tests must be as fast as possible.** They are Go tests; the whole
  suite currently runs in well under a minute.
- **Logic stays out of the DOM** where possible, so it is testable by reading
  the module. `upload.js` is a queue with no DOM access; `upload_panel.js`
  renders it.

### Tooling

- **Go tooling only.** No Node, npm, yarn, Hugo, webpack, or any other
  third-party tool. The documentation site is built by `internal/docsite` in Go
  within the project's own module (`go run ./internal/docsite`), not imported
  from another repository.
- The `Makefile` is the entry point: `make build`, `run`, `watch`, `test`,
  `lint`, `fmt`, `docs`, `docs-serve`, `docs-check`, `clean`. Docker is not part
  of it: the image is built with `docker buildx` directly, as
  `docs/content/docker.md` documents.

### Working process

- **Ask before deciding.** Anything that needs attention, approval, or is a
  software design decision must be asked, not assumed. Preferred method:
  ask **one question at a time**, wait for the answer. Do not ask many decisions
  at once.
- **If an unanswered question matters, ask again.** The user tends to forget,
  and an answer is not implied by silence.
- **Update this file with every new decision or change in plan.** Fold the
  decision into the section it governs, rewritten as the rule in force, and
  mirror it on `docs/content/development.md`. There is no history section: past
  entries are merged into the rules they produced, and superseded ones are
  dropped.
- **Keep the documentation in sync.** Do not let docs drift from behaviour. A
  behaviour change updates the relevant docs page in the same change.
- **Keep all language files in sync.** If anything is missing, translate it in
  every `.json` under `internal/app/frontend/translations/`. See
  [Translations](#translations).
- **Write a plan before large work, and split it into clear steps.**
- **Warn about breaking changes and document the upgrade path.** Anything that
  changes behaviour a user depends on gets a note in
  `docs/content/upgrading.md` and an explicit warning in the response.
- **Do not commit with git.** When a big section is done, ask the user to review
  and commit. The user reviews and commits themselves.
- **Do not rely on git history** — it is not a source of truth for work.

## Repository layout

```
main.go                     flags, config load, start
help.go                     flag set and help screen
internal/
  router/                   small HTTP router with a base path
  config/                   config.json load/save/validate
  security/                 sessions, CSRF, IP lists, security headers
  auth/                     shared user types + Provider interface
    jsonfile/               users.json provider (bcrypt)
    ldap/                   LDAP provider
    sql/                    MySQL and PostgreSQL provider
  storage/                  shared FS/Storage interfaces and sentinels
    local/                  local filesystem backend
    sftp/                   SFTP backend
    s3/                     S3 backend (SigV4 signing, no SDK)
  api/                      user-facing API handlers
    httpx/                  response envelope, error codes, session helper
    admin/                  /api/admin handlers
  archiver/                 zip create/extract with zip-slip protection
  search/                   depth-first name search
  logger/                   JSON Lines event log
  app/                      wiring, routing, embedded frontend
    frontend/               HTML, CSS, ES modules, translations
  apidoc/                   generates the endpoint tables in api.md
  docsite/                  builds the documentation website
docs/
  content/                  the documentation pages (Markdown)
  static/                   bundled fonts and images
repository/                 default storage root (runtime data, git-ignored)
private/                    runtime state: config.json, users.json, logs, tmp
scripts/watch.sh            dependency-free rebuild-and-restart loop
vendor/                     vendored modules (never edit)
```

Convention: `internal/<feature>/<feature>.go` with `_test.go` beside it.
Backends that can be swapped implement a shared interface and live in a
subpackage, so handlers never import a concrete one.

Runtime folders `private/` and `repository/` are git-ignored except for the
`users.json.sample` template and a `repository/.gitkeep` placeholder.

## Conventions verified in the code

Follow the surrounding style. These are patterns already used throughout:

- **Errors:** define sentinel errors (`var ErrX = errors.New("pkg: ...")`) and
  map backend errors onto them. Client-safe errors carry a status and a stable
  code (`httpx.CodedError`, `api.Errorf`); never leak internal errors to the
  client.
- **API responses:** success is `{"data": ...}`, failure is
  `{"error": {"message", "code"}}`, 204 for no content. Use the helpers in
  `internal/api/httpx` / `internal/api/response.go`.
- **Routes:** registered with the router via a `XxxRoutes(rt *router.Router)`
  method on the package's server struct. `RegisterAPIRoutes` in
  `internal/app/routes.go` is the single list; the docs generator uses the same
  list. A new endpoint **must** get a description in `internal/apidoc/main.go`
  or `make docs-check` fails — that is intentional.
- **Capabilities:** backends/providers report what they support; the UI hides
  what is unsupported. Do not special-case a provider in a handler.
- **Comments:** explain *why*, not *what*. Existing code does this consistently;
  match it. Prefer a short paragraph over a restated line of code.
- **Saves that cannot be persisted are refused.** A handler never answers 200
  for a change it could not save: the read-only configuration returns `409`
  with `read_only_config`, the same as changing the storage root or the auth
  provider on a read-only volume. The in-memory change still applies to the
  running server, but the caller is told it will be lost.
- **Tests:** use `t.TempDir()` and absolute paths so tests never write next to
  the source. API tests build a real server with `httptest`, a temporary
  storage root and `config.Load`. Do not point tests at `private/` or
  `repository/`.
- **Formatting:** gofmt + `go vet`, both enforced by `make lint`. `vendor/` is
  excluded from the gofmt check.

## Configuration and first start

- `config.json` is **optional**: a missing file means built-in defaults, on any
  platform. On a writable folder it is created on the first start (it carries
  the session key) and is atomic on every save (`config.json.tmp` + rename,
  mode `0600`). It is never created on a read-only folder; the server runs on
  in-memory defaults instead.
- **A missing configuration folder counts as writable** when it can be created.
  `canWriteDir` probes the nearest directory that exists, because the writer
  calls `MkdirAll`: probing `private/` as if it existed marked every first start
  read-only, and made `config.json` appear only on the second start. A folder
  below an unwritable parent is still read-only, and `Store.Why()` names the
  cause (`… is not writable`, `… is not a folder`) for the messages shown to the
  administrator.
- A save that cannot be persisted is **refused**: `409` with
  `read_only_config`. Settings, the storage root and the auth provider all
  behave the same way — the change applies to the running server, the caller is
  told it will be lost.
- **No `config.json.sample`, unlike `users.json.sample`.** A sample would carry
  a session key shared by every install that copied it, or become a second
  description of the defaults that goes stale; the file is created by the first
  start instead. The reference an operator reads is the **Complete example**
  block in `docs/content/configuration.md`, and
  `internal/config/example_test.go` keeps it equal to `config.Default()` in both
  directions. A new or changed default updates that block in the same change.
  The `sftp`, `s3`, `ldap` and `sql` subtrees are exempt because `storage.md`,
  `authentication.md` and `database.md` own those options — the example must
  still name each backend, as `{}` if need be. `config.json` is strict JSON with
  no comment support, so the block never carries comments; explanations go in
  the prose around it.
- A first start records what it created or could not resolve as **startup
  notes** (`internal/app/startup_notes.go`): the configuration file, the storage
  folder, the users file copied from the sample, and the read-only cases. They
  travel in `/api/config`, are shown in the notice bar to administrators, and
  are empty on a warm start. A note that will be in that public payload must
  never name a configured users file: `TestPublicConfigOmitsSecrets` fails.
- **The first start is a decision, not a side effect.** A binary in an empty
  folder must not create folders behind the operator's back: it asks on the
  terminal (default: no), and `-init` answers yes up front. There is **no flag
  to decline** — the terminal answer is the operator's, and an installation
  prepared by hand never reaches the question. Without a terminal and without
  `-init` it does not start, and says to pass `-init`. Declining creates nothing
  and ends the run: there is no in-memory mode, no partially created install and
  nothing to configure later. Fail fast, with a message that names the flags.
  `app.ResolveInit` decides (it is pure and tested); the terminal detection
  lives in `main`'s `init_terminal_unix.go`, because it is about the process,
  not the application.
- `private/` and `repository/` are created as needed **once the first start is
  allowed**, so a binary started in an empty folder works on the first run. That
  is also what makes a wrong `-config` path look like a working instance: the
  startup notes are how the administrator finds out.

## Containers

- **Keep the `Dockerfile` small.** Two stages: `golang` with the vendored modules
  and `GOPROXY=off`, then a pinned `alpine` (`3.22`, bumped deliberately — never
  `latest`). `CGO_ENABLED=0` for a static binary, so Alpine's musl is never
  loaded; `TARGETOS`/`TARGETARCH`/`TARGETVARIANT` for multi-platform builds. No
  entrypoint script, no `.dockerignore`, no compose file, no image tests, no
  version check in the build, and nothing in the `Makefile`.
- **Alpine over distroless** for the shell (`docker exec`, password reset's
  `stty`) and the CA bundle the TLS backends (S3, LDAP, SSO, SFTP) need, and
  because it publishes `386` and `arm/v6`, which distroless does not.
- **Published platforms: `linux/386`, `linux/amd64`, `linux/arm/v6`,
  `linux/arm/v7`, `linux/arm64`.** All five are covered by `alpine` and build
  statically; check a new base still publishes every platform in the list, or
  the build fails at the runtime stage. `arm/v6` and `arm/v7` need `GOARM` from
  `TARGETVARIANT` (buildx says `v6`/`v7`, Go wants `6`/`7`): without it both get
  the builder's default, and an `arm/v6` image then ships ARMv7 code that does
  not run on the board it is meant for. Adding a platform is the user's
  decision.
- **The runtime stage has no `RUN`, only `COPY`.** A `RUN` there executes as the
  target architecture, so `buildx --platform` for anything but the host fails
  with `exec /bin/sh: exec format error` on a machine without qemu. Directories,
  their ownership and the users sample are prepared in the builder stage (which
  runs as `$BUILDPLATFORM`) and copied across with `--chown`. Do not add a `RUN`
  below the runtime `FROM`.
- **The builder stage is `FROM --platform=$BUILDPLATFORM`.** It must run on the
  host's architecture and cross-compile with `GOOS=$TARGETOS GOARCH=$TARGETARCH`
  (declared as `ARG` inside the stage, which is what brings the global-scope
  platform args into scope). Without `--platform=$BUILDPLATFORM`, buildx starts a
  foreign-architecture `golang` container and the first `RUN` dies with
  `exec /bin/sh: exec format error` on any host without qemu/binfmt — no
  emulation is needed, so do not add one.
- **The build copies sources by name, never `COPY . .`**, so `private/` (password
  hashes, the session key), `repository/`, `bin/` and `docs-site/` cannot reach a
  layer. A new top-level build input must be added to that list.
- **The installation is `/data`.** `private/` and `repository/` resolve against
  the working directory, and `WORKDIR` is `/data`. The users sample is copied to
  `/data/private/users.json.sample` so the first start turns it into
  `users.json`; `-init` is in the default command because a container has no
  terminal for the first-start question.
- **The image runs as a bare UID/GID 65532** — no `passwd` entry, because the app
  never reads the user database. It cannot chown a mount from inside; a bind
  mount is the operator's `chown`, documented in `docs/content/docker.md`.
- **`/data/private` and `/data/repository` are created in the build stage and
  `COPY --chown=65532:65532` into the image.** A directory the image does not
  create is made by Docker as root when it becomes a mount point, and a volume
  seeded from it is root-owned too: the server then cannot write its users file
  and refuses to start. `docker run -p 8080:8080 <image>` with no volume must
  work, so this is not optional.
- **No `VOLUME`.** Declaring one makes `docker run` without `-v` create anonymous
  volumes that `--rm` destroys with the container, while looking persistent.
  Without it, a run with no `-v` writes to the container layer, where temporary
  is obvious.
- **Docs:** `docs/content/docker.md` describes the image and changes with it.

## Frontend conventions

- **ES modules only.** `js/app.js` is the entry point; `index.html` loads it
  with `type="module"`.
- `js/modules/` = reusable logic, `js/views/` = one file per screen.
- **Many small files by responsibility**, not one large file. Upload is split
  into the queue (`upload.js`), rendering (`upload_panel.js`), collision policy
  (`collision.js` / `collisions.js`) and the UI dialog
  (`permissions_dialog.js`, `row_menu.js`, etc.).
- **Every URL goes through `api.url()`.** Never write `/api/...` directly;
  `TestNoHardcodedAPIPaths` will fail. The app can be mounted under a prefix
  and only `url()` knows where.
- **Downloads go through `api.openDownload()`** — a download opens in a new
  tab and never navigates the file manager away.
- **Dialogs** are built from `dialogRoot()` / `openDialog()` so ×, Escape and
  backdrop tap all dismiss.
- **Mobile-first:** the table becomes cards below 560px, touch targets ≥ 44px,
  inputs 16px so iOS does not zoom.
- **Translations:** every visible string goes through `t("...")`. See below.

## Translations

Language files live in `internal/app/frontend/translations/`. They are
discovered from the directory (no registration list), and `english.json` is the
reference: every key in it must exist in every other file, translated. A
missing key falls back to printing the raw key, so an out-of-sync file is a
visible bug in that language.

The key set is exactly what the code shows, nothing more: `english.json` must
carry every key the JavaScript uses, and a key no `t("...")` call names is
removed from every file rather than left behind. `english.json` has drifted
before in both directions (keys the code used that it lacked, keys the code no
longer used that it kept), which is why the call sites are checked against the
files and not only the files against English.

**The administration area is English only** (user decision 2026-10-27). Its
labels are plain English strings in `js/views/admin.js`, never `t()` keys, and
it has no entries in `translations/`. `admin.js` therefore contains no `t()`
call at all — table headers such as `Username`, `Home` and `Actions` included.
Only the file browser, the login screen and their dialogs are translated. The
keys the admin area shares with the browser (`Cancel`, `Save`, `Delete`, `Edit`,
`Permissions`, `Home`, `Actions`, `Username`, `Password`, `Folder`,
`Select Folder`, `Done`, `Create`, `New password`, …) stay translated because
the browser shows them too; a key used only by the admin area is removed from
every language file.

Admin errors are not translated either: admin toasts and inline errors show the
server message as sent (`err.message`), because the validation strings an
administrator can trigger (`app name must not be empty`, `session lifetime must
be positive`, `cannot delete the last administrator`, …) are for operators, not
for translated end users. The browser still translates its errors through
`t(err.message)`, and `profile.js` still translates
`changing your own password is disabled`.

English is the key set: all language files carry exactly the same keys, in the
same order. Two tests keep it that way (`internal/app/translations_test.go`):
one fails when a file's keys differ from English's, and one fails when a
`t("...")` call in the JavaScript names a key English does not have. A third,
`TestAdminIsEnglishOnly` in `internal/app/modules_test.go`, fails on any `t()`
call in `admin.js`, so a missed admin label cannot slip through again.
Placeholders (`{0}`, `{1}`) must match English unless a translation
deliberately keeps an extra one for a value the server fills in
(`upload_panel.js`).

Translations are words, not layout. Do not put arrows, bullets or other
directional punctuation inside a translated string: a left arrow is wrong in a
right-to-left language and belongs in the markup. `renderTabs()` in `admin.js`
draws the back arrow as `<span class="tab-arrow" aria-hidden="true">←</span>`
beside plain `Back` for that reason, and `TestAdminNavIsBelowTheHeader` fails if
an arrow returns to a label.

## Tests and CI

- `make test` → `go test ./...` (about 20 s here).
- `make lint` → gofmt check + `go vet ./...` (blocks a PR).
- `make docs-check` → regenerate endpoint tables in check mode, build the docs
  site, verify every internal link and anchor (blocks a PR).
- `make build` → `go build -o bin/filegator .` (blocks a PR; tests can pass
  while the entrypoint fails).
- CI (`.github/workflows/tests.yml`) runs the four above in one blocking `test`
  job with `GOPROXY=off` and `GOFLAGS=-mod=vendor`, on the Go version read from
  `go.mod`. The `race` job runs `go test -race ./...` with
  `continue-on-error: true` — reported, not enforced.
- The frontend is tested only through Go: served-asset checks, module content
  assertions, import resolution, and the "no hardcoded API path" guard
  (`internal/app/modules_test.go`, `internal/app/app_test.go`,
  `internal/app/frontend_source_test.go`).

## Documentation

- Docs are Markdown in `docs/content/`; the site is generated, never edited by
  hand. The Markdown subset is in `internal/docsite/markdown.go`.
- Two rules keep docs honest: the endpoint tables in `docs/content/api.md` are
  **generated** from the router (`internal/apidoc`), and every link and anchor
  is **checked** (`make docs-check`).
- `docs/content/upgrading.md` is the one place where version 7 and version 8
  are compared (what carries over, what is new, what is missing, and the
  upgrade path). Any behaviour change or breaking change updates it in the same
  change.
- The rest of the docs describe version 8 as it is, without history.
- `docs/content/development.md` must stay in sync with this file.

## Open decisions (do not decide alone)

From `README.md` and the code, these are unresolved. Each needs the user's
answer; ask one at a time.

- **Race detector in CI:** blocking or nightly? The suite passes under `-race`,
  but `internal/api` alone takes minutes. (Currently non-blocking.)
- **Test coverage floor** for packages that hold authorisation logic.
- **Solo mode** (no-login single-user mode): agreed in outline, not built. Open
  question: config key or command-line flag?
- **Sharing / public links:** not built and never in v7. Four possible scopes
  need scoping answers first.
- **S3 multipart upload** and **browser-direct (presigned) uploads:** not
  implemented.
- **LDAP against a real directory**, and **MySQL/PostgreSQL integration tests:**
  no server available in this environment; gate behind an environment variable.

## Breaking changes and upgrade path

Whenever a change could break an existing installation, do all of the
following in the same change:

1. State it plainly in the response or PR, before the details.
2. Document it in `docs/content/upgrading.md` (the comparison page) and in
   whichever feature page it touches.
3. Give the concrete upgrade steps — file format changes, config migration,
   session invalidation, re-exporting a custom interface, etc.
4. Test both the old and new path where it is feasible (for example, a v7
   `users.json` still loads).

## What the user expects at the end of a section

- A short summary of what changed and why.
- Any decisions still open, one at a time.
- A reminder to review and commit (the agent does not run `git commit`).
- Confirmation that `make lint`, `make test` and, when docs changed,
  `make docs-check` pass.
