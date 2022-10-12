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
class AdminTest extends TestCase
{
    public function testOnlyAdminCanPerformUserActions()
    {
        $this->signOut();

        $this->sendRequest('GET', '/listusers');
        $this->assertStatus(404);

        $this->sendRequest('POST', '/storeuser');
        $this->assertStatus(404);

        $this->sendRequest('POST', '/updateuser/test@example.com');
        $this->assertStatus(404);

        $this->sendRequest('POST', '/deleteuser/test@example.com');
        $this->assertStatus(404);
    }

    public function testListUsers()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('GET', '/listusers');
        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                [
                    'role' => 'guest',
                    'permissions' => [],
                    'homedir' => '/',
                    'username' => 'guest',
                    'name' => 'Guest',
                ],
                [
                    'role' => 'admin',
                    'permissions' => [],
                    'homedir' => '/',
                    'username' => 'admin@example.com',
                    'name' => 'Admin',
                ],
                [
                    'role' => 'user',
                    'permissions' => [],
                    'homedir' => '/john',
                    'username' => 'john@example.com',
                    'name' => 'John Doe',
                ],
                [
                    'role' => 'user',
                    'permissions' => [],
                    'homedir' => '/jane',
                    'username' => 'jane@example.com',
                    'name' => 'Jane Doe',
                ],
            ],
        ]);
    }

    public function testAddingNewUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Mike Test',
            'username' => 'mike@example.com',
            'role' => 'user',
            'permissions' => [],
            'password' => 'pass123',
            'homedir' => '/john',
        ]);
        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'role' => 'user',
                'permissions' => [],
                'homedir' => '/john',
                'username' => 'mike@example.com',
                'name' => 'Mike Test',
            ],
        ]);
    }

    public function testAddingNewUserValidation()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => '',
            'username' => '',
            'role' => 'user',
            'permissions' => [],
            'password' => 'pass123',
            'homedir' => '',
        ]);
        $this->assertStatus(422);

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Mike Test',
            'username' => 'mike@example.com',
            'role' => 'bear',
            'permissions' => ['xxx'],
            'password' => 'pass123',
            'homedir' => '/john',
        ]);
        $this->assertStatus(422);
    }

    public function testUpdatingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'Johnny Doe',
            'username' => 'john2@example.com',
            'role' => 'admin',
            'permissions' => ['read', 'write'],
            'homedir' => '/jane',
        ]);
        $this->assertOk();

        $this->assertResponseJsonHas([
            'data' => [
                'role' => 'admin',
                'permissions' => ['read', 'write'],
                'homedir' => '/jane',
                'username' => 'john2@example.com',
                'name' => 'Johnny Doe',
            ],
        ]);
    }

    public function testDeletingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/deleteuser/john@example.com');
        $this->assertOk();
    }

    public function testUpdatingNonExistingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/nonexisting@example.com');
        $this->assertStatus(422);
    }

    public function testUpdatingUserValidation()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => '',
            'username' => '',
            'homedir' => '',
        ]);
        $this->assertStatus(422);

        $this->sendRequest('POST', '/updateuser/john@example.com', [
            'name' => 'something',
            'username' => 'something',
            'homedir' => '/',
            'permissions' => ['xxx', 'write'],
        ]);
        $this->assertStatus(422);
    }

    public function testDeletingNonExistingUser()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/deleteuser/nonexisting@example.com');
        $this->assertStatus(422);
    }

    public function testAddingOrEditingUserWithUsernameThatIsAlreadyTaken()
    {
        $this->signIn('admin@example.com', 'admin123');

        $this->sendRequest('POST', '/storeuser', [
            'name' => 'Mike Test',
            'username' => 'admin@example.com',
            'role' => 'user',
            'password' => '123',
            'permissions' => [],
            'homedir' => '/mike',
        ]);

        $this->assertStatus(422);

        $this->sendRequest('POST', '/updateuser/admin@example.com', [
            'name' => 'Admin',
            'username' => 'john@example.com',
            'role' => 'admin',
            'permissions' => [],
            'homedir' => '/',
        ]);

        $this->assertStatus(422);
    }
}
