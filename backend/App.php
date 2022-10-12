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

        foreach ($config->get('services', []) as $key => $service) {
            $container->set($key, $container->get($service['handler']));
            $container->get($key)->init(isset($service['config']) ? $service['config'] : []);
        }

        $response->send();

        $this->container = $container;
    }

    public function resolve($name)
    {
        return $this->container->get($name);
    }
}
