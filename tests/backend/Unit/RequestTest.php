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

        $this->assertEquals($request->all(), [
            'r' => '/test',
            'a' => '1',
            'b' => '2',
        ]);

        $this->assertEquals($request->input('r'), '/test');
        $this->assertEquals($request->input('a'), '1');
        $this->assertEquals($request->input('b'), '2');
    }

    public function testPostRequest()
    {
        $request = Request::create(
            '/somewhere',
            'POST',
            ['param1' => '1', 'param2' => '2']
            );

        $this->assertEquals($request->all(), [
            'param1' => '1',
            'param2' => '2',
        ]);

        $this->assertEquals($request->input('param1'), '1');
        $this->assertEquals($request->input('param2'), '2');
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

        $this->assertEquals($request->all(), [
            'sample' => 'content',
        ]);

        $this->assertEquals($request->input('sample'), 'content');
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

        $this->assertEquals($request->all(), [
            'priority' => '1',
            'sample' => 'content',
            'more' => '1',
        ]);

        $this->assertEquals($request->input('priority'), '1');
        $this->assertEquals($request->input('sample'), 'content');
        $this->assertEquals($request->input('more'), '1');
    }

    public function testGetPostParametersTogether()
    {
        $request = Request::create(
            '/test?priority=10&something=else',
            'POST',
            ['param' => 'param1', 'priority' => 5],
            );

        $this->assertEquals($request->all(), [
            'priority' => '10',
            'something' => 'else',
            'param' => 'param1',
        ]);

        $this->assertEquals($request->input('priority'), '10');
        $this->assertEquals($request->input('something'), 'else');
        $this->assertEquals($request->input('param'), 'param1');
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

        $this->assertEquals($request->all(), [
            'priority' => '10',
            'something' => 'else',
            'param' => 'param1',
            'sample' => 'content',
        ]);

        $this->assertEquals($request->input('priority'), '10');
        $this->assertEquals($request->input('something'), 'else');
        $this->assertEquals($request->input('param'), 'param1');
        $this->assertEquals($request->input('sample'), 'content');
    }
}
