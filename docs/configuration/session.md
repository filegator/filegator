
### Configuring Session service to use database

First, create a table ```sessions``` with this sql:
```
CREATE TABLE `sessions` (
      `sess_id` varbinary(128) NOT NULL,
      `sess_data` blob NOT NULL,
      `sess_lifetime` mediumint(9) NOT NULL,
      `sess_time` int(10) unsigned NOT NULL,
      PRIMARY KEY (`sess_id`)
) CHARSET=utf8 COLLATE=utf8_bin;
```

Then, open ```configuration.php``` and update Auth handler under section ```services``` to something like this:

```
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'session_handler' => 'database',
                'available' => [
                    'database' => function () {
                        $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\PdoSessionHandler(
                            'mysql://root:password@localhost:3360/filegator'
                        );

                        return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([], $handler);
                    },
                ],
            ],
        ],

```
Don't forget to enter correct mysql username, password, and database.


### Tweaking session options

The Underying Symfony session [component](https://github.com/symfony/symfony/blob/4.4/src/Symfony/Component/HttpFoundation/Session/Storage/NativeSessionStorage.php) constructor accepts an array options.
For example you can pass ```cookie_lifetime``` parameter and extend session lifetime like this:
```
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'session_handler' => 'database',
                'available' => [
                    'database' => function () {
                        $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\PdoSessionHandler(
                            'mysql://root:password@localhost:3360/filegator'
                        );

                        return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([
                            'cookie_lifetime' => 365 * 24 * 60 * 60, // one year
                        ], $handler);
                    },
                ],
            ],
        ],

```
