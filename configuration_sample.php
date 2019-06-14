<?php

return [
    'public_path' => APP_PUBLIC_PATH,
    'public_dir' => APP_PUBLIC_DIR,

    'frontend_config' => [
        'app_name' => 'FileGator',
        'app_version' => APP_VERSION,
        'language' => 'english',
        'logo' => 'https://raw.githubusercontent.com/filegator/static/master/logo.png',
        'upload_max_size' => 1000000 * 1024 * 1024,
        'upload_chunk_size' => 1 * 1024 * 1024,
        'upload_simultaneous' => 3,
        'default_archive_name' => 'archive.zip',
    ],

    'services' => [
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
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'session_handler' => 'filesession',
                'available' => [
                    'filesession' => function () {
                        $save_path = null; // use default system path
                        //$save_path = __DIR__.'/private/sessions';
                        $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\NativeFileSessionHandler($save_path);

                        return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([], $handler);
                    },
                    'database' => function () {
                        $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\PdoSessionHandler(
                            'mysql://root:password@localhost:3360/filegator'
                        );

                        return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([], $handler);
                    },
                ],
            ],
        ],
        'Filegator\Services\Cors\Cors' => [
            'handler' => '\Filegator\Services\Cors\Cors',
            'config' => [
                'enabled' => APP_ENV == 'production' ? false : true,
            ],
        ],
        'Filegator\Services\Tmpfs\TmpfsInterface' => [
            'handler' => '\Filegator\Services\Tmpfs\Adapters\Tmpfs',
            'config' => [
                'path' => __DIR__.'/private/tmp/',
                'gc_probability_perc' => 10,
                'gc_older_than' => 60 * 60 * 24 * 2, // 2 days
            ],
        ],
        'Filegator\Services\Security\Security' => [
            'handler' => '\Filegator\Services\Security\Security',
            'config' => [
                'csrf_protection' => true,
                'ip_whitelist' => [],
                'ip_blacklist' => [],
            ],
        ],
        'Filegator\Services\View\ViewInterface' => [
            'handler' => '\Filegator\Services\View\Adapters\Vuejs',
        ],
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'filesystem_adapter' => 'localfilesystem',
                'adapters' => [
                    'localfilesystem' => function () {
                        return new \League\Flysystem\Adapter\Local(
                            __DIR__.'/repository'
                        );
                    },
                    'ftp' => function () {
                        // see: https://flysystem.thephpleague.com/docs/adapter/ftp/
                        return new \League\Flysystem\Adapter\Ftp([
                            'host' => 'example.com',
                            'username' => 'demo',
                            'password' => 'password',
                            'port' => 21,
                            'timeout' => 10,
                        ]);
                    },
                    'sftp' => function () {
                        // composer require league/flysystem-sftp
                        // see: https://flysystem.thephpleague.com/docs/adapter/sftp/
                        return new \League\Flysystem\Sftp\SftpAdapter([
                            'host' => 'example.com',
                            'port' => 22,
                            'username' => 'demo',
                            'password' => 'password',
                            'timeout' => 10,
                        ]);
                    },
                    'dropbox' => function () {
                        // composer require spatie/flysystem-dropbox
                        // see: https://flysystem.thephpleague.com/docs/adapter/dropbox/
                        $authorizationToken = '1234';
                        $client = new \Spatie\Dropbox\Client($authorizationToken);

                        return new \Spatie\FlysystemDropbox\DropboxAdapter($client);
                    },
                ],
            ],
        ],
        'Filegator\Services\Archiver\ArchiverInterface' => [
            'handler' => '\Filegator\Services\Archiver\Adapters\ZipArchiver',
            'config' => [],
        ],
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Filegator\Services\Auth\Adapters\JsonFile',
            'config' => [
                'file' => __DIR__.'/private/users.json',
            ],
            //'handler' => '\Filegator\Services\Auth\Adapters\Database',
            //'config' => [
            //    'driver' => 'mysqli',
            //    'host' => 'localhost',
            //    'username' => 'root',
            //    'password' => 'password',
            //    'database' => 'filegator',
            //],
        ],
        'Filegator\Services\Router\Router' => [
            'handler' => '\Filegator\Services\Router\Router',
            'config' => [
                'query_param' => 'r',
                'routes_file' => __DIR__.'/backend/Controllers/routes.php',
            ],
        ],
    ],
];
