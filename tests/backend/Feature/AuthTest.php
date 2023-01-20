<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Tests\TestCase;

/**
 * @internal
 */
class AuthTest extends TestCase
{
    public function testSuccessfulLogin()
    {
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);

        $this->assertOk();
    }

    public function testBadLogin()
    {
        $this->sendRequest('POST', '/login', [
            'username' => 'fake',
            'password' => 'fake',
        ]);

        $this->assertUnprocessable();
    }

    public function testBruteForceLogin()
    {
        $this->sendRequest('POST', '/login', [
            'username' => 'fake',
            'password' => 'fake',
        ], [], ['REMOTE_ADDR' => '10.10.10.10']);
        $this->assertUnprocessable();

        for ($i = 0; $i < 20; $i++) {
            $this->sendRequest('POST', '/login', [
                'username' => 'fake',
                'password' => 'fake',
            ], [], ['REMOTE_ADDR' => '10.10.10.10']);
        }
        $this->assertStatus(429);

        for ($i = 0; $i < 20; $i++) {
            $this->sendRequest('POST', '/login', [
                'username' => 'fake',
                'password' => 'fake',
            ], [], ['REMOTE_ADDR' => '2001:db8:3333:4444:5555:6666:7777:8888']);
        }
        $this->assertStatus(429);


        // new ip should be ok
        $this->sendRequest('POST', '/login', [
            'username' => 'fake',
            'password' => 'fake',
        ], [], ['REMOTE_ADDR' => '10.10.10.1']);
        $this->assertUnprocessable();
    }

    public function testAlreadyLoggedIn()
    {
        $username = 'john@example.com';
        $this->signIn($username, 'john123');

        $this->sendRequest('POST', '/login', ['username' => $username, 'password' => 'john123']);

        $this->assertStatus(404);
    }

    public function testGetUser()
    {
        $user = 'john@example.com';
        $this->signIn($user, 'john123');

        $this->sendRequest('GET', '/getuser');

        $this->assertOk();
        $this->assertResponseJsonHas([
            'data' => [
                'username' => $user,
                'name' => 'John Doe',
                'role' => 'user',
                'homedir' => '/john',
            ],
        ]);
    }

    public function testGetAdmin()
    {
        $admin = 'admin@example.com';
        $this->signIn($admin, 'admin123');

        $this->sendRequest('GET', '/getuser');

        $this->assertOk();
        $this->assertResponseJsonHas([
            'data' => [
                'username' => $admin,
                'name' => 'Admin',
                'role' => 'admin',
                'homedir' => '/',
            ],
        ]);
    }

    public function testReceiveGuestIfNoUserIsLoggedIn()
    {
        $this->sendRequest('GET', '/getuser');

        $this->assertOk();
        $this->assertResponseJsonHas([
            'data' => [
                'role' => 'guest',
            ],
        ]);
    }

    public function testLogout()
    {
        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('POST', '/logout');

        $this->assertOk();
    }

    public function testResponseThrows404()
    {
        $request = Request::create(
            '?r=/notfound',
            'GET'
            );

        $app = $this->bootFreshApp(null, $request);

        $response = $app->resolve(Response::class);

        $this->assertEquals($response->getStatusCode(), 404);
    }

    public function testChangePassword()
    {
        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('POST', '/changepassword', [
            'oldpassword' => 'john123',
            'newpassword' => '',
        ]);
        $this->assertStatus(422);

        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('POST', '/changepassword', [
            'oldpassword' => 'wrongpass',
            'newpassword' => 'password123',
        ]);
        $this->assertStatus(422);

        $this->signIn('john@example.com', 'john123');
        $this->sendRequest('POST', '/changepassword', [
            'oldpassword' => 'john123',
            'newpassword' => 'password123',
        ]);
        $this->assertOk();
    }
}
