
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

## Compiles and hot-reloads (backend and frontend on ports 8081 and 8080)

```
npm run serve
```
Once everything is ready visit: ```http://localhost:8080```

## Run tests & static analysis

```
vendor/bin/phpunit
vendor/bin/phpstan analyse ./backend
```

## Deployment

Set the website document root to ```/dist``` directory.

