---
currentMenu: router
---

## Router service

Router service is using well-known [FastRoute](https://github.com/nikic/FastRoute) library. There is no need to change this service unless you're extending the script.

The router uses unique query parameter `?r=` to pass the route info. Because of this feature, this (single-page) application does not require rewrite rules, .htaccess or similar tweaks.

Example routes:

- `http://example.com/?r=/some/route&param1=val1&param2=val2`
- `http://example.com/?r=/user/{user_id}&param1=val1`


## Routes file

Routes file is located here `backend/Controllers/routes.php` Each route in the routes array looks like this:


```
    [
        'route' => [
            'GET', '/download/{path_encoded}', '\Filegator\Controllers\DownloadController@download',
        ],
        'roles' => [
            'guest', 'user', 'admin',
        ],
        'permissions' => [
            'download',
        ],
    ],
```

As you can see in the example, you can assign required user roles and permissions for each route.

## Controllers

Since FileGator is using an awesome dependency injection [container](https://github.com/PHP-DI/PHP-DI) you can type-hint dependencies directly in your controllers. 

You can also mix route parameters and dependencies in any order like in this example:

```

    public function __construct(Config $config, Session $session, AuthInterface $auth, Filesystem $storage)
    {
      // ...
    }

    public function download($path_encoded, Request $request, Response $response, StreamedResponse $streamedResponse)
    {
      // ...
    }
```
