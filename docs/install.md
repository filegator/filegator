---
currentMenu: install
---

## Requirements
- PHP 7.1.3+


## Download precompiled build
Precompiled build is created for non-developers. In this version, the frontend (html & javascript) is compiled for you and the source code is removed so the final archive contains only minimum files.

- Latest: [v7.0.1](https://github.com/filegator/static/raw/master/builds/filegator_v7.0.1.zip)
- Unzip files and upload them to your PHP server
- Make sure you webserver can read and write to `/storage` and `/private` folders
- Set the website document root to `/dist` directory. This is also known as 'public' folder.
- Visit web page, if something goes wrong check `/private/logs/app.log`
- Login with default credentials `admin/admin123`
- Change default admin's password

NOTE: For security reasons `/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser.

## Upgrade

Since version 7 is completely rewriten from scratch, there is no clear upgrade path from the older versions.

If you have an older version of FileGator please backup everything and install the script again.


