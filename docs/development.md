---
currentMenu: development
---

## Project setup for development (Linux)

You must have `git`, `php`, `npm`, and `composer` installed.

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

## Compiles and hot-reloads

The following command will launch backend and frontend on ports 8081 and 8080:

```
npm run serve
```
Once everything is ready visit: `http://localhost:8080`

## Run tests & static analysis

Testing requires xdebug and sqlite php extensions.

```
vendor/bin/phpunit
vendor/bin/phpstan analyse ./backend
npm run lint
npm run e2e
```

## Deployment

Set the website document root to `/dist` directory. This is also known as 'public' folder.

NOTE: For security reasons `/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser.

