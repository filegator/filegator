---
currentMenu: auth
---

## Default Auth service
By default, users are stored in json file. For some use-cases, this is enough. It also makes this app lightweight since no database is required.

Default handler accepts only file name parameter. This file should be writable by the web server.

```
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Filegator\Services\Auth\Adapters\JsonFile',
            'config' => [
                'file' => __DIR__.'/private/users.json',
            ],
        ],

```

## Configuring Auth service to use database
You can use mysql database to store your users.

First, create a table `users` with this sql query:
```
CREATE TABLE `users` (
    `id` int(10) NOT NULL AUTO_INCREMENT,
    `username` varchar(255) NOT NULL,
    `name` varchar(255) NOT NULL,
    `role` varchar(20) NOT NULL,
    `permissions` varchar(200) NOT NULL,
    `homedir` varchar(2000) NOT NULL,
    `password` varchar(255) NOT NULL,
    PRIMARY KEY (`id`),
    KEY `username` (`username`)
) CHARSET=utf8 COLLATE=utf8_bin;
```
Then, import default users with sql query:

```
INSERT INTO `users` (`username`, `name`, `role`, `permissions`, `homedir`, `password`)
VALUES
('guest', 'Guest', 'guest', '', '/', ''),
('admin', 'Admin', 'admin', 'read|write|upload|download|batchdownload|zip', '/', '$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq');
```

At the end, open `configuration.php` and update AuthInterface handler to reflect your database settings:

```
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Filegator\Services\Auth\Adapters\Database',
            'config' => [
                'driver' => 'mysqli',
                'host' => 'localhost',
                'username' => 'root',
                'password' => 'password',
                'database' => 'filegator',
            ],
        ],
```


## API authentication

Front-end will use session based authentication to authenticate and consume the back-end.

Note: The application will not work if you disable cookies.


