---
title: "First five minutes"
weight: 12
---

You have built or unpacked FileGator and started it. This page is the shortest
path from a running binary to something you would let other people use. Every
step is in the browser; no configuration file needs editing.

## 1. Sign in

Open <http://127.0.0.1:8080> and sign in as `admin` with the password
`admin123`.

> If you started the server with `FILEGATOR_ADMIN_PASSWORD` set, use that
> password instead and skip step 2 — see
> [Installation]({{< ref "install.md" >}}). It only applies until the password
> is changed, so the variable can stay in your compose file.

A banner across the top of the screen says the administrator still uses the
default password. That is expected; the next step removes it.

## 2. Change the password

Click **Change password** in the banner (or the account button in the header).
Enter `admin123` as the current password and choose a new one.

The current password is required on purpose: someone who finds an open session
on an unlocked machine cannot take the account over.

The banner disappears once the password is different from the default, and the
startup warning stops appearing in the log.

## 3. Point it at a folder

Out of the box the files live in `repository/` next to the binary. To manage an
existing folder instead:

1. Click **Setup** in the header.
2. Open **Repository**.
3. Pick the folder in the picker — it browses the whole server, not just
   `repository/`.

The change applies immediately, with no restart, and is saved to the
configuration file. If the folder cannot be read, the old one stays active and
the reason is shown.

The folder is managed **in place**: FileGator writes no index, no metadata and
no hidden files into it, so your other tools keep seeing ordinary files.

## 4. Create a user

1. **Setup → Users → New user.**
2. Choose a username and a role. For a normal account, leave the role on
   **user**.
3. Set the **home folder** — start typing, or pick a folder in the tree. The
   user sees **only** this folder and nothing above it.
4. Tick the permissions. For everyday use: **read**, **write**, **upload**,
   **download**, **batchdownload**, **zip**.
5. Enter a password and save.

The home folder is created if it does not exist, so you do not have to make the
folder first.

> A **user** is confined to their home folder. An **administrator** always sees
> the whole storage root, whatever home folder is set — that is deliberate, so a
> mistyped path cannot lock the only account that could repair it.

## 5. Decide about anonymous access

Two things are off by default, and both mean "anyone on the network can read
your files":

- **Guest access.** Anonymous visitors browse and download when the `guest`
  account has `read`. Turn it on from the guest row in **Users** (the
  **Enable** button) after pointing its home folder at just the folder you want
  to publish — see [Guest access]({{< ref "guest-access.md" >}}).
- **IP allow/deny lists.** If the server is reachable from anywhere, consider
  adding your network to **IP allowlist** on **Setup → Security**: once that
  list is non-empty, only the addresses in it may connect.

Leave both alone if only you will use the instance.

## 6. Put it behind HTTPS

FileGator does not serve TLS itself. Run it on loopback and let a reverse proxy
terminate HTTPS, then:

1. **Setup → Security** → tick **Secure cookies (HTTPS only)** and save.
2. Set **Trusted IP header** to the header your proxy sets
   (`X-Real-IP`, `X-Forwarded-For`, `CF-Connecting-IP`), so the login lockout
   and the IP lists see real addresses rather than the proxy's.
3. **Restart** the process: the cookie flags are read at startup, unlike most
   other settings.

The nginx and Caddy snippets are in [Installation]({{< ref "install.md" >}})
and [Running under a subpath]({{< ref "subpath.md" >}}).

## Keep a way back in

Two things prevent the commonest lockout — a forgotten administrator password
and no other way in:

1. **Create a second administrator** in **Setup → Users**. Either one can reset
   the other's password.
2. **Keep a copy of `private/users.json`.** It holds every account, and it is
   small.

If you do get locked out with shell access to the server:

```sh
./bin/filegator -reset-admin-password admin
```

See [Forgotten admin password]({{< ref "install.md" >}}) for the details.

## Where to go next

- [Users, roles and permissions]({{< ref "users.md" >}}) — what each permission
  actually allows
- [Storage backends]({{< ref "storage.md" >}}) — SFTP or S3 instead of a local
  folder
- [Event log]({{< ref "logging.md" >}}) — who signed in, and who deleted that
  file
- [Installation]({{< ref "install.md" >}}) — `private/` holds the accounts and
  the configuration, and is the part worth backing up
- [Upgrading and what's new]({{< ref "upgrading.md" >}}) — what changed in this
  version, and what to do if you upgraded from an older release
