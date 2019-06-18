
## Configuring Logging service

Logging is provided via powerful [Monolog](https://github.com/Seldaek/monolog) library. Please check their docs for more info.

Default logger handler will use ```/private/logs/app.log``` file to store application logs:

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
