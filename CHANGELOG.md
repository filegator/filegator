# Changelog

## Upcoming...

## 7.7.0 - 2021-09-27

* Default cookie options added: cookie_httponly=true, cookie_secure=null
* Clickjacking prevention with X-Frame-Options/Content-Security-Policy headers
* Fixes #243, #239, #246, #251, #254, #257
* Slovenian translation added, (Thanks megamiska.eu)
* Dependency bumps
* Docs update
* Default logo update to vector

Notes / Breaking Changes:

The new default value of the cookie_secure option is null, which makes cookies secure when the request is using HTTPS and doesn't modify them when the request uses HTTP. The new behavior is a good balance between making your app "safe by default" and not breaking any existing app.

If your filegator is used inside an iFrame, it may stop working after the upgrade. Set 'allow_insecure_overlays' to true to maintain compatibility. https://github.com/filegator/filegator/blob/63645f6e047eef828a96f913bd421f7018c94e05/configuration_sample.php#L75

## 7.6.0 - 2021-07-12

* Better search with configurable simultaneous search limit, fixes #234

## 7.5.3 - 2021-07-05

* Invalidate sessions when the user is changed, prevents session fixation (json, database)
* Cookie samesite defaults to Lax, fixes #232

## 7.5.2 - 2021-06-24

* Composer update
* Flysystem patch GHSA-9f46-5r25-5wfm
* Min supported PHP version is now 7.2.5

## 7.5.1 - 2021-03-23

* New csrf token key config option added
* Ldap adapter improvements, new config param for attributes, pr #184 (Thanks @lzkill)
* Logger added to security service, fixes #183
* Japanese translation added (Thanks @tubuanha)
* Two consecutive periods bugfix for #202
* Axios auto-transform json turned off, fixes #201

## 7.5.0 - 2021-02-05

* Show filesize and remaining time on download #181 (Thanks @ahaenggli)
* Min supported PHP version is now 7.2

## 7.4.7 - 2021-01-13

* New feature - hiding files/folders on front-end, fixes #76 (Thanks @ahaenggli)
* Fixes #135 (Thanks @ahaenggli)
* Fixes #153 (Thanks @Gui13)
* Fixes #163
* Swedish language added #174 (Thanks leifa71)

## 7.4.6 - 2020-11-02

* New feature - upload folder with drag&drop, fixes #25 (Thanks @ahaenggli)
* New LDAP auth adapter (Thanks @ahaenggli)
* Fixes #17 (Thanks @ahaenggli)
* Hungarian translation added (Thanks zsolti19850610)

## 7.4.5 - 2020-10-12

* New config: 'download_inline' #141 (download configured extensions inline in the browser)
* Korean language added #119 (Thanks Jinhee-Kim)
* Galician language added #126 (Thanks vinpoloaire)
* Russian language added #128 (Thanks BagriyDmitriy)

## 7.4.4 - 2020-07-28 [Security]

* This version patches a security vulnerability #116 please upgrade

## 7.4.3 - 2020-07-18

* disabling axios response auto-transformation when editing content, fixes #110
* config params: .json and .md extensions added as 'editable' by default
* config params: timezone support added, mostly for accurate logging, defaults to UTC
* fixes #102

## 7.4.2 - 2020-07-18

* inclusive terminology: BC! please replace ip_whitelist/ip_blacklist to ip_allowlist/ip_denylist in your configuration.php
* fixes #113 #108
* add mime-types to download headers
* support for vector images (svg)
* fonts update
* catch/fix NavigationDuplicated errors

## 7.4.1 - 2020-05-17

* libzip BC fix
* zip adapter fix
* composer update dependencies
* npm update / audit fix
* right-click opens single file context menu
* fixes #81, #82, #86

## 7.4.0 - 2020-05-09 ✌️

* WordPress Auth adapter is now included in the main repo
* New config: 'guest_redirection' (useful for external auth adapters)
* More css classes so the elements can be easily hidden (e.g. add_to_head style)
* Integrated https://github.com/filegator/filegator/pull/74
* Updated docs

## 7.3.5 - 2020-04-18

* Translations added: Polish, Italian
* Bump symfony, dibi

## 7.3.4 - 2020-03-23

* New config param: overwrite files on upload

## 7.3.3 - 2020-03-08

* Download filename bugfix
* Language fix

## 7.3.2 - 2020-03-06

* View PDF files in the browser (thanks @pauloklaus)
* Fixes #31 #51
* Password reveal added to login screen
* Language fix

## 7.3.1 - 2020-02-26

* Fixes for #41 #42 #43 #45 #46 #47
* Slovak translation (thanks @jannyba)

## 7.3.0 - 2020-02-25

* Search feature

## 7.2.1 - 2020-02-21

* Better editor & image gallery
* New config: `editable` (file extensions that can be opened with editor)
* New config: `date_format`

## 7.2.0 - 2020-02-20

* File preview & edit feature added (preview images, edit txt files)

## 7.1.6 - 2020-01-12

* Translations added, Bulgarian, Serbian, French

## 7.1.5 - 2020-01-09

* Translations added, Dutch, Chinese

## 7.1.4 - 2019-12-30

* npm updates, vue & vue-cli

## 7.1.3 - 2019-12-04

* Symfony update, fixes CVE-2019-18888

## 7.1.2 - 2019-09-09

* Fix for filename sanitize/cut during upload process

## 7.1.1 - 2019-09-05

* Fix for multibyte filename uploads

## 7.1.0 - 2019-09-05

* Fix for UTF8 filename issues (#12) - may brake existing download links generated with previous versions

## 7.0.1 - 2019-08-01

* Fixed file upload bug - merging chunks after upload was failing

## 7.0.0 - 2019-07-26

* Initial release

