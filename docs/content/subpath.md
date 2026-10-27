---
title: "Running under a subpath"
weight: 50
---

FileGator can be mounted at a URL prefix such as
`https://myapp.com/browser/` instead of the site root. This is useful when one
domain hosts several applications.

## Setting it

The default is `/` (the site root), so nothing changes unless you ask.

With the `-base-path` flag:

```sh
./bin/filegator -addr 127.0.0.1:8080 -base-path /browser
```

With `make`:

```sh
make run BASE_PATH=/browser
make watch BASE_PATH=/browser
```

`browser`, `/browser` and `/browser/` all mean the same thing. An invalid value
(such as `/a//b`) stops startup with a clear message.

**This is a start-up setting, not a stored one.** It is never written to
`config.json`: where an instance lives is decided by the proxy or the container
that runs it, and the same folder of files can be mounted at different prefixes
by different deployments. Restart without the flag and the app is back at the
site root. The active prefix is printed at startup:

```
2026/10/25 12:29:34 base path "/browser"
2026/10/25 12:29:34 listening on http://127.0.0.1:18770/browser/
```

## Behind a reverse proxy

The proxy must **not** strip the prefix: FileGator expects the full path.
Pass it through unchanged:

```nginx
location /browser/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

    # Large uploads need room; match upload_max_size.
    client_max_body_size 100M;

    # Don't buffer streamed downloads and batch zips.
    proxy_buffering off;
}
```

Caddy:

```
handle_path /browser/* {
    reverse_proxy 127.0.0.1:8080
}
```

Note `handle_path` **does** strip the prefix — use `handle` with a
`rewrite` if you need the opposite behaviour. FileGator wants the prefix kept:

```
handle /browser/* {
    reverse_proxy 127.0.0.1:8080
}
```

## What the prefix affects

- **Every route**, including `/api/...`. Requests outside the prefix get `404`,
  so the app does not answer on `/` by accident.
- **Assets and page links**, which are relative and resolved against a
  `<base>` element the server fills in. Nothing in the frontend hardcodes `/`.
- **Cookies**: the session and CSRF cookies are scoped to the prefix
  (`Path=/browser`), so they are not sent to other applications on the same
  domain and cannot be read by them.

## Multiple instances

Each instance needs its own prefix, configuration file, `private/` folder and
storage root:

```sh
./bin/filegator -addr 127.0.0.1:8081 -config /srv/files/config.json -base-path /files
```

Each instance passes its own prefix; nothing about it is shared through the
configuration file.

Two instances on **the same origin** are fine, because their cookies are scoped
to different paths. They must not share `private/`, since the session key is
stored there.

## Troubleshooting

**Blank page or 404s in the console after changing the prefix** — the browser
cached the old page. Reload with the cache disabled; the page is served with
`Cache-Control: no-cache` for exactly this reason.

**Login succeeds but the next request is unauthenticated** — the session cookie
has `Secure` enabled over plain HTTP, or the proxy rewrites paths so the
`Path=/browser` cookie is not returned. Check **Admin → Security → Secure
cookies** and that the proxy keeps the prefix.

**API calls go to `/api/...` instead of `/browser/api/...`** — an old cached
script. The assets are versioned by the page load, not by a build hash, so clear
the cache once after changing `base_path`.
