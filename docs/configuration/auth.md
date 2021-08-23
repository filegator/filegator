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

## Configuring Auth service to use WordPress

Replace your current Auth handler in `configuration.php` file like this:

```
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Filegator\Services\Auth\Adapters\WPAuth',
            'config' => [
                'wp_dir' => '/var/www/my_wordpress_site/',
                'permissions' => ['read', 'write', 'upload', 'download', 'batchdownload', 'zip'],
                'private_repos' => false,
            ],
        ],
```
Adjust in the config above:
- `wp_dir` should be the directory path of your wordpress installation
- `permissions` is the array of permissions given to each user
- `private_repos` each user will have its own sub folder, admin will see everything (false/true)

Note: With more recent versions of FileGator you can set `guest_redirection` in your `configuration.php` to redirect logged-out users back to your WP site:
```
'frontend_config' => [
  ...
    'guest_redirection' => 'http://example.com/wp-admin/',
  ...
]
```

## Configuring Auth service to use LDAP

Replace your current Auth handler in `configuration.php` file like this:

```
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Filegator\Services\Auth\Adapters\LDAP',
            'config' => [
                    'private_repos' => false,
                    'ldap_server'=>'ldap://192.168.1.1',
                    'ldap_bindDN'=>'uid=ldapbinduser,cn=users,dc=ldap,dc=example,dc=com',
                    'ldap_bindPass'=>'ldapbinduser-password',
                    'ldap_baseDN'=>'cn=users,dc=ldap,dc=example,dc=com',
                    'ldap_filter'=>'(uid=*)', //ex: 'ldap_filter'=>'(&(uid=*)(memberOf=cn=administrators,cn=groups,dc=ldap,dc=example,dc=com))',
                    'ldap_attributes' => ["uid","cn","dn"],
                    'ldap_userFieldMapping'=> [
                        'username' =>'uid',
                        'username_AddDomain' =>'@example.com',
                        'username_RemoveDomains' =>['@department1.example.com', '@department2.example.com'],
                        'name' =>'cn',
                        'userDN' =>'dn',
                        'default_permissions' => 'read|write|upload|download|batchdownload|zip',
                        'admin_usernames' =>['user1', 'user2'],
                    ],
            ],
        ],
```

## Custom Authentication using 3rd party

If you want to use FileGator as a part of another application, you probably already have users stored somewhere else. What you need in this case is to build a new custom Auth adapter that matches the [AuthInterface](https://github.com/filegator/filegator/blob/master/backend/Services/Auth/AuthInterface.php) to connect those two. This new adapter will try to authenticate users in your application and translate each user into filegator [User](https://github.com/filegator/filegator/blob/master/backend/Services/Auth/User.php) object.

## API authentication

Front-end will use session based authentication to authenticate and consume the back-end.

Note: The application will not work if you disable cookies.


