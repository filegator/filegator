FROM php:7-apache-buster

ENV APACHE_DOCUMENT_ROOT=/var/www/filegator/

RUN apt-get update > /dev/null && \
    # Install and enable php zip extension
    apt-get install -y wget unzip libzip-dev && \
    docker-php-ext-install zip && \
    docker-php-ext-enable zip && \
    # Download and extract latest build
    cd /var/www/ && \
    wget https://github.com/filegator/static/raw/master/builds/filegator_latest.zip && \
    unzip filegator_latest.zip && rm filegator_latest.zip && \
    chown -R www-data:www-data filegator/ && \
    chmod -R 775 filegator/ && \
    # configure Apache to use the value of APACHE_DOCUMENT_ROOT as its default Document Root
    sed -ri -e 's!/var/www/html!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/sites-available/*.conf && \
    sed -ri -e 's!/var/www/!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/apache2.conf /etc/apache2/conf-available/*.conf && \
    # configure php
    mv "$PHP_INI_DIR/php.ini-production" "$PHP_INI_DIR/php.ini" && \
    # cleanup apt
    apt-get purge -y wget unzip && \
    apt-get autoclean -y && \
    rm -Rf /var/lib/apt/lists/*

EXPOSE 80

VOLUME /var/www/filegator/repository
VOLUME /var/www/filegator/private

WORKDIR "/var/www/filegator/"
