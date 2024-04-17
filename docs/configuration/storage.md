---
currentMenu: storage
---

## Adapters
Different storage adapters are provided through the awesome [Flysystem](https://github.com/thephpleague/flysystem) library.

You can use local filesystem (default), FTP, SFTP, Amazon S3, DigitalOcean Spaces, Microsoft Azure Blob, Dropbox and many others.

Please check the Flysystem [docs](https://flysystem.thephpleague.com/v1/docs/) for the exact setup required for each adapter.

Notes: Some adapters do not support folder operations or their support is limited.
Requiring additional libraries with [composer](https://getcomposer.org/download/) requires root access.

## Default Local Disk Adapter
With default adapter you just need to configure where your `repository` folder is. This folder will serve as a root for everything else.

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'adapter' => function () {
                  return new \League\Flysystem\Adapter\Local(
                      __DIR__.'/repository'
                      );
                },
            ],
        ],

```

## FTP Adapter
See official [documentation](https://flysystem.thephpleague.com/v1/docs/adapter/ftp/).

Sample configuration:

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'adapter' => function () {
                  return new \Filegator\Services\Storage\Adapters\FilegatorFtp([
                      'host' => 'example.com',
                      'username' => 'demo',
                      'password' => 'password',
                      'port' => 21,
                      'timeout' => 10,
                  ]);
                },
            ],
        ],

```

## SFTP Adapter
You must require additional library `composer require league/flysystem-sftp:^1.0 -W`

For more advanced options like using your private key or changing the document root see official [documentation](https://flysystem.thephpleague.com/v1/docs/adapter/sftp/).

Sample configuration:

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'adapter' => function () {
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

```
## Dropbox Adapter
You must require additional library `composer require spatie/flysystem-dropbox`

See official [documentation](https://flysystem.thephpleague.com/v1/docs/adapter/dropbox/).

Sample configuration:

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [
                    'case_sensitive' => false,
                ],
                'adapter' => function () {
                  $authorizationToken = '1234';
                  $client = new \Spatie\Dropbox\Client($authorizationToken);

                  return new \Spatie\FlysystemDropbox\DropboxAdapter($client);
                },
            ],
        ],

```

## Amazon S3 Adapter (v3)
You must require additional library `composer require league/flysystem-aws-s3-v3:^1.0 -W`

See official [documentation](https://flysystem.thephpleague.com/v1/docs/adapter/aws-s3-v3/).

Sample configuration:

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'adapter' => function () {
                    $client = new \Aws\S3\S3Client([
                        'credentials' => [
                            'key' => '123456',
                            'secret' => 'secret123456',
                        ],
                        'region' => 'us-east-1',
                        'version' => 'latest',
                    ]);

                    return new \League\Flysystem\AwsS3v3\AwsS3Adapter($client, 'my-bucket-name');
                },
            ],
        ],

```

## DigitalOcean Spaces
You must require additional library `composer require league/flysystem-aws-s3-v3:^1.0 -W`

The DigitalOcean Spaces API are compatible with those of S3.

See official [documentation](https://flysystem.thephpleague.com/v1/docs/adapter/digitalocean-spaces/).

Sample configuration:

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'adapter' => function () {
                    $client = new \Aws\S3\S3Client([
                        'credentials' => [
                            'key' => '123456',
                            'secret' => 'secret123456',
                        ],
                        'region' => 'us-east-1',
                        'version' => 'latest',
                        'endpoint' => 'https://nyc3.digitaloceanspaces.com',
                    ]);

                    return new \League\Flysystem\AwsS3v3\AwsS3Adapter($client, 'my-bucket-name');
                },
            ],
        ],

```
## Microsoft Azure Blob Storage
You must require additional library `composer require league/flysystem-azure-blob-storage:^1.0 -W`

See official [documentation](https://flysystem.thephpleague.com/v1/docs/adapter/azure/).

Sample configuration:

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [],
                'adapter' => function () {
                    $accountName = 'your_storage_account_name';
                    $accountKey = '123456';
                    $containerName = 'my_container';

                    $client = \MicrosoftAzure\Storage\Blob\BlobRestProxy::createBlobService(
                        "DefaultEndpointsProtocol=https;AccountName=${accountName};AccountKey=${accountKey};"
                    );

                    return new \League\Flysystem\AzureBlobStorage\AzureBlobStorageAdapter($client, $containerName);
                },
            ],
        ],
```

## Replicate Adapter
You must require additional library `composer require league/flysystem-replicate-adapter`

The ReplicateAdapter facilitates smooth transitions between adapters, allowing an application to stay functional and migrate its files from one adapter to another. The adapter takes two other adapters, a source and a replica. Every change is delegated to both adapters, while all the read operations are passed onto the source only.

See official [documentation](https://flysystem.thephpleague.com/v1/docs/adapter/replicate/).

Sample configuration:

```
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'config' => [
                    'case_sensitive' => false,
                ],
                'adapter' => function () {
                    $authorizationToken = '1234';
                    $client = new \Spatie\Dropbox\Client($authorizationToken);

                    $source = new \Spatie\FlysystemDropbox\DropboxAdapter($client);
                    $replica = new \League\Flysystem\Adapter\Local(__DIR__.'/repository');

                    return new League\Flysystem\Replicate\ReplicateAdapter($source, $replica);
                },
            ],
        ],

```
