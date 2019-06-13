<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\App;
use Filegator\Config\Config;
use Filegator\Container\Container;
use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Services\View\Adapters\Vuejs;
use Tests\FakeResponse;
use Tests\FakeStreamedResponse;
use Tests\TestCase;

/**
 * @internal
 */
class MainTest extends TestCase
{
    public function testMainApp()
    {
        $config = new Config();
        $request = new Request();
        $response = new FakeResponse();
        $sresponse = new FakeStreamedResponse();
        $container = new Container();

        $app = new App($config, $request, $response, $sresponse, $container);

        $this->assertEquals($app->resolve(Config::class), $config);
        $this->assertEquals($app->resolve(Request::class), $request);
        $this->assertInstanceOf(Response::class, $app->resolve(Response::class));
    }

    public function testServices()
    {
        $config = [
            'services' => [
                'Service1' => [
                    'handler' => 'Filegator\Services\View\Adapters\Vuejs',
                ],
                'Service2' => [
                    'handler' => 'Filegator\Services\View\Adapters\Vuejs',
                ],
            ],
        ];

        $app = new App(new Config($config), new Request(), new FakeResponse(), new FakeStreamedResponse(), new Container());

        $this->assertEquals($app->resolve('Service1'), new Vuejs(new Config($config)));
        $this->assertEquals($app->resolve('Service2'), new Vuejs(new Config($config)));
    }
}
