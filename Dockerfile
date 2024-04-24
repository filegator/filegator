#################################
# stage builder: build and test
#################################
FROM php:8.3-apache-bullseye AS builder

RUN curl -sL https://deb.nodesource.com/setup_14.x | bash -

RUN apt-get update > /dev/null
RUN apt-get install -y git libzip-dev nodejs python2 libgtk2.0-0 libgtk-3-0 libgbm-dev libnotify-dev libgconf-2-4 libnss3 libxss1 libasound2 libxtst6 xauth xvfb

RUN curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer

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
