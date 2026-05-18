<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator;

use Filegator\Config\Config;
use Filegator\Container\Container;
use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Kernel\StreamedResponse;

class App
{
    private $container;

    public function __construct(Config $config, Request $request, Response $response, StreamedResponse $sresponse, Container $container)
    {
        $container->set(Config::class, $config);
        $container->set(Container::class, $container);
        $container->set(Request::class, $request);
        $container->set(Response::class, $response);
        $container->set(StreamedResponse::class, $sresponse);

        $routerKey = 'Filegator\Services\Router\Router';
        $routerSeen = false;
        $servicesAfterRouter = [];

        foreach ($config->get('services', []) as $key => $service) {
            // Track if we've seen the Router
            if ($key === $routerKey) {
                $routerSeen = true;
            } elseif ($routerSeen) {
                // Collect services registered after Router for warning
                $servicesAfterRouter[] = $key;
            }

            $instance = $container->get($service['handler']);
            $container->set($key, $instance);
            $instance->init(isset($service['config']) ? $service['config'] : []);
        }

        // Warn about services registered after Router
        if (!empty($servicesAfterRouter)) {
            error_log('WARNING: The following services are configured AFTER Router in configuration.php: ' . implode(', ', $servicesAfterRouter));
            error_log('WARNING: Router dispatches requests immediately during init(), so these services will NOT be available to controllers.');
            error_log('WARNING: Move these services BEFORE Router in configuration.php to fix injection issues.');
        }

        $response->send();

        $this->container = $container;
    }

    public function resolve($name)
    {
        return $this->container->get($name);
    }
}
