<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Router;

use FastRoute;
use Filegator\Container\Container;
use Filegator\Kernel\Request;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Service;

class Router implements Service
{
    protected $request;

    protected $auth;

    protected $container;

    protected $user;

    public function __construct(Request $request, AuthInterface $auth, Container $container)
    {
        $this->request = $request;
        $this->container = $container;
        $this->user = $auth->user() ?: $auth->getGuest();
    }

    public function init(array $config = [])
    {
        $uri = '/';
        $http_method = $this->request->getMethod();

        if ($r = $this->request->query->get($config['query_param'])) {
            $this->request->query->remove($config['query_param']);
            $uri = rawurldecode($r);
        }

        $routes = require $config['routes_file'];

        $dispatcher = FastRoute\simpleDispatcher(function (FastRoute\RouteCollector $r) use ($routes) {
            if ($routes && ! empty($routes)) {
                foreach ($routes as $params) {
                    if ($this->user->hasRole($params['roles']) && $this->user->hasPermissions($params['permissions'])) {
                        $r->addRoute($params['route'][0], $params['route'][1], $params['route'][2]);
                    }
                }
            }
        });

        $routeInfo = $dispatcher->dispatch($http_method, $uri);

        $controller = '\Filegator\Controllers\ErrorController';
        $action = 'notFound';
        $params = [];

        switch ($routeInfo[0]) {
        case FastRoute\Dispatcher::FOUND:
            $handler = explode('@', $routeInfo[1]);
            $controller = $handler[0];
            $action = $handler[1];
            $params = $routeInfo[2];

            break;
        case FastRoute\Dispatcher::METHOD_NOT_ALLOWED:
            $action = 'methodNotAllowed';

            break;
        }

        $this->container->call([$controller, $action], $params);
    }
}
