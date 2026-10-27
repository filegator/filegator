---
title: "Customising the interface"
weight: 55
---

The interface — HTML, CSS, JavaScript and the language files — is compiled into
the binary. You can serve it from a folder instead, which means changing a
template or a stylesheet is a file edit rather than a rebuild.

## Serving from a folder

```sh
./bin/filegator -frontend-dir /srv/filegator/ui
```

or in **Admin → UI → Interface folder**, or in `private/config.json`:

```json
{ "frontend_dir": "/srv/filegator/ui" }
```

The flag applies to that run only and is never written to the configuration; the
interface field and the config key make it permanent. The interface field warns
that **a restart is required**, because the interface is loaded once at startup,
and a folder that does not exist is rejected when saving so a typo surfaces
immediately rather than at the next restart.

The form shows where the interface is currently coming from, so it is obvious
whether a saved change is already in effect.

Start by exporting the interface **from the binary you are running**:

```sh
./bin/filegator -export-frontend /srv/filegator/ui
```

```
wrote 76 files to /srv/filegator/ui; start with -frontend-dir /srv/filegator/ui to use them
```

The count grows as the interface gains files; what matters is that the command
starts, so check that `index.html`, `js/app.js` and `css/app.css` are there.

Use the export rather than copying from a source checkout: the files then match
the binary exactly, which a copy from a different commit does not.

The command writes nothing else — no configuration, no storage, no server — and
refuses to write into a non-empty folder that is not already a frontend, so it
cannot scatter files over your notes.

### Re-exporting after an upgrade

An export that is **unchanged** can be written over: running the command again
against a fresh export refreshes it in place.

An export that has been **customised** is refused, and the files that differ are
named:

```
export: /srv/filegator/ui has been customised (css/app.css, js/site.js);
refusing to overwrite it. Export into an empty folder, or delete the old one first
```

This is the one way the command could lose work, so it does not overwrite
edited, added or deleted files. To upgrade a customised install:

1. Note what you changed, or copy the folder aside.
2. Upgrade the binary.
3. Export into a **new, empty** folder.
4. Re-apply your changes to the new files — a diff against the old folder shows
   what the upgrade changed.
5. Point `frontend_dir` at the new folder and restart.

There is no merge: keeping the changes in a short, written list is the reliable
approach.

The folder must contain at least:

| File | Why |
|------|-----|
| `index.html` | the page |
| `js/app.js` | the module entry point |
| `css/app.css` | the base stylesheet |

If any is missing the server **refuses to start** and names what it could not
find. That is deliberate: silently falling back to the built-in copy would leave
you editing files that are never served.

## Light and dark

The interface has a dark theme. It follows the operating system by default, and
the **Dark mode / Light mode item in the account menu** — the button with your
name at the top right of the file browser — switches to the other one and
remembers the choice in the browser. It is per browser, not per account, so a
shared computer keeps each person's own preference.

The menu is where everything about you as a person lives: the theme, **Change
password**, **Language**, and **Log out**. **Setup** stays a separate button,
next to the menu, and is shown to administrators only.

**Language** opens a small dialog with a dropdown of every language the
installation offers, with the current one selected; choosing another and
pressing **Save** switches the interface and remembers the choice in the
browser. Closing the dialog without saving changes nothing. The languages are
listed by the same names the application uses for them (`english`, `german`,
`portuguese_br`) with the first letter capitalised. When an administrator has
turned language selection off (`language_selection`), the item is not offered at
all.

Every language file carries the same messages as `english.json`; a missing one
would fall back to English.

The item shows the theme the click produces: "Dark mode" while the interface is
light, "Light mode" while it is dark.

- **Following the system** is the default and is not a third stored value: with
  no choice made, the page has no `data-theme` attribute and a
  `prefers-color-scheme` media query decides. The icon follows it too, so it is
  right before any JavaScript runs.
- **A stored choice wins in both directions** — dark on a light machine, and
  light on a dark one.
- **The choice is applied before the first paint**, by a small inline script in
  `index.html`, so there is no flash of light on every page load.
