---
currentMenu: install
---

## Requirements
- PHP 7.1.3+


## Download precompiled build
Precompiled build is created for non-developers. In this version, the frontend (html, css and javascript) is compiled for you and the source code is removed so the final archive contains only minimum files.

- Download: [v7.3.3](https://github.com/filegator/static/raw/master/builds/filegator_v7.3.3.zip)
- Unzip files and upload them to your PHP server
- Make sure your webserver can read and write to `/repository` and `/private` folders
- Set the website document root to `/dist` directory. This is also known as 'public' folder
- Visit web page, if something goes wrong check `/private/logs/app.log`
- Login with default credentials `admin/admin123`
- Change default admin's password

NOTE: For security reasons `/dist` is the ONLY folder you want to be exposed through the web. Everything else should be outside of your web root, this way people can’t access any of your important files through the browser.


## Show your support

Please star this repository on [GitHub](https://github.com/filegator/filegator/stargazers) if this project helped you!


## Upgrade

Since version 7 is completely rewriten from scratch, there is no clear upgrade path from older versions.

If you have an older version of FileGator please backup everything and install the script again.

Upgrade instructions for non-developers:

- Backup everythig
- Download the latest version
- Replace all files and folders except `repository/` and `private/`

Which versions am I running? Look for `APP_VERSION` inside `dist/index.php` file
