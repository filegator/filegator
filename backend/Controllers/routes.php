<?php

return [
    [
        'route' => [
            'GET', '/', '\Filegator\Controllers\ViewController@index',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/login', '\Filegator\Controllers\AuthController@login',
        ],
        'roles' => [
            'guest',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/logout', '\Filegator\Controllers\AuthController@logout',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'GET', '/getuser', '\Filegator\Controllers\AuthController@getUser',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/changepassword', '\Filegator\Controllers\AuthController@changePassword',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'GET', '/getconfig', '\Filegator\Controllers\ViewController@getFrontendConfig',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/changedir', '\Filegator\Controllers\FileController@changeDirectory',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read',
        ],
    ],
    [
        'route' => [
            'POST', '/getdir', '\Filegator\Controllers\FileController@getDirectory',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read',
        ],
    ],
    [
        'route' => [
            'POST', '/copyitems', '\Filegator\Controllers\FileController@copyItems',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write',
        ],
    ],
    [
        'route' => [
            'POST', '/moveitems', '\Filegator\Controllers\FileController@moveItems',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write',
        ],
    ],
    [
        'route' => [
            'POST', '/renameitem', '\Filegator\Controllers\FileController@renameItem',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write',
        ],
    ],
    [
        'route' => [
            'POST', '/zipitems', '\Filegator\Controllers\FileController@zipItems',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write', 'zip',
        ],
    ],
    [
        'route' => [
            'POST', '/unzipitem', '\Filegator\Controllers\FileController@unzipItem',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write', 'zip',
        ],
    ],
    [
        'route' => [
            'POST', '/chmoditems', '\Filegator\Controllers\FileController@chmodItems',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write', 'chmod',
        ],
    ],
    [
        'route' => [
            'POST', '/deleteitems', '\Filegator\Controllers\FileController@deleteItems',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write',
        ],
    ],
    [
        'route' => [
            'POST', '/createnew', '\Filegator\Controllers\FileController@createNew',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write',
        ],
    ],
    [
        'route' => [
            'GET', '/upload', '\Filegator\Controllers\UploadController@chunkCheck',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'upload',
        ],
    ],
    [
        'route' => [
            'POST', '/upload', '\Filegator\Controllers\UploadController@upload',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'upload',
        ],
    ],
    [
        'route' => [
            'GET', '/download', '\Filegator\Controllers\DownloadController@download',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'download',
        ],
    ],
    [
        'route' => [
            'POST', '/batchdownload', '\Filegator\Controllers\DownloadController@batchDownloadCreate',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'download', 'batchdownload',
        ],
    ],
    [
        'route' => [
            'GET', '/batchdownload', '\Filegator\Controllers\DownloadController@batchDownloadStart',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'download', 'batchdownload',
        ],
    ],
    // admins
    [
        'route' => [
            'GET', '/listusers', '\Filegator\Controllers\AdminController@listUsers',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/storeuser', '\Filegator\Controllers\AdminController@storeUser',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/updateuser/{username}', '\Filegator\Controllers\AdminController@updateUser',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/deleteuser/{username}', '\Filegator\Controllers\AdminController@deleteUser',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/savecontent', '\Filegator\Controllers\FileController@saveContent',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write',
        ],
    ],
];
