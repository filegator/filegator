---
currentMenu: logging
---

## Configuring Logging service

Logging is provided trough the powerful [Monolog](https://github.com/Seldaek/monolog) library. Please check their docs for more info.

Default handler will use simple `private/logs/app.log` file to store application logs and errors.

```
        'Filegator\Services\Logger\LoggerInterface' => [
            'handler' => '\Filegator\Services\Logger\Adapters\MonoLogger',
            'config' => [
                'monolog_handlers' => [
                    function () {
                        return new \Monolog\Handler\StreamHandler(
                            __DIR__.'/private/logs/app.log',
                            \Monolog\Logger::DEBUG
                        );
                    },
                ],
            ],
        ],
```

There are many different handlers you can add on top of the stack (monolog_handlers array). Some of them are listed [here](https://github.com/Seldaek/monolog#documentation).
