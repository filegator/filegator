---
currentMenu: sessions
---

## Default Session handler

Session handling is provided through the Symfony's [HttpFoundation](https://symfony.com/doc/4.4/components/http_foundation.html) component. Please check their docs for more info.

Default session handler will user PHP's built in file storage. You can also specify your own `$save_path` to store session files.

```
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'handler' => function () {
                    $save_path = null; // use default system path
                    //$save_path = __DIR__.'/private/sessions';
                    $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\NativeFileSessionHandler($save_path);

                    return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([], $handler);
                },
            ],
        ],
```
## Configuring Session service to use database

First, create a table `sessions` with this sql:
```
CREATE TABLE `sessions` (
      `sess_id` varbinary(128) NOT NULL,
      `sess_data` blob NOT NULL,
      `sess_lifetime` mediumint(9) NOT NULL,
      `sess_time` int(10) unsigned NOT NULL,
      PRIMARY KEY (`sess_id`)
) CHARSET=utf8 COLLATE=utf8_bin;
```

Then, open `configuration.php` and update Session handler to:

```
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'handler' => function () {
                    $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\PdoSessionHandler(
                            'mysql://root:password@localhost:3306/filegator'
                            );

                    return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([], $handler);
                },
            ],
        ],

```
Don't forget to enter correct database details.

## Configuring Session service to use Redis

You must require additional [predis](https://github.com/nrk/predis/) library `composer require predis/predis`

```
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'handler' => function () {
                    $predis = new \Predis\Client('tcp://127.0.0.1:6379');
                    $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\RedisSessionHandler($predis);

                    return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([], $handler);
                },
            ],
        ],
```

## Tweaking session options

The underying [session component](https://github.com/symfony/symfony/blob/4.4/src/Symfony/Component/HttpFoundation/Session/Storage/NativeSessionStorage.php) accepts array of options.
For example you can pass `cookie_lifetime` parameter to extend default session lifetime:
```
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'handler' => function () {
                    $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\PdoSessionHandler(
                            'mysql://root:password@localhost:3306/filegator'
                            );

                    return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([
                            'cookie_lifetime' => 365 * 24 * 60 * 60, // one year
                    ], $handler);
                },
            ],
        ],

```
