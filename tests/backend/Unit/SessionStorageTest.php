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
use Filegator\Services\Session\Adapters\SessionStorage;
use Tests\TestCase;

/**
 * @internal
 */
class SessionStorageTest extends TestCase
{
    protected $session_service;

    protected function setUp(): void
    {
        $this->session_service = new SessionStorage(new Request());
        $this->session_service->init([
            'handler' => function () {
                return new \Symfony\Component\HttpFoundation\Session\Storage\MockFileSessionStorage();
            },
        ]);

        parent::setUp();
    }

    public function testInvalidateSession()
    {
        $this->session_service->set('test1', 444);
        $this->session_service->invalidate();

        $this->assertNull($this->session_service->get('test1'));
    }

    public function testInvalidateSessionWhichIsNotStartedYet()
    {
        $this->session_service->invalidate();

        $this->assertNull($this->session_service->get('something'));
    }

    public function testUseSession()
    {
        $this->session_service->set('test2', 999);
        $this->session_service->save();

        $this->assertEquals(999, $this->session_service->get('test2'));
        $this->assertNull($this->session_service->get('test1'));
    }
}
