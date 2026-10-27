---
title: "Docker"
weight: 15
---

FileGator ships as one image: `filegator/filegator`, a single static binary on a
small Alpine base, running as an unprivileged user.

| Platform | For |
|----------|-----|
| `linux/386` | 32-bit x86: old PCs, some NAS and thin clients |
| `linux/amd64` | 64-bit x86: PCs, servers, most VPS hosts |
| `linux/arm/v6` | First-generation Raspberry Pi and similar ARMv6 boards |
| `linux/arm/v7` | Raspberry Pi 2 and 3, and most 32-bit ARM boards |
| `linux/arm64` | Raspberry Pi 4 and 5 on a 64-bit OS, ARM servers, AWS Graviton, Apple Silicon |

`docker pull` picks the right one from the tag; there is nothing to choose by
hand. A 64-bit ARM host running a 64-bit OS needs `arm64` — it cannot run the
32-bit `arm` image at all, which is why both are published. `arm/v6` and
`arm/v7` are separate builds: an ARMv7 binary does not run on an ARMv6 board.

```sh
docker run -d --name filegator \
  -p 127.0.0.1:8080:8080 \
  -v filegator-data:/data \
  -e FILEGATOR_ADMIN_PASSWORD='a long passphrase of your choosing' \
  filegator/filegator:v8.0.0-beta.1
```

Open <http://127.0.0.1:8080> and sign in as `admin`. `FILEGATOR_ADMIN_PASSWORD`
became the password; without it the account keeps the published default
`admin123` and the notice bar says so until you change it.

The image needs no setup of its own, so a bare run starts a working server on
the first try:

```sh
docker run --rm -p 8080:8080 filegator/filegator:v8.0.0-beta.1
```

It creates `private/` with the accounts, a configuration with a fresh session
key, and the storage folder. Everything it writes lives in the container, so
that form is for a quick look, not for keeping: add `-v` as above for an
installation that survives.

Titled `v8.0.0-beta.1`, and `latest` once a final release is made. Pre-releases
are never tagged `latest`.

## The volume

The installation is **`/data`**, and `private/` and `repository/` live under it:

| Path | Purpose |
|------|---------|
| `/data/private/config.json` | Configuration, with the session key |
| `/data/private/users.json` | Accounts, roles, permissions and password hashes |
| `/data/private/tmp/` | Upload staging |
| `/data/repository/` | Storage root: the files |

One volume at `/data` is the whole installation:

```sh
-v filegator-data:/data
```

The version 7 layout works as two mounts, at the same host folders:

```sh
-v ./private:/data/private -v ./repository:/data/repository
```

**Without a volume, everything is lost with the container.** The image declares
no volumes on purpose: a container started with no `-v` writes into its own
layer, which is plainly temporary. The alternative — declaring them — makes
`docker run` create anonymous volumes whose names nobody can find and which
`--rm` destroys along with the container: data that looks kept and is not. Name
the volume, or it is gone.

## Ownership

The image runs as **UID/GID 65532**, an unprivileged user, and cannot take
ownership of a mount from inside. A named volume works as it is. A
bind mount created by root is not writable by that user, and the server refuses
to start rather than run without being able to save a password change:

```sh
mkdir -p ./filegator-data
sudo chown -R 65532:65532 ./filegator-data
```

## Behind a proxy

The container listens on `0.0.0.0:8080`, which is why the example publishes it on
`127.0.0.1` only. Put nginx, Caddy or Traefik in front for TLS, then:

- set **Security → `secure_cookies`** to `true` and restart the container — the
  cookie flags are built at startup,
- set **Security → `trusted_ip_header`** to the header your proxy fills in, or the
  allow/deny lists and the login lockout see the proxy's address for every visitor.

To reach it without a proxy, publish `8080:8080` and put TLS elsewhere.

## Flags

