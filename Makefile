BINARY := filegator
BINDIR := bin
ADDR ?= 127.0.0.1:8080
CONFIG ?= private/config.json
# URL prefix to serve under, e.g. "/browser". Empty means the site root, which
# is the default; the server reads base_path from the config when unset.
BASE_PATH ?=

# Keep the Go toolchain's own state out of the repository.
#
# Go derives its cache and config locations from HOME (or XDG_CACHE_HOME and
# XDG_CONFIG_HOME). When HOME points at the project — or is unset — Go writes
# .cache/go-build and .config/go/telemetry into the working tree. Pinning the
# cache and the telemetry directory to the system temp folder makes the build
# independent of whoever's environment it runs in.
#
# XDG_CACHE_HOME and XDG_CONFIG_HOME are set rather than GOTELEMETRYDIR:
# GOTELEMETRYDIR is itself an environment variable recorded in Go's config
# file (`go env -w`), so a saved value silently wins over ours. The XDG
# variables decide where that config file and the telemetry counters live, and
# they are not overridable from inside Go.
#
# TMPDIR is honoured for platforms where /tmp does not exist. All three may be
# overridden from the environment. A stray `go build` outside make can still
# pollute the tree, so .gitignore also covers both paths.
GOTOOLSTATE ?= $(or $(TMPDIR),/tmp)
GOCACHE ?= $(GOTOOLSTATE)/filegator-gocache
GOMODCACHE ?= $(GOTOOLSTATE)/filegator-gomodcache
XDG_CACHE_HOME ?= $(GOTOOLSTATE)/filegator-xdg-cache
XDG_CONFIG_HOME ?= $(GOTOOLSTATE)/filegator-xdg-config
GOFLAGS ?= -mod=vendor

# GOPATH is pinned for the same reason as the rest: Go defaults it to $HOME/go,
# so a stray GOPATH=$PWD (or a HOME pointing here) would record module
# downloads in ./go/pkg/mod. Dependencies are vendored, so in a healthy build
# this cache stays empty — but it must stay empty *outside* the checkout.
GOPATH ?= $(GOTOOLSTATE)/filegator-gopath

export GOCACHE
export GOMODCACHE
export GOPATH
export XDG_CACHE_HOME
export XDG_CONFIG_HOME
export GOFLAGS

.PHONY: help
help:
	@echo "Targets:"
	@echo "  build       Compile the binary into ./$(BINDIR)/$(BINARY)"
	@echo "  run         Build and run the server (ADDR=$(ADDR), BASE_PATH=$(BASE_PATH))"
	@echo "  watch       Rebuild and restart the server when files change"
	@echo "  test        Run all tests"
	@echo "  lint        gofmt + go vet"
	@echo "  fmt         Format all Go files"
	@echo "  docs        Regenerate the endpoint tables, then build the docs site"
	@echo "  docs-serve  Build the docs site and serve it on :8099"
	@echo "  docs-check  Fail if an endpoint table or a docs link is out of date"
	@echo "  clean       Remove build output"
	@echo ""
	@echo "Go cache: $(GOCACHE)"

.PHONY: build
build:
	go build -o $(BINDIR)/$(BINARY) .

.PHONY: run
run: build
	./$(BINDIR)/$(BINARY) -addr $(ADDR) -config $(CONFIG) $(if $(BASE_PATH),-base-path $(BASE_PATH))

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	go fmt ./...

# vendor/ is not our code, so it is left out of the formatting check.
UNFORMATTED = $$(gofmt -l . | grep -v '^vendor/')

.PHONY: lint
lint:
	@test -z "$(UNFORMATTED)" || (echo "Unformatted files:"; echo "$(UNFORMATTED)"; exit 1)
	go vet ./...

# The documentation website is built by internal/docsite, in Go — not by Hugo.
#
# Three commands, one per intention:
#
#   make docs        regenerate the endpoint tables in api.md, then build the site
#   make docs-serve  build it and serve it on :8099
#   make docs-check  verify it — what CI runs
#
# docs-check is not a convenience: it is the job that fails a pull request when
# a route was added without regenerating its table, or a link points at a page
# that does not exist.
DOCS_DIR ?= docs-site

# docs regenerates before building, so the published site never shows a table
# belonging to a previous version of the router. The generator refuses to run
# for an endpoint that has no description, rather than publishing a blank row.
.PHONY: docs
docs:
	go run ./internal/apidoc
	go run ./internal/docsite -out $(DOCS_DIR)

# Building and serving in one invocation: the server reads docs-site/ as it was
# just written, and the template placeholders are resolved once. It does not
# watch; after editing a page, stop it and run it again.
.PHONY: docs-serve
docs-serve:
	go run ./internal/docsite -out $(DOCS_DIR) -serve :8099

.PHONY: docs-check
docs-check:
	go run ./internal/apidoc -check
	go run ./internal/docsite -check

.PHONY: clean
clean:
	rm -rf $(BINDIR)

# watch rebuilds and restarts the server whenever a Go or frontend file
# changes, using scripts/watch.sh (no third-party watchers).
.PHONY: watch
watch:
	ADDR=$(ADDR) CONFIG=$(CONFIG) BASE_PATH=$(BASE_PATH) BINARY=$(BINDIR)/$(BINARY) ./scripts/watch.sh
