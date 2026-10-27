---
title: "Event log"
weight: 35
---

FileGator writes an append-only event log of security-relevant activity. It is
plain [JSON Lines](https://jsonlines.org/): one JSON object per line, so it can
be read with `tail`, `jq` or a log shipper without a custom parser.

## What is recorded

| Event | When |
|-------|------|
| `auth.login` | A successful sign-in |
| `auth.login_failed` | An unknown username or wrong password |
| `auth.login_locked` | An attempt while the address is locked out |
| `auth.logout` | A sign-out |
| `auth.password_changed` | A password was changed, or the change was refused |
| `auth.sso_login` | A single-sign-on hand-off opened a session |
| `auth.sso_failed` | A hand-off was refused (bad signature, expired, reused nonce, unknown application) |
| `security.csrf_failed` | A state-changing request without a valid CSRF token |
| `security.ip_blocked` | A request refused by the IP allow/deny lists |
| `security.permission_denied` | A user tried an action they lack permission for |
| `file.create`, `file.delete`, `file.rename`, `file.copy` | File and folder changes |
| `file.save` | A file saved in the editor |
| `file.upload` | An upload completed |
| `file.zip`, `file.unzip`, `file.batch_download` | Archives and downloads |
| `file.chmod` | Permission bits changed |
| `admin.user_created`, `admin.user_updated`, `admin.user_deleted` | User management |
| `admin.settings_saved` | Settings changed |
| `admin.storage_changed`, `admin.auth_changed` | A backend was switched |
| `server.error` | An unexpected server-side failure |

Plain folder listings and single-file downloads are **not** logged: they are
the bulk of normal use and would bury the events that matter.
`file.batch_download`, `file.zip` and `file.unzip` are logged because they take
a whole selection at once.

Passwords never appear in the log, and neither does file content.

## Record format

```json
{
  "time": "2026-10-25T10:51:11.57245436Z",
  "level": "INFO",
  "msg": "user created",
  "event": "admin.user_created",
  "user": "admin",
  "target_user": "eve",
  "ip": "10.0.0.7",
  "method": "POST",
  "path": "/api/admin/users"
}
```

- `event` is a stable identifier; `msg` is the human-readable description.
  Archive and alert on `event`, not on `msg`.
- `user` is the account that acted. For admin events the affected account is
  `target_user`, so the two are never confused.
- `ip` is the connection's address. It deliberately ignores
  `trusted_ip_header`: an entry cannot be forged by sending a header. Behind a
  reverse proxy the address will therefore be the proxy's.

## Enabling and configuring

The log is on by default and writes **JSON Lines to standard output**, so
`docker logs` or `journalctl` collects it with no extra setup and no file to
rotate. Point `logging.file` at a path to keep a file instead. Change it on
**Admin → Security**:

| Setting | Default | Meaning |
|---------|---------|---------|
| Write an event log | on | Turn it off to log nothing |
| Log file | `-` (stdout) | `-` for standard output; a path (relative to the working directory) for a file |
| Log level | `info` | `debug`, `info`, `warn` or `error` |
| Maximum log size | 0 | With a file: rotate past this size. The field takes **megabytes** and stores bytes, so `10` means 10 MB. `0` disables rotation, and must be `0` with `-` |

### Logging to stdout (the default)

```json
{
  "logging": {
    "enabled": true,
    "file": "-",
    "level": "info",
    "max_size": 0
  }
}
```

Records go to **standard output**; startup messages stay on standard error, so
`docker logs` shows both and either can be redirected separately:

```sh
./bin/filegator | jq -c '{time, event, user}'
```

Rotation is refused together with `-` because the supervisor (Docker, systemd,
journald) owns that file and rotating it behind its back would lose records.
A size above `0` with `-` is rejected at startup with that explanation.

An **empty** log file is different from `-`: it means standard **error**, the
fallback used when the configured file cannot be opened.

To keep the event log in a file, set the log file to `private/logs/app.log` and
the maximum size to `10` MB, matching the older layout.

When logging to a file, it is created with mode `0600`. When it grows past the
limit its contents move to `<file>.1` and the live file starts empty, so disk
use is bounded at roughly twice the limit. Exactly one previous file is kept.

The level applies immediately: switching to `debug` takes effect on the next
request, with no restart.

## If the file cannot be written

The server still starts. It prints a warning explaining that event logging is
disabled and continues logging startup messages to stderr, which `docker logs`
collects. This is deliberate: a read-only volume must not stop the server from
serving files.

The server still runs in that case; set the log file to `-` to send events to
standard output instead. There is no syslog or network sink.

## Reading it

With the default stdout destination:

```sh
# Failed sign-ins this week, from journald
journalctl -u filegator | grep '"event":"auth.login_failed"'

# Everything one user did
docker logs filegator 2>&1 | grep '"user":"alice"' | jq -c '{time, event, path}'

# Who deleted something today
journalctl -u filegator --since today | grep '"event":"file.delete"'
```

With a log file, use the path instead of the service name.

## Limitations

- The log is **append-only, not tamper-proof**: anyone who can write the file
  can edit it. Ship it elsewhere if it must be evidence.
- There is no built-in search or viewer in the admin interface; use the tools
  above. The file is intentionally not served through the file browser.
- Nothing is entered in the log for read-only browsing — listing a folder or
  downloading a single file is not recorded. Bulk downloads
  (`file.batch_download`) and archive creation are, because they move more data
  at once.