The command line is passed straight through, so every flag in
[Installation]({{< ref "install.md" >}}#run-it) works:

```sh
docker run -d -p 127.0.0.1:8080:8080 -v filegator-data:/data \
  filegator/filegator:v8.0.0-beta.1 \
  -init -addr 0.0.0.0:8080 -config /data/private/config.json -base-path /files
```

The image's default command is:

```
-init -addr 0.0.0.0:8080 -config /data/private/config.json
```

- **`-init`** because a container has no terminal to answer the first-start
  question on. It creates `private/`, `config.json` and the storage folder
  without asking, and does nothing once they exist. See
  [The first start asks first]({{< ref "install.md" >}}#the-first-start-asks-first).
- **`-config`** so the configuration is read from the volume.
- **`-addr 0.0.0.0:8080`** because the binary's default is loopback, where a
  published port reaches nothing.

The users file is created from the sample that ships in the image, so a fresh
start gives a complete installation with the `admin` and `guest` accounts. An
existing `users.json` is never overwritten.

## Environment

`FILEGATOR_ADMIN_PASSWORD` sets the password of every administrator still using
the default, once. It is ignored afterwards, so it is safe to leave in place; read
it from a secret file rather than writing it in a compose file.

## Upgrading

```sh
docker pull filegator/filegator:v8.0.0-beta.1
```

The volume is read as it is: same layout, same `users.json` format. A release that
changes `config.json` has a note in [Upgrading and what's new]({{< ref "upgrading.md" >}}).
**Back up `private/users.json` and `repository/` before a major upgrade.**

## Coming from a version 7 image

The **files** carry over untouched: `private/users.json` keeps its format, every
account and password works as it did, and `repository/` is the same ordinary
folder of files. What changes is where the container looks for them, and which
user it runs as.

### Your compose file

A version 7 file like this:

```yaml
version: '2'
services:
  filegator:
    container_name: filegator
    image: filegator/filegator
    restart: always
    ports:
      - "8080:8080"
    volumes:
      - ./files:/var/www/filegator/repository
      # - ./users.json:/var/www/filegator/private/users.json
      # - ./configuration.php:/var/www/filegator/configuration.php
```

becomes this:

```yaml
services:
  filegator:
    container_name: filegator
    image: filegator/filegator:v8.0.0-beta.1
    restart: always
    ports:
      - "8080:8080"
    volumes:
      - ./files:/data/repository
      - ./private:/data/private
      # - ./users.json:/data/private/users.json
      # - ./config.json:/data/private/config.json
```

Four changes, each covered below.

### 1. The paths move to `/data`

The container's working directory is `/data`, and `private/` and `repository/`
are resolved against it. A mount at `/var/www/filegator/...` is created but never
read: the listing is empty, uploads go into the container, and they are lost when
it is recreated.

| Version 7 | Version 8 |
|-----------|-----------|
| `/var/www/filegator/repository` | `/data/repository` |
| `/var/www/filegator/private` | `/data/private` |

The host folders on the left stay where they are — only the right-hand side of
the mount changes.

### 2. Mount `private/`, not only `repository/`

A version 7 file that mounted only the repository keeps its files but starts with
fresh accounts and settings every time the container is recreated, because
`private/` then lives inside the container. Mount it:

```yaml
- ./private:/data/private
```

### 3. Give the folders to UID 65532

This image runs as **UID/GID 65532**. The version 7 image ran as `www-data`
(33), so folders it created are owned by 33 and are **not** writable now. The server refuses to start rather than run without being able to
save a password change:

```
cannot start: auth: users file is read-only: private is not writable: permission denied
FileGator needs to write the users file so password and permission changes are saved.
Fix the permissions or mount the volume read-write.
```

It names the folder that needs the owner.

Hand them to the new user once, on the host:

```sh
sudo chown -R 65532:65532 ./private ./files
```

A named volume needs nothing. If you want to keep using the folder as a normal
host account too, give it to group 65532 instead and keep the group write bit:

```sh
sudo chgrp -R 65532 ./private ./files && sudo chmod -R g+rwX ./private ./files
```

### 4. `configuration.php` is not read

Version 8 has no PHP configuration. It reads `private/config.json`, which it
creates on the first start, and the settings are changed on the **Setup** screens
in the browser: app name, logo, upload limits, storage root, authentication and
security.

The mount is harmless — the file is simply ignored — but the settings in it are
not imported. If you mounted a custom `configuration.php`, note what it changed
and set the same values on the Setup screens.

### Then

1. **Stop the version 7 container.** Both write `private/users.json`, and two
   processes writing it at once lose changes.
2. **Back up `private/users.json` and the repository folder.** The users file is
   the only copy of the accounts.
3. **Change the mounts, the image tag and the ownership** as above.
4. **`docker compose up -d` and sign in** with your existing account.
5. **Set the storage root** on **Setup → Repository** if your files are somewhere
   other than `/data/repository`.
6. **Follow [How to upgrade]({{< ref "upgrading.md" >}}#how-to-upgrade)** for the
   rest: authentication settings, HTTPS, and the guest account.

Until `latest` is published, pin the tag: `filegator/filegator:v8.0.0-beta.1`.

### If the volume looks empty

`docker compose logs` says what the first start created. A missing configuration
file, a users file created from the sample, or a read-only folder all appear there
and in the notice bar on **Setup** — a `private/` that was not mounted is the
usual cause of accounts that disappear on restart.

## Commands without a shell

The image has a shell, so `docker exec -it filegator sh` gives you one. The
binary's own commands work too, and are what you normally want:

```sh
docker exec filegator /usr/local/bin/filegator -config /data/private/config.json -list-admins

docker exec -it filegator /usr/local/bin/filegator -config /data/private/config.json \
  -reset-admin-password admin
```

See [Forgotten admin password]({{< ref "install.md" >}}#forgotten-admin-password).

## Building it yourself

```sh
docker buildx build --load -t filegator/filegator:v8.0.0-beta.1 .
docker run --rm -p 127.0.0.1:8080:8080 -v filegator-data:/data filegator/filegator:v8.0.0-beta.1
```

A release builds every platform. `--push` is required on a multi-platform build —
the result is a manifest list, which cannot be loaded into the local Docker:

```sh
docker buildx create --use            # once, for a multi-platform builder
docker buildx build \
  --platform linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64 \
  -t filegator/filegator:v8.0.0-beta.1 --push .
```

`arm/v7` and `arm64` are unrelated builds: the first is 32-bit ARM, the second
64-bit. Both are published because neither runs on the other's hardware.

Dropping `--push` still compiles every platform, which is a quick check that none
is broken. `--load` works for one platform only.

`alpine` also publishes `ppc64le`, `riscv64` and `s390x`. The binary builds
for them, but none is published: adding a platform is a decision, not a build
tweak.

### No emulation needed

The build cross-compiles and the runtime stage contains no `RUN`, only `COPY`.
That is deliberate: with no `RUN` there, the image builds on a plain amd64
machine for every platform, with no qemu. A `RUN` in the runtime stage executes
as the *target* architecture, and fails with `exec /bin/sh: exec format error`
on a host that has no emulation registered. If you add one, register qemu first:

```sh
docker run --privileged --rm tonistiigi/binfmt --install all
```
