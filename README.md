<p align="center">
<img src="https://raw.githubusercontent.com/filegator/filegator/master/dist/img/logo.svg">
</p>


# FileGator v8 (beta)

FileGator version 8 was rewritten from the ground up, replacing the PHP/Vue stack with Go only — a single self-contained binary serving static HTML, CSS and modern JavaScript, with no build step and no frontend framework.

# Documentation
Documentation link while in beta: 
[https://docs-v8.filegator.io/](https://docs-v8.filegator.io/)

# Version 8 highlights

- **Why the rewrite:** keep the project simple and easy to maintain for the long term, after the PHP/Vue setup had grown harder to evolve and harder to deploy.
- **Simpler:** one binary, one runtime, one deployment target instead of a PHP runtime, Apache2 and a JavaScript toolchain.
- **Fewer third-party libraries:** Go standard library only — far less to track and keep up to date.
- **Easier maintenance:** much less code, no framework upgrades, no frontend build pipeline.
- **Security, kept a first-class concern with fewer places to go wrong:** path confinement enforced inside the storage layer, only bcrypt password hashes accepted, stateless signed session cookies, CSRF protection, IP allow/deny lists and configurable security headers built in.
- **Easy upgrade:** `private/users.json` keeps exactly the same format and the familiar `private/` plus `repository/` layout still works, including Docker volumes mounted from an older installation. What carries over, and what does not, is on one page: [Upgrading and what's new](docs/content/upgrading.md).
- **Easy configuration:** storage and auth are set up from the admin interface instead of hand-edited config files.
- **Trade-off:** you can no longer copy a few PHP files to a cheap shared hosting account and have it run — FileGator now needs somewhere to run a single Go binary, such as a small VPS, a container or a systemd service.
- **How the rewrite was made:** the version 8 rewrite was done with LLM assistance (an AI coding agent) working under the project's written guidelines in [AGENTS.md](AGENTS.md), with the test suite, `make lint` and `make docs-check` as the gate. The code is human-reviewed and human-committed.

## Status and remaining work

FileGator v8 is usable: the file browser, admin area, all three storage
backends (local, SFTP, S3), all three auth providers (users file, LDAP, MySQL,
PostgreSQL), SSO handoff, guest access, the event log and the documentation site
are in place, with the test suite and `make lint` / `make docs-check` / `make
build` green in CI.

What is left is below.

### Open — needs a decision

- [ ] **Race detector in CI: blocking or nightly?** The suite passes under
      `-race` on every package (`internal/api` alone takes ~3.5 minutes), so the
      job works; the question is whether to make it a required check and double
      every pull request's wait, or to run it on a schedule instead.
      Needs a number for the packages that hold authorisation logic.
- [ ] **Sharing / public links**: not built yet. Four possible
      scopes (file link, folder link, logged-in-only, upload drop-box) still need
      the scoping questions answered first.

### Open — need testing against real services

- [ ] **LDAP against a real directory**: the mapping logic is unit-tested, the
      connection paths are not.
- [ ] **MySQL / PostgreSQL integration tests**: test against the real connection.
- [ ] **S3 multipart upload**, for objects over the current 5 GiB single-request
      limit, and **browser-direct (presigned) uploads**. Neither is implemented.
      Test against a real S3-compatible service.


## License

FileGator v8 is licensed under the **MIT License**. See [LICENSE](LICENSE) for
the full text.

### Contributing

Contributions are welcome under the same MIT License.
