<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Tests\TestCase;

/**
 * @internal
 */
class FeatureTest extends TestCase
{
    public function testHome()
    {
        $this->sendRequest('GET', '/');

        $this->assertOk();
    }

    public function testMethodNotAllowed()
    {
        $this->sendRequest('DELETE', '/');

        $this->assertStatus(401);
    }

    public function testNotFoundPage()
    {
        $this->sendRequest('GET', '/fakeroute');

        $this->assertStatus(404);
    }

    public function testUnprocessableRequest()
    {
        $this->sendRequest('POST', '/login', 'dddddd');

        $this->assertUnprocessable();
    }

    public function testGetFrontendConfig()
    {
        $this->sendRequest('GET', '/getconfig');

        $this->assertOk();
        $this->assertResponseJsonHas([
            'data' => [
                'app_name' => 'FileGator',
                'upload_max_size' => 2 * 1024 * 1024,
                'upload_chunk_size' => 1 * 1024 * 1024,
                'upload_simultaneous' => 3,
            ],
        ]);
    }
}
