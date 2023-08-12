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
            $uri = rawurldecode($r); // TODO: this is likely not used with my changes now?
        } else {
            if (!empty($this->request->server->get("REQUEST_URI"))) {
                $this->request->query->remove($config['query_param']);
                $uri = strtok($this->request->server->get("REQUEST_URI"),'?'); // get the URL from the... URL funny enough! (stripping off query parameters)
            }
        }

        $routes = require $config['routes_file'];

        $dispatcher = FastRoute\simpleDispatcher(function (FastRoute\RouteCollector $r) use ($routes) {
            if ($routes && ! empty($routes)) {
                foreach ($routes as $params) {
                    if ($this->user->hasRole($params['roles']) && $this->user->hasPermissions($params['permissions'])) {
                        $r->addRoute($params['route'][0], $params['route'][1], $params['route'][2]);
//                        error_log("Added route " . $params['route'][1] . "(" . $params['route'][0] . ")");
                    }
                }
            }
        });

        $routeInfo = $dispatcher->dispatch($http_method, $uri);

        //TODO: remove this / make sure we do serve an error if it doesn't exist?
        $controller = '\Filegator\Controllers\ErrorController';
        $action = 'notFound';
        $params = [];

        switch ($routeInfo[0]) {
            case FastRoute\Dispatcher::FOUND:
                $handler = explode('@', $routeInfo[1]);
                $controller = $handler[0];
                $action = $handler[1];
                $params = $routeInfo[2];

                if ($controller=="\Filegator\Controllers\ViewController") {
                    if (!empty($this->request->server->get("REQUEST_URI"))) {
                        if (!str_starts_with($this->request->server->get("REQUEST_URI"),"/api/")) {
//                            error_log("     LAURIE: OVERRIDING CONTROLLER");
//                            $controller = '\Filegator\Controllers\DownloadController';
//                            $action = "download";
                        }
                    }
                }


                break;
            case FastRoute\Dispatcher::METHOD_NOT_ALLOWED:
                $action = 'methodNotAllowed';

                break;
            case FastRoute\Dispatcher::NOT_FOUND:
                error_log("Couldn't find route for ". $uri . ", trying download");
                $controller = '\Filegator\Controllers\DownloadController';
                $action = "download";
                break;
        }

        error_log("Using controller:");
        error_log($controller);
        error_log("Action:");
        error_log($action);
//         error_log("Params:");
//         error_log(print_r($params,true));
        error_log("URL:"); # LAURIE: my idea is to find the URL bit in this function, then exclude all the normal gubins (probably exclusion list), and if it's not just a root / then try running a download operation
        error_log($this->request->server->get("REQUEST_URI"));
        $this->container->call([$controller, $action], $params);
    }
}
