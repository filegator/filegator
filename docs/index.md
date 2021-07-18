## FileGator
<a href="https://demo.filegator.io"><img src="https://img.shields.io/badge/Live-Demo-brightgreen.svg?style=flat-square" alt="Live demo"></a>
<a href="https://github.com/filegator/filegator/actions"><img src="https://github.com/filegator/filegator/workflows/PHP/badge.svg" alt="Build Status PHP"></a>
  <a href="https://github.com/filegator/filegator/actions"><img src="https://github.com/filegator/filegator/workflows/Node/badge.svg" alt="Build Status Node"></a>
<a href="https://codecov.io/gh/filegator/filegator"><img src="https://codecov.io/gh/filegator/filegator/branch/master/graph/badge.svg" alt="Code Coverage"></a>
<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License"></a>

<br>

[FileGator](https://filegator.io) is a free, [open-source](https://github.com/filegator/filegator), self-hosted web application for managing files and folders.

You can manage files inside your local repository folder (on your server's hard drive) or connect to other storage adapters (see below).

FileGator has multi-user support so you can have admins and other users managing the files with different access permissions, roles and home folders.

All basic file operations are supported: copy, move, rename, create, delete, zip, unzip, download, upload.

If allowed, users can download multiple files or folders at once.

File upload supports drag&drop, progress bar, pause and resume. Upload is chunked so you should be able to upload large files regardless of your server's configuration.



## Features & Goals
- Multiple storage adapters (Local, FTP, Amazon S3, Dropbox, DO Spaces, Azure Blob and many others via [Flysystem](https://github.com/thephpleague/flysystem))
- Multiple auth adapters with roles and permissions (Store users in json file, database or use WordPress)
- Multiple session adapters (Native File, Pdo, Redis, MongoDB, Memcached and others via [Symfony](https://github.com/symfony/symfony/tree/4.4/src/Symfony/Component/HttpFoundation/Session/Storage/Handler))
- Single page front-end (built with [Vuejs](https://github.com/vuejs/vue), [Bulma](https://github.com/jgthms/bulma) and [Buefy](https://github.com/buefy/buefy))
- Chunked uploads (built with [Resumable.js](https://github.com/23/resumable.js))
- Zip and bulk download support
- Highly extensible, decoupled and tested code
- No database required
- Framework free [™](https://www.youtube.com/watch?v=L5jI9I03q8E)

## Limitations
- Symlinks are not supported by the underlying [Flysystem](https://flysystem.thephpleague.com/v1/docs/adapter/local/)
- File permission operations are not supported (chmod/chown)


## Why Open Source on GitHub?

There are several reasons why we switched to open source model and GitHub.

Basically, we wanted to increase:

- Code quality by bringing more developers on board
- Code auditability and visibility
- Security
- Project lifetime

At the end, the more people who can see and test a set of code, the more likely any flaws will be caught and fixed quickly.



## Show your support

- Please star this repository on [GitHub](https://github.com/filegator/filegator/stargazers) if this project helped you!
- Become a backer or sponsor on [Patreon](https://www.patreon.com/alcalbg).

## License

Copyright (c) 2019 [Milos Stojanovic](https://github.com/alcalbg).

This project is MIT licensed.
