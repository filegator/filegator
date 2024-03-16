---
currentMenu: development
---

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

## Project setup for development (Docker compose version)

You must have `git`, `docker` and `docker-compose-plugin` installed.

```
git clone https://github.com/filegator/filegator.git
cd filegator
cp configuration_sample.php configuration.php
chmod -R 775 private/
chmod -R 775 repository/
npm run dc:up
```

If you need a shell inside (for composer for example) always make sure you run under the `app` user. `npm run dc -- exec filegator su app`

Ftp and sftp images for testing are also available in `docker-compose-dev.yml` file.

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

Set the website document root to `filegator/dist` directory. This is also known as 'public' folder.

NOTE: For security reasons `filegator/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser. If you run the script from the root folder, you will see the message **'Development mode'** as a security warning.

