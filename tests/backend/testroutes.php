<?php

return [
    [
        'route' => [
            'GET', '/', '\Filegator\Controllers\ViewController@index',
        ],
        'roles' => [
            'guest',
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
            'GET', '/noguests', 'ProtectedController@protectedMethod',
        ],
        'roles' => [
            'user', 'admin',
        ],
        'permissions' => [
        ],
    ],
    [
        'route' => [
            'GET', '/adminonly', 'AdminController@adminOnlyMethod',
        ],
        'roles' => [
            'admin',
        ],
        'permissions' => [
        ],
    ],
];
