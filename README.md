<p align="center">
<img src="https://raw.githubusercontent.com/filegator/static/master/logo.gif">
</p>

[![Build Status](https://travis-ci.org/filegator/filegator.svg?branch=master)](https://travis-ci.org/filegator/filegator)

## FileGator - Powerful Multi-User File Manager
Copy, move, rename, create, edit or delete online files and folders.
Upload with drag&drop, progress bar, pause and resume.
Download multiple files or directories at once.
Zip and unzip files and folders.
Create users with different access permissions and home directories for each user.

## Features & Goals
- Multiple storage adapters (using [Flysystem](https://github.com/thephpleague/flysystem))
- Multiple auth adapters with roles and permissions
- Multiple session adapters (file, database)
- Single page front-end (built with [Vuejs](https://github.com/vuejs/vue))
- Chunked uploads (via [Resumable.js](https://github.com/23/resumable.js))
- Zip and bulk download support
- Highly extensible, decoupled and tested code
- Framework free


## Requirements
- PHP 7.1.3


## Download precompiled build
[v7.0.0-RC1](https://github.com/filegator/static/raw/master/builds/filegator_v7.0.0-RC1.zip)

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
Visit http://localhost:8080


### Run tests
```
vendor/bin/phpunit
```

### Deployment
Set the website document root to /dist directory

## Security
If you discover any security related issues, please email alcalbg@gmail.com instead of using the issue tracker.

