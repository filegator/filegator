---
title: "Guest access"
weight: 40
---

Guest access lets anonymous visitors **browse and download** without logging
in. It is **off by default**: a fresh install never publishes anything until an
administrator turns it on.

## Turning it on

The rule: an anonymous visitor is served as the guest account whenever that
account can **read**. To publish the repository, give the guest the `read`
permission in **Admin → Users**, or in `private/users.json`.

> **A guest can hold only `read`, `download` and `batchdownload`.** Every other
> permission is refused by the server and hidden from the admin form, which
> follows the role: promote the account to `user` and the full list returns.
> Removing `read` switches guest access off: anonymous visitors then get the
> login screen.

The guest account is an ordinary entry in `private/users.json` with the `guest`
role, and the only permissions the server accepts for it are the read ones:

```json
{
  "2": {
    "username": "guest",
    "name": "Guest",
    "role": "guest",
    "homedir": "/",
    "permissions": "read|download",
    "password": ""
  }
}
```

The guest row is listed **last** and marked **Public** while it can read,
because that row publishes the repository.

> Permissions use the pipe format (`read|download`). A comma-separated value is
> not a valid record and the users file will refuse to load.

## What guests can do

**Reads only.** This is enforced in code, not just by the stored permissions, so
editing `users.json` cannot grant more:

| Action | Guest |
|--------|-------|
| Browse and search | allowed with `read` |
| Open the gallery | needs `read` **and** `download` |
| Download a file | allowed with `download` |
| Download several as a zip | allowed with `batchdownload` (plus `read` and `download`) |
| Upload, create, rename, move, copy, delete | **always refused** |
| Zip, unzip, chmod, admin area | **always refused** |

Even if someone edits `users.json` to give the guest `write` or `upload`,
those permissions are ignored.

### Why guests cannot write

There is no login behind a guest session, so a CSRF token proves nothing — an
attacker can simply fetch one, exactly like any other visitor. Anonymous write
access would therefore be **unauthenticated write access** to your server's
disk.

If you want visitors to send you files, that is a different feature with a
different design (a token in the URL, expiry, revocation). Do not reach for
guest write permissions.

## Scope

The guest has a **home folder** like any user, set on its row. It is confined
to that folder: `..`, absolute paths and symlinks cannot escape it. Point it at
a subfolder to publish only part of the repository.

## Signing in

A visitor browsing as a guest sees **Log in** as the label on the account menu
in the header. The menu then offers only the theme and **Log in** — there is no
password to change and no session to end, so it does not pretend to offer them.
Signing in switches to their own account, and **Setup** stays hidden throughout.

A visitor who was logged in and whose session expired is **not** demoted to
guest — they are asked to log in again.

## Deleting the guest

To stop guest access, clear the guest's permissions (or uncheck `read`) in
**Admin → Users**; anonymous visitors then get the login screen.

The guest account itself cannot be deleted: that would silently unpublish the
repository and lose its home folder setting.

## Which files does the guest see?

The guest's **home folder**, which starts as `/` — the whole storage root. Before
enabling guest access, point it at a subfolder so only that folder is published:

1. **Admin → Users → Guest → Edit**
2. change **Home**, then **Save**
3. now click **Enable**

The guest is confined to that folder exactly like any user.
