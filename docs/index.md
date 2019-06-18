## FileGator

[FileGator](https://filegator.io) is a free, open-source PHP script for managing files and folders.

You can manage files inside your local repository folder (on your server's hard drive) or connect to other storage adaptes (see below).

FileGator has multi-user support so you can have admins and other users managing the files with different access permissions, roles and home folders.

All basic file operations are supported: copy, move, rename, create, delete, zip, unzip, download, upload.

If allowed, users can download multiple files or folders at once.

File upload supports drag&drop, progress bar, pause and resume. Upload is chunked so you should be able to upload large files regardless of your server's configuration.



## Features & Goals
- Multiple storage adapters (Local, FTP, S3, Dropbox and many others via [Flysystem](https://github.com/thephpleague/flysystem))
- Multiple auth adapters with roles and permissions (Store users in json file or database)
- Multiple session adapters (Native File, Pdo, MongoDB, Memcached and others via [Symfony](https://github.com/symfony/symfony/tree/master/src/Symfony/Component/HttpFoundation/Session/Storage/Handler))
- Single page front-end (built with [Vuejs](https://github.com/vuejs/vue), [Bulma](https://github.com/jgthms/bulma) and [Buefy](https://github.com/buefy/buefy))
- Chunked uploads (built with [Resumable.js](https://github.com/23/resumable.js))
- Zip and bulk download support
- Highly extensible, decoupled and tested code
- No database required
- Framework free [™](https://www.youtube.com/watch?v=L5jI9I03q8E)

