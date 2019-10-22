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

        $this->assertEquals($config, $app->resolve(Config::class));
        $this->assertEquals($request, $app->resolve(Request::class));
        $this->assertInstanceOf(Response::class, $app->resolve(Response::class));
    }
}
