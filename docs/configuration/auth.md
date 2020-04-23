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

## Custom Authentication using 3rd party

If you want to use FileGator as a part of another application, you probably already have users stored somewhere else. What you need in this case is to build a new custom Auth adapter that matches the [AuthInterface](https://github.com/filegator/filegator/blob/master/backend/Services/Auth/AuthInterface.php) to connect those two. This new adapter will try to authenticate users in your application and store a [User](https://github.com/filegator/filegator/blob/master/backend/Services/Auth/User.php) object into the filegator's session.

Please look at this simplest [demo auth adapter](https://github.com/filegator/demo_auth_adapter) to see how all this works.


## API authentication

Front-end will use session based authentication to authenticate and consume the back-end.

Note: The application will not work if you disable cookies.


