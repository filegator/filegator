<p align="center">
<img src="https://raw.githubusercontent.com/filegator/filegator/master/dist/img/logo.svg">
</p>

<p align="center">
<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License"></a>
</p>


# FileGator v8

FileGator version 8 was rewritten from the ground up, replacing the PHP/Vue stack with Go only — a single self-contained binary serving static HTML, CSS and modern JavaScript, with no build step and no frontend framework.

## Sponsors & Backers
FileGator is a free, open-source project. It's an independent project with its ongoing development made possible entirely thanks to the support by these awesome sponsors:

<table align="center">
  <tbody>
    <tr>
      <td align="center" valign="middle">
        <a href="https://www.hostg.xyz/aff_c?offer_id=815&aff_id=244019" target="_blank">
          <img title="Hostinger" width="177px" src="https://filegator.io/img/hstngr.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://www.linkpreview.net/?ref=github.com%2Ffilegator%2Ffilegator" target="_blank">
          <img title="Preview Web Links with our Free API service. Get JSON Response for any URL" width="200px" src="https://www.linkpreview.net/images/logo-dark.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://correctme.app/?ref=github.com%2Ffilegator%2Ffilegator" target="_blank">
          <img title="Free Online Grammar and Spell Checker" width="177px" src="https://correctme.app/static/img/logo.svg">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://interactive32.com/?ref=github.com%2Ffilegator%2Ffilegator" target="_blank">
          <img title="Modern approach to software development" width="177px" src="https://interactive32.com/images/logo.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://www.vanillavoice.com/?ref=github.com%2Ffilegator%2Ffilegator" target="_blank">
          <img title="VanillaVoice - Turn any Text into Human-Sounding Speech" width="200px" src="https://www.vanillavoice.com/static/img/logo.svg">
        </a>
      </td>
    </tr>
  </tbody>
</table>

## Typical use cases
- share a folder with colleagues, your team, friends or family
- give students access to upload their work
- allow workers to upload field data / docs / images
- use as cloud backup
- manage cdn with multiple people
- use as sftp replacement
- manage s3 or other 3rd party cloud storage
- use to quickly zip and download remote files

# Documentation
Check out the official [Documentation](https://docs-v8.filegator.io/) on how to download, install, and use FileGator.

## Features & Goals
- Multiple storage adapters (Local Disk, SFTP, Amazon S3)
- Multiple auth adapters with roles and permissions (Store users in json file, database or use LDAP/Active Directory)
- Adjustable, single page front-end built with plain JavaScript, HTML and CSS (no frontend framework).
- Zip and bulk download support
- No database required

# Try it out with docker
```
docker run --rm -p 8080:8080 filegator/filegator:v8
visit: http://127.0.0.1:8080 login as admin/admin123
```
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

## Show your support

Please ⭐️ this repository if this project helped you!

## Security

If you discover a security vulnerability, please report it privately via GitHub Security Advisories instead of using the public issue tracker:

https://github.com/filegator/filegator/security/advisories

## License

Copyright (c) 2026 [Milos Stojanovic](https://github.com/alcalbg) and contributors.

This project is licensed under the **MIT License**. See [LICENSE](LICENSE) for
the full text.

## Contributing

Contributions are welcome under the same MIT License.
