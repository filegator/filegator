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
            'POST', '/login/mfa', '\Filegator\Controllers\AuthController@loginMfa',
        ],
        'roles' => [
            'guest',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/login/mfa/setup', '\Filegator\Controllers\AuthController@loginMfaSetup',
        ],
        'roles' => [
            'guest',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/login/mfa/cancel', '\Filegator\Controllers\AuthController@loginMfaCancel',
        ],
        'roles' => [
            'guest',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'GET', '/mfa/state', '\Filegator\Controllers\MfaController@state',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/mfa/enroll/begin', '\Filegator\Controllers\MfaController@beginEnroll',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/mfa/enroll/confirm', '\Filegator\Controllers\MfaController@confirmEnroll',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/mfa/disable', '\Filegator\Controllers\MfaController@disable',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/mfa/backup_codes/regenerate', '\Filegator\Controllers\MfaController@regenerateBackupCodes',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/me/email', '\Filegator\Controllers\MfaController@updateEmail',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/password/forgot', '\Filegator\Controllers\PasswordResetController@request',
        ],
        'roles' => [
            'guest',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/password/reset/validate', '\Filegator\Controllers\PasswordResetController@validateToken',
        ],
        'roles' => [
            'guest',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'POST', '/password/reset', '\Filegator\Controllers\PasswordResetController@confirm',
        ],
        'roles' => [
            'guest',
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
        // Switch the user's active folder. Required for multi-folder users
        // before any file-op endpoint will succeed; for single-folder users
        // the active homedir is auto-seeded at login.
        'route' => [
            'POST', '/selectfolder', '\Filegator\Controllers\FileController@selectFolder',
        ],
        'roles' => [
            'user', 'admin',
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
            'POST', '/admin/users/{username}/reset_mfa', '\Filegator\Controllers\AdminController@resetMfa',
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
