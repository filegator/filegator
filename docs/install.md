---
currentMenu: install
---

## Minimum Requirements
- PHP 7.2.5+ (with php-zip extension)


## Download precompiled build
Precompiled build is created for non-developers. In this version, the frontend (html, css and javascript) is compiled for you and the source code is removed so the final archive contains only minimum files.

- Download: [latest release](https://github.com/filegator/static/raw/master/builds/filegator_latest.zip)
- Unzip files and upload them to your PHP server
- Make sure your webserver can read and write to `filegator/repository/` and `filegator/private/` folders
- Set the website document root to `filegator/dist/` directory. This is also known as 'public' folder
- Visit web page, if something goes wrong check `filegator/private/logs/app.log`
- Login with default credentials `admin/admin123`
- Change default admin's password

NOTE: For security reasons `/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser.

## Install on fresh Ubuntu 18.04 or Debian 10.3
On a new server ([get $100 in server credits](https://m.do.co/c/93994ebda78d)) login as root and enter this into the shell:
```
apt update
apt install -y wget unzip php apache2 libapache2-mod-php php-zip

cd /var/www/
wget https://github.com/filegator/static/raw/master/builds/filegator_latest.zip
unzip filegator_latest.zip && rm filegator_latest.zip

chown -R www-data:www-data filegator/
chmod -R 775 filegator/

echo "
<VirtualHost *:80>
    DocumentRoot /var/www/filegator/dist
</VirtualHost>
" >> /etc/apache2/sites-available/filegator.conf

a2dissite 000-default.conf
a2ensite filegator.conf
systemctl restart apache2

exit
```
Open your browser and go to http://your_server_ip_address



## Show your support

- Please star this repository on [GitHub](https://github.com/filegator/filegator/stargazers) if this project helped you!
- Become a backer or sponsor on [Patreon](https://www.patreon.com/alcalbg).


## Upgrade

Since version 7 is completely rewriten from scratch, there is no clear upgrade path from older versions.

If you have an older version of FileGator please backup everything and install the script again.

Upgrade instructions for non-developers:

- Backup everythig
- Download the latest version
- Replace all files and folders except `repository/` and `private/`

Which versions am I running? Look for `APP_VERSION` inside `dist/index.php` file
