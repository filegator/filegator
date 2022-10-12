<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Kernel\Request;
use Tests\TestCase;

/**
 * @internal
 */
class RequestTest extends TestCase
{
    public function testGetRequest()
    {
        $request = Request::create(
            '?r=/test&a=1&b=2',
            'GET'
        );

        $this->assertEquals([
            'r' => '/test',
            'a' => '1',
            'b' => '2',
        ], $request->all());

        $this->assertEquals('/test', $request->input('r'));
        $this->assertEquals('1', $request->input('a'));
        $this->assertEquals('2', $request->input('b'));
    }

    public function testPostRequest()
    {
        $request = Request::create(
            '/somewhere',
            'POST',
            ['param1' => '1', 'param2' => '2']
            );

        $this->assertEquals([
            'param1' => '1',
            'param2' => '2',
        ], $request->all());

        $this->assertEquals('1', $request->input('param1'));
        $this->assertEquals('2', $request->input('param2'));
    }

    public function testJsonRequest()
    {
        $request = Request::create(
            '',
            'GET',
            [],
            [],
            [],
            [],
            json_encode(['sample' => 'content'])
        );

        $this->assertEquals([
            'sample' => 'content',
        ], $request->all());

        $this->assertEquals('content', $request->input('sample'));
    }

    public function testGetAndJsonParametersTogether()
    {
        $request = Request::create(
            '/test?priority=1',
            'POST',
            [],
            [],
            [],
            [],
            json_encode(['sample' => 'content', 'more' => '1'])
        );

        $this->assertEquals([
            'priority' => '1',
            'sample' => 'content',
            'more' => '1',
        ], $request->all());

        $this->assertEquals('1', $request->input('priority'));
        $this->assertEquals('content', $request->input('sample'));
        $this->assertEquals('1', $request->input('more'));
    }

    public function testGetPostParametersTogether()
    {
        $request = Request::create(
            '/test?priority=10&something=else',
            'POST',
            ['param' => 'param1', 'priority' => 5]
            );

        $this->assertEquals([
            'priority' => '10',
            'something' => 'else',
            'param' => 'param1',
        ], $request->all());

        $this->assertEquals('10', $request->input('priority'));
        $this->assertEquals('else', $request->input('something'));
        $this->assertEquals('param1', $request->input('param'));
    }

    public function testGetPostAndJsonParametersTogether()
    {
        $request = Request::create(
            '/test?priority=10&something=else',
            'POST',
            ['param' => 'param1', 'priority' => 5],
            [],
            [],
            [],
            json_encode(['sample' => 'content', 'priority' => '2'])
        );

        $this->assertEquals([
            'priority' => '10',
            'something' => 'else',
            'param' => 'param1',
            'sample' => 'content',
        ], $request->all());

        $this->assertEquals('10', $request->input('priority'));
        $this->assertEquals('else', $request->input('something'));
        $this->assertEquals('param1', $request->input('param'));
        $this->assertEquals('content', $request->input('sample'));
    }
}
