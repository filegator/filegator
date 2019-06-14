<p align="center">
<img src="https://raw.githubusercontent.com/filegator/static/master/logo.gif">
</p>

<p align="center">
<a href="https://travis-ci.org/filegator/filegator"><img src="https://travis-ci.org/filegator/filegator.svg?branch=master" alt="Build Status"></a>
<a href="https://codecov.io/gh/filegator/filegator"><img src="https://codecov.io/gh/filegator/filegator/branch/master/graph/badge.svg" alt="Code Coverage"></a>
<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-green.svg" alt="License"></a>
  </p>


## FileGator - Powerful Multi-User File Manager
Copy, move, rename, create, edit or delete online files and folders.
Upload with drag&drop, progress bar, pause and resume.
Download multiple files or directories at once.
Zip and unzip files and folders.
Create users with different access permissions and home directories for each user.

## Features & Goals
- Multiple storage adapters (Local, FTP, S3, Dropbox and many others via [Flysystem](https://github.com/thephpleague/flysystem))
- Multiple auth adapters with roles and permissions (Store users in json file or database)
- Multiple session adapters (Native File, Pdo, MongoDB, Memcached and others via [Symfony](https://github.com/symfony/symfony/tree/master/src/Symfony/Component/HttpFoundation/Session/Storage/Handler))
- Single page front-end (built with [Vuejs](https://github.com/vuejs/vue) and [Buefy](https://github.com/buefy/buefy))
- Chunked uploads (via [Resumable.js](https://github.com/23/resumable.js))
- Zip and bulk download support
- Highly extensible, decoupled and tested code
- No database required
- Framework free [™](https://www.youtube.com/watch?v=L5jI9I03q8E)


## Requirements
- PHP 7.1.3+


## Download precompiled build
- Latest: [v7.0.0-RC1](https://github.com/filegator/static/raw/master/builds/filegator_v7.0.0-RC1.zip)
- Unzip files and upload them to your PHP server
- Make sure you webserver can read and write to /storage and /private folders
- Set the website document root to /dist directory
- Visit web page, if something goes wrong check /private/logs/app.log
- Login with default credentials admin/admin123
- Change default admin's password
- Adjust configuration.php


## Project setup for development
```
git clone git@github.com:filegator/filegator.git
cd filegator
cp configuration_sample.php configuration.php
sudo chmod -R 777 private/
sudo chmod -R 777 repository/
composer install
npm install
npm run build
```

### Compiles and hot-reloads (backend and frontend on ports 8081 and 8080)
```
npm run serve
```
Once everything is ready visit: ```http://localhost:8080```


### Run tests
```
vendor/bin/phpunit
```

### Deployment
Set the website document root to /dist directory

## Security
If you discover any security related issues, please email alcalbg@gmail.com instead of using the issue tracker.

