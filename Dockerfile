#################################
# stage builder: build and test
#################################
# Pin Node and Composer source images by digest so the bytes we COPY in
# can never silently change. The prior curl-and-pipe installs had no
# integrity check at all.
FROM node:22-bullseye@sha256:62f550497561d6285e10abd952730db89c905be990237eaf8744137929c72844 AS node-source
FROM composer:2@sha256:b09bccd91a78fe8a9ab4b33d707b862e8fe54fec17782e32683ad2a69c46867d AS composer-source

FROM php:8.3-apache-bullseye AS builder

COPY --from=node-source /usr/local/bin/node /usr/local/bin/node
COPY --from=node-source /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
 && ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

RUN apt-get update > /dev/null
RUN apt-get install -y git libzip-dev python2 libgtk2.0-0 libgtk-3-0 libgbm-dev libnotify-dev libgconf-2-4 libnss3 libxss1 libasound2 libxtst6 xauth xvfb

COPY --from=composer-source /usr/bin/composer /usr/local/bin/composer

RUN docker-php-ext-install zip
RUN docker-php-ext-enable zip

RUN git clone https://github.com/filegator/filegator.git /var/www/filegator/
WORKDIR "/var/www/filegator/"
RUN cp configuration_sample.php configuration.php
RUN composer install
RUN composer require league/flysystem-sftp:^1.0 -W
RUN composer require league/flysystem-aws-s3-v3:^1.0 -W
RUN npm install
RUN npm run build
RUN vendor/bin/phpunit
RUN npm run lint
#RUN npm run test:e2e
RUN rm -rf node_modules frontend tests docs .git .github
RUN rm README.md couscous.yml repository/.gitignore babel.config.js cypress* .env* .eslint* .gitignore jest.* .php_cs* phpunit* postcss* vue*

#################################
# stage production
#################################
FROM php:8.3-apache-bullseye

RUN apt-get update > /dev/null
RUN apt-get install -y git libzip-dev libldap2-dev

RUN docker-php-ext-configure ldap --with-libdir=lib/x86_64-linux-gnu/
RUN docker-php-ext-install zip ldap
RUN docker-php-ext-enable zip ldap

COPY --from=builder /var/www/filegator /var/www/filegator
RUN chown -R www-data:www-data /var/www/filegator/
WORKDIR "/var/www/filegator/"
RUN chmod -R g+w private/
RUN chmod -R g+w repository/

ENV APACHE_DOCUMENT_ROOT=/var/www/filegator/dist/
ENV APACHE_PORT=8080
RUN sed -ri -e 's!/var/www/html!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/sites-available/000-default.conf
RUN sed -ri -e 's!/var/www/!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/apache2.conf /etc/apache2/conf-available/docker-php.conf
RUN sed -ri -e 's!80!${APACHE_PORT}!g' /etc/apache2/ports.conf
RUN sed -ri -e 's!80!${APACHE_PORT}!g' /etc/apache2/sites-available/000-default.conf
RUN a2enmod rewrite

EXPOSE ${APACHE_PORT}

VOLUME /var/www/filegator/repository
VOLUME /var/www/filegator/private

USER www-data

CMD ["apache2-foreground"]
