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
            'POST', '/api/login', '\Filegator\Controllers\AuthController@login',
        ],
        'roles' => [
            'guest',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/api/logout', '\Filegator\Controllers\AuthController@logout',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'GET', '/api/getuser', '\Filegator\Controllers\AuthController@getUser',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/api/changepassword', '\Filegator\Controllers\AuthController@changePassword',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'GET', '/api/getconfig', '\Filegator\Controllers\ViewController@getFrontendConfig',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/api/changedir', '\Filegator\Controllers\FileController@changeDirectory',
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
            'POST', '/api/getdir', '\Filegator\Controllers\FileController@getDirectory',
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
            'POST', '/api/copyitems', '\Filegator\Controllers\FileController@copyItems',
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
            'POST', '/api/moveitems', '\Filegator\Controllers\FileController@moveItems',
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
            'POST', '/api/renameitem', '\Filegator\Controllers\FileController@renameItem',
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
            'POST', '/api/zipitems', '\Filegator\Controllers\FileController@zipItems',
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
            'POST', '/api/unzipitem', '\Filegator\Controllers\FileController@unzipItem',
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
            'POST', '/api/deleteitems', '\Filegator\Controllers\FileController@deleteItems',
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
            'POST', '/api/createnew', '\Filegator\Controllers\FileController@createNew',
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
            'GET', '/api/upload', '\Filegator\Controllers\UploadController@chunkCheck',
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
            'POST', '/api/upload', '\Filegator\Controllers\UploadController@upload',
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
            'POST', '/api/batchdownload', '\Filegator\Controllers\DownloadController@batchDownloadCreate',
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
            'GET', '/api/batchdownload', '\Filegator\Controllers\DownloadController@batchDownloadStart',
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
            'GET', '/api/listusers', '\Filegator\Controllers\AdminController@listUsers',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/api/storeuser', '\Filegator\Controllers\AdminController@storeUser',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/api/updateuser/{username}', '\Filegator\Controllers\AdminController@updateUser',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/api/deleteuser/{username}', '\Filegator\Controllers\AdminController@deleteUser',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/api/savecontent', '\Filegator\Controllers\FileController@saveContent',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'read', 'write',
        ],
    ],
];
