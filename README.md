<p align="center">
<img src="https://raw.githubusercontent.com/filegator/filegator/master/dist/img/logo.gif">
</p>

<p align="center">
<a href="https://travis-ci.org/filegator/filegator"><img src="https://travis-ci.org/filegator/filegator.svg?branch=master" alt="Build Status"></a>
<a href="https://codecov.io/gh/filegator/filegator"><img src="https://codecov.io/gh/filegator/filegator/branch/master/graph/badge.svg" alt="Code Coverage"></a>
<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License"></a>
  </p>


## FileGator - Powerful Multi-User File Manager

FileGator is a free, open-source PHP script for managing files and folders.

You can manage files inside your local repository folder (on your server's hard drive) or connect to other storage adaptes (see below).

FileGator has multi-user support so you can have admins and other users managing files with different access permissions, roles and home folders.

All basic file operations are supported: copy, move, rename, create, delete, zip, unzip, download, upload.

If allowed, users can download multiple files or folders at once.

File upload supports drag&drop, progress bar, pause and resume. Upload is chunked so you should be able to upload large files regardless of your server configuration.


## Demo
[https://demo.filegator.io](https://demo.filegator.io)

This is read-only demo with guest account enabled.
You can also log in with john/john to see John's private files.


## Documentation
[Check out the documentation](https://docs.filegator.io/)


## Features & Goals
- Multiple storage adapters (Local, FTP, S3, Dropbox and many others via [Flysystem](https://github.com/thephpleague/flysystem))
- Multiple auth adapters with roles and permissions (Store users in json file or database)
- Multiple session adapters (Native File, Pdo, MongoDB, Memcached and others via [Symfony](https://github.com/symfony/symfony/tree/master/src/Symfony/Component/HttpFoundation/Session/Storage/Handler))
- Single page front-end (built with [Vuejs](https://github.com/vuejs/vue), [Bulma](https://github.com/jgthms/bulma) and [Buefy](https://github.com/buefy/buefy))
- Chunked uploads (built with [Resumable.js](https://github.com/23/resumable.js))
- Zip and bulk download support
- Highly extensible, decoupled and tested code
- No database required
- Framework free [™](https://www.youtube.com/watch?v=L5jI9I03q8E)


## Requirements
- PHP 7.1.3+


## Security
If you discover any security related issues, please email alcalbg@gmail.com instead of using the issue tracker.

