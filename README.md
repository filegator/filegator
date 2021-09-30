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
FileGator is a free, open-source project. It's an independent project with its ongoing development made possible entirely thanks to the support by these awesome [backers](https://github.com/filegator/filegator/blob/master/BACKERS.md). If you'd like to join them, please consider:

- [Become a backer or sponsor on Patreon](https://www.patreon.com/alcalbg).

<table>
  <tbody>
    <tr>
      <td align="center" valign="middle">
        <a href="https://www.linkpreview.net/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="Preview Web Links with our Free API service. Get JSON Response for any URL" width="177px" src="https://www.linkpreview.net/images/logo-dark.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://www.savepage.io/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="Screenshot any website with our powerful API" width="177px" src="https://www.savepage.io/images/logo.svg">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://www.getping.info/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="Trigger an email, sms or slack notification with a simple GET request" width="177px" src="https://www.getping.info/img/logo.svg">
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
    </tr><tr></tr>
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

## Demo
[https://demo.filegator.io](https://demo.filegator.io)

This is read-only demo with guest account enabled.
- you can log in as `john/john` to see John's private files
- or `jane/jane` as readonly + download user.


## Documentation
[Check out the documentation](https://docs.filegator.io/)


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

## Minimum Requirements
- PHP 7.2.5+ (with php-zip extension)

See [install instructions](https://docs.filegator.io/install.html) for Ubuntu 18.04 or Debian 10.3. Get $100 in ([server credits here](https://m.do.co/c/93994ebda78d)) so you can play around.


## Download precompiled build
Precompiled build is created for non-developers. In this version, the frontend (html, css and javascript) is compiled for you and the source code is removed so the final archive contains only minimum files.

[Download & install instructions](https://docs.filegator.io/install.html)


## Project setup for development (Linux)

You must have `git`, `php`, `npm`, and `composer` installed.

```
git clone https://github.com/filegator/filegator.git
cd filegator
cp configuration_sample.php configuration.php
chmod -R 775 private/
chmod -R 775 repository/
composer install --ignore-platform-reqs
npm install
npm run build
```


## Compiles and hot-reloads

The following command will launch backend and frontend on ports 8081 and 8080:

```
npm run serve
```
Once everything is ready visit: `http://localhost:8080`


## Run tests & static analysis

Testing requires xdebug, php-zip and sqlite php extensions.

```
vendor/bin/phpunit
vendor/bin/phpstan analyse ./backend
npm run lint
npm run e2e
```


## Deployment

Set the website document root to `/dist` directory. This is also known as 'public' folder.

NOTE: For security reasons `/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser.

## Show your support

Please ⭐️ this repository if this project helped you!

## Security

If you discover any security related issues, please email alcalbg@gmail.com instead of using the issue tracker.

## License

Copyright (c) 2019 [Milos Stojanovic](https://github.com/alcalbg).

This project is MIT licensed.
