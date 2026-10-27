---
title: "Storage backends"
weight: 25
---

A storage backend is where a user's files actually live. It is chosen in
**Admin → Repository**, which validates the settings before switching: if the new
backend cannot be reached or its root cannot be read, the previous one stays
active and nothing is saved.

## What stays the same

Every backend sees the same thing: one root, folders that are real directories,
and users confined to their home folder. A folder under `homedir` is reachable
with `/` and nothing above it — `..`, absolute paths and symlinks cannot escape,
and the check lives in the backend, not in the HTTP layer.

Files look identical whichever backend is used, so switching does not change how
the interface behaves.

## Local folder (default)

```json
{ "storage": { "type": "local", "root": "repository" } }
```

| Key | Meaning |
|-----|---------|
| `root` | Folder on this machine. Relative to the working directory, or absolute |

The admin can point it at **any folder on the server** with a folder picker, and
the change applies immediately. Use an absolute path in production so a
different working directory cannot change what users see.

Permissions are the operating system's. FileGator runs as one user, so that user
must be able to read and write everything inside the root.

## SFTP

```json
{
  "storage": {
    "type": "sftp",
    "sftp": {
      "host": "files.example.com",
      "port": 22,
      "user": "filegator",
      "password": "",
      "private_key": "/etc/filegator/id_ed25519",
      "known_hosts": "/etc/filegator/known_hosts",
      "timeout": 30
    },
    "root": "/srv/files"
  }
}
```

| Key | Default | Meaning |
|-----|---------|---------|
| `host`, `port` | — , `22` | SSH server |
| `user` | — | Login name |
| `password` or `private_key` | — | One is required. The key must be unencrypted PEM/OpenSSH |
| `known_hosts` | `""` | **Set this in production.** Empty accepts the host key on first use, which is convenient and vulnerable to interception |
| `timeout` | `15` | Seconds for connecting and for individual operations. `0` means the default |
| `root` | — | Remote folder served as `/` |

SFTP has no server-side copy, so copying streams through FileGator (bounded to
4 MB in memory). Large folders copied over a slow link therefore take as long as
downloading and uploading them.

## S3

```json
{
  "storage": {
    "type": "s3",
    "s3": {
      "bucket": "my-files",
      "region": "us-east-1",
      "endpoint": "",
      "access_key": "…",
      "secret_key": "…",
      "use_path_style": false,
      "prefix": "team",
      "timeout": 60
    }
  }
}
```

| Key | Meaning |
|-----|---------|
| `bucket` | Bucket name |
| `region` | AWS region. Some compatible services ignore it but require a value |
| `endpoint` | Override the AWS host, e.g. `https://nyc3.digitaloceanspaces.com`. Empty means AWS |
| `access_key`, `secret_key` | Credentials |
| `use_path_style` | Put the bucket in the path instead of the host name. **MinIO and most self-hosted services need `true`** |
| `prefix` | Key prefix, so one bucket can hold several repositories |
| `timeout` | Seconds per request. Default `60`; `0` means the default |

S3 has no directories. Folders are placeholder objects with a trailing `/`;
FileGator creates and lists them so the interface still shows a folder tree.

### Limitations

- **Single objects are capped at 5 GiB.** Multipart upload is not implemented, so
  a larger file fails with a clear error rather than silently truncating.
- **`chmod` is unavailable.** Object stores have no permission bits, so the
  action is hidden.
- **Renaming and copying a folder is one request per object.** A folder with
  10 000 files takes 10 000 requests.
- Deleting a folder deletes its objects; there is no trash.

## Choosing one

| | Local | SFTP | S3 |
|---|---|---|---|
| Speed | fastest | network-bound | network-bound |
| Server-side copy | yes | no | no |
| Server-side rename | yes | yes | copy + delete |
| Permission bits | yes | yes | no |
| Max file size | disk | disk | 5 GiB |
| Needs a writable volume | yes | no | no |

For a single server, local storage is simplest and fastest. SFTP suits
"files already on another machine". S3 suits containers with no persistent disk,
at the cost of size limits and slow bulk operations.

## Security notes

- **Credentials live in `config.json` in plain text.** Restrict it:
  `chmod 600 private/config.json`, and do not commit it (`private/` is
  git-ignored).
- API responses never return the SFTP password or S3 secret. An empty value in
  the admin form keeps the stored one.
- Nothing is written into a user's folders by FileGator itself — no index files,
  no metadata. Any existing folder can be pointed at and managed.
