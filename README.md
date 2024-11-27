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

<table align="center">
  <tbody>
    <tr>
      <td align="center" valign="middle">
        <a href="https://www.linkpreview.net/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="Preview Web Links with our Free API service. Get JSON Response for any URL" width="200px" src="https://www.linkpreview.net/images/logo-dark.png">
        </a>
      </td>
      <td align="center" valign="middle">
        <a href="https://www.vanillavoice.com/?utm_campaign=Sponsored%20GitHub%20FileGator" target="_blank">
          <img title="VanillaVoice - Turn any Text into Human-Sounding Speech" width="200px" src="https://www.vanillavoice.com/logo.svg">
        </a>
      </td>
    </tr>
  </tbody>
</table>

## Demo
[https://demo.filegator.io](https://demo.filegator.io)

This is read-only demo with guest account enabled
- you can log in as `john/john` to see John's private files
- or `jane/jane` as readonly + download user.


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

## Limitations
- Symlinks are not supported by the underlying [Flysystem](https://flysystem.thephpleague.com/v1/docs/adapter/local/)
- File ownership is not supported (chown)

## Docker
Check out [the official docker image](https://hub.docker.com/r/filegator/filegator) with instructions on how to use it

Docker quick start:
```
docker run -p 8080:8080 -d filegator/filegator
visit: http://127.0.0.1:8080 login as admin/admin123
```

## Download & Installation
See [install instructions](https://docs.filegator.io/install.html). Get $100 in ([server credits here](https://m.do.co/c/93994ebda78d)) so you can play around.


## Project setup for development (Docker)

```
git clone https://github.com/filegator/filegator.git
cd filegator
docker compose -f docker-compose-dev.yml up
```
Once everything is ready visit: [http://localhost:8080](http://localhost:8080) and login as admin/admin123, Ctrl+c to stop.

See `docker-compose-dev.yml` for more informations about configurations and dependencies.

## Project setup for development (Linux)

You must have `git`, `php`, `node (v14)`, `npm`, and `composer` installed.

```
git clone https://github.com/filegator/filegator.git
cd filegator
cp configuration_sample.php configuration.php
chmod -R 775 private/
chmod -R 775 repository/
composer install --ignore-platform-reqs
npm install
npm run build
npm run serve
```
Once everything is ready visit: [http://localhost:8080](http://localhost:8080) and login as admin/admin123


## Run tests & static analysis

Testing requires xdebug, php-zip and sqlite php extensions.

```
vendor/bin/phpunit
vendor/bin/phpstan analyse ./backend
npm run lint
npm run test:e2e
```


## Deployment

Set the website document root to `filegator/dist` directory. This is also known as 'public' folder.

NOTE: For security reasons `filegator/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser. If you run the script from the root folder, you will see the message **'Development mode'** as a security warning.

## Show your support

Please ⭐️ this repository if this project helped you!

## Security

If you discover any security related issues, please email alcalbg@gmail.com instead of using the issue tracker.

## License

Copyright (c) 2019 [Milos Stojanovic](https://github.com/alcalbg).

This project is MIT licensed.
