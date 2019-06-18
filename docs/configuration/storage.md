## Adapters
Different storage adapters are provided through the awesome [Flysystem](https://github.com/thephpleague/flysystem) library.

You can use local filesystem (default), FTP, S3, Dropbox and many others.

Please check the Flysystem [docs](https://github.com/thephpleague/flysystem) for the exact setup required for each adapter.

## Default Local Disk Adapter
With default adapter you just need to configure where your ```repository``` folder is. This folder will serve as a root for everything else.

```
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
                ],
            ],
        ],

```

## FTP Adapter
See official [documentation](https://flysystem.thephpleague.com/docs/adapter/ftp/)

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'filesystem_adapter' => 'ftp',
                'adapters' => [
                    'ftp' => function () {
                        return new \League\Flysystem\Adapter\Ftp([
                            'host' => 'example.com',
                            'username' => 'demo',
                            'password' => 'password',
                            'port' => 21,
                            'timeout' => 10,
                        ]);
                    },
                ],
            ],
        ],

```

## SFTP Adapter
You must require additional library ```composer require league/flysystem-sftp```.

See official [documentation](https://flysystem.thephpleague.com/docs/adapter/sftp/).

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'filesystem_adapter' => 'sftp',
                'adapters' => [
                    'sftp' => function () {
                        return new \League\Flysystem\Sftp\SftpAdapter([
                            'host' => 'example.com',
                            'port' => 22,
                            'username' => 'demo',
                            'password' => 'password',
                            'timeout' => 10,
                        ]);
                    },
                ],
            ],
        ],

```
## Dropbox Adapter
You must require additional library ```composer require spatie/flysystem-dropbox```.

See official [documentation](https://flysystem.thephpleague.com/docs/adapter/dropbox/)

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'filesystem_adapter' => 'dropbox',
                'adapters' => [
                    'dropbox' => function () {
                        $authorizationToken = '1234';
                        $client = new \Spatie\Dropbox\Client($authorizationToken);

                        return new \Spatie\FlysystemDropbox\DropboxAdapter($client);
                    },
                ],
            ],
        ],

```
