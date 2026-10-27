---
title: "Users, roles and permissions"
weight: 27
---

Every account has a **home folder** and a set of **permissions**. Users see only
their home folder; administrators see the whole storage root.

## Roles

| Role | Meaning |
|------|---------|
| `admin` | Full access to the admin area. **Always sees the whole storage root**, whatever its home folder says, so a mistyped home cannot lock out the last administrator |
| `user` | Confined to its home folder |
| `guest` | The anonymous account. Read-only, and only active when it can `read` — see [Guest access]({{< ref "guest-access.md" >}}) |

## Permissions

| Permission | Grants |
|------------|--------|
| `read` | List folders, stat an entry, search, read a file for the editor |
| `write` | Create, rename, move, copy, delete, save in the editor, and **zip/unzip** |
| `upload` | Upload files |
| `download` | Download files; also what the image gallery's **View** needs |
| `batchdownload` | Download several files as one zip (with `read` **and** `download`) |
| `zip` | Required by **zip and unzip**, together with `read` and `write` |
| `chmod` | Change permission bits (needs `read` and `write` too) |

Permissions are independent: `read` does not imply `download`, and `upload`
does not imply `write`. Grant what the person needs.

A user with `read` but not `download` sees file names but cannot open them; the
interface shows those names as plain text rather than as dead links. The same
applies to a guest.

## Home folders

`homedir` is a path inside the storage root — one key of a user's entry in
`private/users.json`:

```json
{
  "1": {
    "username": "alice",
    "name": "Alice",
    "role": "user",
    "homedir": "/team/alice",
    "permissions": "read|write|upload|download",
    "password": "$2y$10$…"
  }
}
```

- `/` means the storage root itself. Fine for administrators; for a normal user
  it publishes everything.
- Folders may be **pre-created**, or created on the fly with "create home
  folder if it does not exist".
- **Several users may share one home folder.** They see the same files, which is
  the simplest way to give a team a shared area.
- A home folder must exist inside the root. A user pointing at a missing folder
  cannot sign in; startup and the admin form both check, and the admin form
  offers to create it.

Confinement is enforced in the storage layer, not in the HTTP handlers, so no
endpoint can be tricked into reading outside the home — not with `..`, not with
an absolute path, not with a symlink.

## Managing users

**Admin → Users** lists accounts, with the guest row last. From there you can:

- **create** a user, choosing a role, home folder and permissions
- **edit** any of those, and set a new password (blank keeps the current one)
- **set a password** without knowing the old one — the row action, which also
  works while self-service password changes are switched off
- **delete** a user — except the last administrator, and except the guest

### Keeping at least one administrator

There must always be an administrator able to sign in and manage the instance,
so **the last one cannot be removed**. Two actions would do it, and both are
refused with **409**:

- **delete** the last administrator — *"cannot delete the last administrator"*;
- **change its role** to `user` or `guest` — *"cannot change the role of the
  last administrator"*.

The rule is "at least one administrator remains", not "you cannot touch your
own account". An administrator may therefore delete or demote **themselves**,
as long as someone else still holds the role — that is how a handover between
colleagues works, and refusing it would be a nuisance rather than a safeguard.
To make yourself a normal user on an instance where you are the only
administrator, create or promote the new administrator first.

Deleting the guest is refused for a different reason: clearing its permissions
is how guest access is switched off.

If an installation is already stuck with no administrator — a hand-edited
`users.json`, a restored backup — set the role back to `admin` in
`private/users.json` and save. No restart is required: the file is read on
every request. **Sign in again afterwards**, because changing a role ends that
account's existing sessions.

`filegator -list-admins` shows the accounts and which of them are
administrators, which is the fastest way to see whether an administrator
remains. `filegator -reset-admin-password <user>` resets a password but does
**not** change a role, so it cannot promote a demoted account on its own.

With LDAP the create, edit, delete and set-password actions are hidden: the
directory owns its accounts. A SQL-backed installation manages users here, the
same way a `users.json` one does.

## Changing your own password

Every user can change their own password from the header. The current password
must be supplied, so a stolen session cannot take over the account, and the
session cookie is reissued afterwards.

With LDAP this button is hidden: passwords belong to the directory. An
administrator can also switch self-service changes off for everyone on
**Admin → UI** (*users may change their own password*); setting a password from
**Admin → Users** still works, which is how someone who cannot sign in is
helped.

## The users file

Accounts are stored in `private/users.json`:

```json
{
  "1": {
    "username": "alice",
    "name": "Alice",
    "role": "user",
    "homedir": "/team/alice",
    "permissions": "read|write|upload|download",
    "password": "$2y$10$…"
  }
}
```

- **Permissions are pipe-separated** (`read|write`). A comma-separated value is
  not a valid record and the whole file fails to load.
- Passwords are bcrypt hashes. Plain text never authenticates, not even for old
  records.
- The file is created on first start from `users.json.sample` and is
  git-ignored. **Back it up**: it is the only copy of the accounts.

## Changing a user logs them out

Sessions carry a fingerprint of the user's role, permissions, home folder and
password. Changing any of them invalidates existing sessions immediately — which
is what you want when revoking access, and is worth knowing before you tidy up
someone's permissions while they are working.
