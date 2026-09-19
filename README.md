<p align="center">
<img src="https://raw.githubusercontent.com/filegator/filegator/master/dist/img/logo.svg">
</p>

<p align="center">
<a href="https://demo.filegator.io"><img src="https://img.shields.io/badge/Live-Demo-brightgreen.svg?style=flat-square" alt="Live demo"></a>
<a href="https://github.com/filegator/filegator/actions"><img src="https://github.com/filegator/filegator/workflows/PHP/badge.svg?branch=master" alt="Build Status PHP master"></a>
  <a href="https://github.com/filegator/filegator/actions"><img src="https://github.com/filegator/filegator/workflows/Node/badge.svg?branch=master" alt="Build Status Node master"></a>
<a href="https://codecov.io/gh/filegator/filegator"><img src="https://codecov.io/gh/filegator/filegator/branch/master/graph/badge.svg" alt="Code Coverage"></a>
<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License"></a>
  </p>


## FileGator - Powerful Multi-User File Manager

FileGator is a free, open-source, self-hosted web application for managing files and folders.

You can manage files inside your local repository folder (on your server's hard drive) or connect to other storage adapters (see below).

FileGator has multi-user support so you can have admins and other users managing files with different access permissions, roles and home folders.

All basic file operations are supported: copy, move, rename, edit, create, delete, preview, zip, unzip, download, upload.

If allowed, users can download multiple files or folders at once.

File upload supports drag&drop, progress bar, pause and resume. Upload is chunked so you should be able to upload large files regardless of your server configuration.

<p align="center">
<a href="https://demo.filegator.io"><img src="https://filegator.io/img/animated.gif" alt="Screenshot"></a>
</p>


## Sponsors & Backers
FileGator is a free, open-source project. It's an independent project with its ongoing development made possible entirely thanks to the support by these awesome [backers](https://github.com/filegator/filegator/blob/master/BACKERS.md) and sponsors:

<table align="center">
  <tbody>
    <tr>
      <td align="center" valign="middle">
        <a href="https://www.hostinger.com" target="_blank">
          <img title="Hostinger" width="177px" src="https://filegator.io/img/hstngr.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://www.linkpreview.net/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="Preview Web Links with our Free API service. Get JSON Response for any URL" width="200px" src="https://www.linkpreview.net/images/logo-dark.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://correctme.app/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="Free Online Grammar and Spell Checker" width="177px" src="https://correctme.app/logo.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://interactive32.com/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="Modern approach to software development" width="177px" src="https://interactive32.com/images/logo.png">
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
- use as ftp/sftp replacement
- manage s3 or other 3rd party cloud storage
- use to quickly zip and download remote files


## Documentation
Check out the official [documentation](https://docs.filegator.io/) on how to download, [install](https://docs.filegator.io/install.html) and use FileGator.


## Features & Goals
- Multiple storage adapters (Local, FTP, Amazon S3, Dropbox, DO Spaces, Azure Blob and many others via [Flysystem](https://github.com/thephpleague/flysystem))
- Multiple auth adapters with roles and permissions (Store users in json file, database or use WordPress)
- Multiple session adapters (Native File, Pdo, Redis, MongoDB, Memcached and others via [Symfony](https://github.com/symfony/symfony/tree/4.4/src/Symfony/Component/HttpFoundation/Session/Storage/Handler))
- Single page front-end (built with [Vuejs](https://github.com/vuejs/vue), [Bulma](https://github.com/jgthms/bulma) and [Buefy](https://github.com/buefy/buefy))
- Chunked uploads (built with [Resumable.js](https://github.com/23/resumable.js))
- Zip and bulk download support
- Highly extensible, decoupled and tested code
- No database required

## Limitations
- Symlinks are not supported by the underlying [Flysystem](https://flysystem.thephpleague.com/v1/docs/adapter/local/)
- File ownership is not supported (chown)
- Too many files in the same directory can negatively impact performance

## Docker
Check out [the official docker image](https://hub.docker.com/r/filegator/filegator) with instructions on how to use it

Docker quick start:
```
docker run --rm -p 8080:8080 filegator/filegator
visit: http://127.0.0.1:8080 login as admin/admin123
```

## Other installation methods
Sponsored or community-contributed installation methods for various platforms:

- **[One-click deployment](https://www.hostg.xyz/SHK2W)** provided by [Hostinger](https://hostinger.com). No technical setup or manual server configuration is required.                                      
- **[Deploy with Easypanel](https://easypanel.io/templates/filegator)**, a self-hosted Docker deployment platform.

Please note that this section may contain affiliate links. We may earn a commission at no extra cost to you, which helps support FileGator and its development.


## Show your support

Please ⭐️ this repository if this project helped you!

## Security

If you discover a security vulnerability, please report it privately via GitHub Security Advisories instead of using the public issue tracker:

https://github.com/filegator/filegator/security/advisories

## License

Copyright (c) 2019 [Milos Stojanovic](https://github.com/alcalbg).

This project is MIT licensed.