- **The account menu is in the file browser's header**, so the login and admin
  screens do not show it. The theme still applies there; the choice is
  remembered, so setting it once is enough.

To force one theme for everyone, set the attribute on `<html>` in `index.html` in
a `frontend_dir` folder:

```html
<html lang="en" data-theme="dark">
```

Note that the theme button then switches between that theme and the other one
for the person clicking it, because the stored choice takes precedence. To
remove the choice entirely, delete the button and the inline script.

The colour variables are in `css/app.css`: `--surface`, `--bg`, `--text`,
`--border`, `--primary`, `--tint` and the rest. Override them in `css/site.css`
rather than editing a component: the dark theme works by changing those
variables, and a component stylesheet that hardcodes `#fff` does not follow it.

## What can be changed, and where

The interface is made of files, and anything a stylesheet, a translation or the
page itself decides can be changed from a `frontend_dir` folder. The page is
readable source: edit a file and reload, with no bundler and no build step. Only
the copy compiled into the binary needs a rebuild, and that is the project's
job, not yours.

The **Setup** area is English only by design: it is written for the
administrator, and its labels are plain English in `js/views/admin.js` rather
than translation entries.

| Change | How |
|---|---|
| Colours and spacing | CSS custom properties in a stylesheet that loads after the built-in ones — see *Add a stylesheet* under [Examples](#examples). No recompilation |
| An analytics tag, a `<meta>`, an extra script | A tag in `index.html` — see [Adding to `<head>` or `<body>`](#adding-to-head-or-body) |
| A label or a translation | A `.json` file in `translations/`, discovered automatically; there is no registration list to keep in sync. The **Setup** (administration) area is English only and has no translation entries |
| The page shell itself | `index.html` is the whole page. The server substitutes one placeholder, `{{BASE_PATH}}` (in the `<base href>` element), and sends it as-is |
| Logo, app name, default language | Screen settings under **Admin → UI**. Do not patch the files for those: a file edit is lost when you re-export, a setting is not |

### Uploading over an existing file

There is **no "overwrite on upload" setting**, on purpose. Every upload whose
name is already taken stops and asks what to do. With the default policy the
refusal arrives on the **first chunk**, before anything is stored, so a doomed
upload costs one request rather than the whole file:

| Choice | Result |
|---|---|
| **Keep both** | The new file is stored under the next free name: `report.pdf` becomes `report (1).pdf`, and `archive.tar.gz` becomes `archive (1).tar.gz`. The whole compound extension is kept. |
| **Replace** | The existing file is overwritten. Destructive, so it is never the default and never offered for folders. |
| **Skip** | This file is not uploaded; the rest of the batch carries on. |
| **Cancel** | The upload is abandoned. |

For a batch, the dialog also offers **apply to the remaining files**, so fifty
clashing files do not mean fifty prompts. Folders never offer **Replace**:
deleting a whole tree to create a directory of the same name is never what
anyone meant.

With **Keep both** or **Replace** the final name is settled when the last chunk
arrives, not on the first: the conflicting file can appear or disappear while a
long upload is still running, so the decision belongs to the moment the file is
actually put in place.

The default, when nobody is asked — an API client that sends no policy at all —
is to **refuse** with `409 Conflict`. Nothing is overwritten without being
asked, and an automated uploader must state its intent:

```json
{ "parent": "/", "name": "report.pdf", "on_collision": "upcount" }
```

`on_collision` accepts `refuse` (default), `upcount` or `replace`, and applies
to create, mkdir, rename, move and copy as well as to uploads; see the
[HTTP API]({{< ref "api.md" >}}). If an installation needs "always replace,
never ask", that is an explicit `on_collision: "replace"` in the client, not a
setting on the server.

### The logo

**Admin → UI** has two logo fields — **Logo URL** and **Logo URL (dark
mode)** — and they are the whole logo configuration. There is one pair, and it
is used in the header of the file browser, in the header of the administration
area and on the login page; the login screen has no separate setting. The
browser shows the dark variant while the interface is in dark mode.

An empty field means the FileGator logo that is **built into the binary**
(`/img/filegator-logo.svg` and `/img/filegator-logo-dark.svg`), which is the
default on a fresh install. No configuration is needed to get it, and nothing
is fetched from another server to show it.

To use your own artwork, either give an `http(s)` URL or put the file in a
custom interface folder and point at it with a path:

1. Set **Interface folder** on the same page and restart (the files are read at
   startup).
2. Put the picture in `<folder>/img/mine.svg`.
3. Set **Logo URL** to `/img/mine.svg` and **Logo URL (dark mode)** to a
   lightened copy — a dark logo on a dark page is the reason the second field
   exists. Leaving it empty reuses your light logo in both themes.

The preview under the fields shows each variant on the background it will
appear on, so a dark logo is judged against a dark card rather than against the
white settings page.

SVG is what the built-in logos are, and what you should use: it stays sharp at
any size and on any screen. A raster format works too — the fields are plain
`<img src>` values.

The **browser tab icon is the logo too**, as on the documentation site: the
light logo becomes the favicon, so a customised install is branded in the tab
as well as on the page. There is no separate favicon setting and no raster
favicon to keep in step; with no logo configured, the tab shows the built-in
FileGator drawing.

Upgrading a customised interface:

1. Keep your `private/users.json` and `repository/` — both are unchanged.
2. The colour, snippet and translation changes are **not** migrated. Re-apply
   them as described in the next section.
3. Anything else in an old frontend tree has no equivalent; export the current
   interface from your binary instead of patching a compiled bundle.

## Adding to `<head>` or `<body>`

There is **no snippet setting**. An arbitrary-markup value in the configuration
would be an unauthenticated path to running script in your origin.

The equivalent is a `frontend_dir` folder with the tag added to `index.html`:

```html
<link rel="stylesheet" href="css/app.css" />
<!-- your own tags, e.g. an analytics snippet -->
<link rel="stylesheet" href="css/site.css" />
...
<script src="js/site.js"></script>
```

Keep the paths **relative**, exactly like the built-in stylesheets, so the
snippet keeps working under a prefix such as `/browser`. Everyone who can
write to that folder can run script in your origin, so treat its permissions
like the config file's.

## What it replaces

The whole interface, not part of it — including `translations/`. A copy that
carries a new `index.html` but the old JavaScript would produce a page whose
parts do not fit together, so the folder is all or nothing.

| Served from the folder | Always from the binary |
|---|---|
| `index.html`, `css/`, `js/`, `translations/` | the API (`/api/…`) |

## Editing safely

- **Reload with the cache disabled.** The page is sent with
  `Cache-Control: no-cache`, but a proxy may still cache it.
- **Keep a copy of the original** so you can diff after an upgrade.
- **Export a fresh copy after upgrading FileGator.** The embedded interface
  moves with the binary: a customised folder keeps working, but it will not gain
  new features or fixes until you merge them in. See
  [Re-exporting after an upgrade](#re-exporting-after-an-upgrade).
- The folder does not need to be writable; a read-only mount is fine.

## Examples

Change the page title in `index.html`:

```html
<title>Acme Files</title>
```

The title is normally set from **Admin → UI** (app name), so prefer that
unless you want the tab to differ from the app name.

Note the paths are **relative**: the page carries a `<base>` element the server
fills in with the prefix, so it works whether the app is at `/` or under a
prefix like `/browser`. Keep them relative.

Add a stylesheet that loads after the built-in ones:

```html
<link rel="stylesheet" href="css/app.css" />
<link rel="stylesheet" href="css/site.css" />
```

Change a colour across the interface in `css/site.css`:

```css
:root {
  --primary: #2563eb;
}
```

Rewrite a label in `translations/english.json`:

```json
{ "New": "New", "Folder": "Folder", "Upload": "Send" }
```

## When not to use it

If you only need a different **logo**, **app name**, **language**, **date
format** or **maximum width**, use **Admin → UI** — no files, no restart, and
nothing to merge after an upgrade. Use `frontend_dir` when you need to change
the markup or behaviour itself.
