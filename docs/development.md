---
currentMenu: development
---

## Project setup for development (Docker)

```
git clone https://github.com/filegator/filegator.git
cd filegator
docker compose -f docker-compose-dev.yml up
```
Once everything is ready visit: [http://localhost:8080](http://localhost:8080) and login as admin/admin123, Ctrl+c to stop.

See `docker-compose-dev.yml` for more informations about configurations and dependencies.

## Project setup for development (Linux)

You must have `git`, `php`, `node`, `npm`, and `composer` installed.

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
npm run test:e2e
```

## Deployment

Set the website document root to `filegator/dist` directory. This is also known as 'public' folder.

NOTE: For security reasons `filegator/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser. If you run the script from the root folder, you will see the message **'Development mode'** as a security warning.

