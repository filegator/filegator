<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Auth;

use Filegator\Kernel\Request;
use Filegator\Services\Auth\User;
use Filegator\Services\Session\Adapters\SessionStorage;
use Symfony\Component\HttpFoundation\Session\Storage\MockFileSessionStorage;
use Tests\TestCase;
use Exception;

abstract class AuthTest extends TestCase
{
    public $auth;

    protected $session;

    protected function setUp(): void
    {
        $this->session = new SessionStorage(new Request());
        $this->session->init([
            'handler' => function () {
                return new MockFileSessionStorage();
            },
        ]);

        $this->setAuth();

        parent::setUp();
    }

    abstract public function setAuth();

    public function addAdmin($password = '123456')
    {
        $admin = new User();
        $admin->setRole('admin');
        $admin->setHomedir('/');
        $admin->setUsername('admin@example.com');
        $admin->setName('Admin');

        return $this->auth->add($admin, $password);
    }

    public function addMike($password = '98747')
    {
        $mike = new User();
        $mike->setRole('user');
        $mike->setHomedir('/');
        $mike->setUsername('mike@example.com');
        $mike->setName('Mike');

        return $this->auth->add($mike, $password);
    }

    public function addGuest()
    {
        $guest = new User();
        $guest->setRole('guest');
        $guest->setHomedir('/');
        $guest->setUsername('guest');
        $guest->setName('Guest');

        return $this->auth->add($guest, '');
    }

    public function testWeCanFindAUser()
    {
        $this->addAdmin();

        $user = $this->auth->find('admin@example.com');

        $this->assertTrue($user->isAdmin());
    }

    public function testWeCanAddAUser()
    {
        $mike = $this->addMike();

        $user = $this->auth->find('mike@example.com');
        $this->assertEquals($mike, $user);
    }

    public function testWeCanUpdateExistingUser()
    {
        $this->addAdmin();
        $user = $this->auth->find('admin@example.com');
        $this->assertNotNull($user);

        $user->setName('Jonny B');
        $user->setHomeDir('/jonnyshome');
        $user->setUsername('jonny@example.com');
        $user->setRole('user');

        $updated_user = $this->auth->update('admin@example.com', $user);

        $this->assertEquals('Jonny B', $updated_user->getName());
        $this->assertEquals('/jonnyshome', $updated_user->getHomeDir());
        $this->assertEquals('jonny@example.com', $updated_user->getUsername());
        $this->assertTrue($updated_user->isUser());
    }

    public function testWeCanAuthenticateUser()
    {
        $admin = $this->addAdmin('test123');

        $this->auth->authenticate('admin@example.com', 'test123');
        $auth_user = $this->auth->user();
        $this->assertEquals($auth_user->getUsername(), $admin->getUsername());

        $this->assertFalse($this->auth->authenticate('test123@example.com', 'xxxxxx'));
    }

    public function testWeCanForgetUser()
    {
        $admin = $this->addAdmin('test123');

        $this->auth->authenticate('admin@example.com', 'test123');
        $auth_user = $this->auth->user();
        $this->assertEquals($admin->getUsername(), $auth_user->getUsername());

        $this->auth->forget();

        $auth_user = $this->auth->user();
        $this->assertNull($auth_user);
    }

    public function testWeCanUpdateUsersPassword()
    {
        $admin = $this->addAdmin('test123');

        $this->auth->update('admin@example.com', $admin, 'newpassword');

        $this->assertFalse($this->auth->authenticate('test123@example.com', 'test123'));

        $this->auth->authenticate('admin@example.com', 'newpassword');
        $auth_user = $this->auth->user();

        $this->assertEquals($admin->getUsername(), $auth_user->getUsername());
    }

    public function testWeCanDeleteUser()
    {
        $admin = $this->addAdmin();
        $find = $this->auth->find('admin@example.com');
        $this->assertNotNull($find);

        $this->auth->delete($admin);
        $find = $this->auth->find('admin@example.com');
        $this->assertNull($find);
    }

    public function testWeCannotUpdateNonExistingUser()
    {
        $this->expectException(Exception::class);

        $user = new User();
        $user->setRole('user');
        $user->setHomedir('/');
        $user->setUsername('tim@example.com');
        $user->setName('Tim');

        $this->auth->update('somebody@example.com', $user);
    }

    public function testWeCannotDeleteNonExistingUser()
    {
        $user = new User();
        $user->setRole('user');
        $user->setHomedir('/');
        $user->setUsername('tim@example.com');
        $user->setName('Tim');

        $this->expectException(Exception::class);

        $this->auth->delete($user);
    }

    public function testWeCannotAddUserWithTheSameUsername()
    {
        $this->addAdmin();

        $second_admin = new User();
        $second_admin->setRole('user');
        $second_admin->setHomedir('/');
        $second_admin->setUsername('admin@example.com');
        $second_admin->setName('Admin2');

        $this->expectException(Exception::class);

        $this->auth->add($second_admin, 'pass444');
    }

    public function testWeCannotEditUserAndSetUsernameThatIsAlreadyTaken()
    {
        $this->addMike();
        $this->addAdmin();
        $user = $this->auth->find('mike@example.com');
        $this->assertNotNull($user);

        $user->setName('Jonny B');
        $user->setHomeDir('/jonnyshome');
        $user->setUsername('admin@example.com');
        $user->setRole('user');

        $this->expectException(Exception::class);
        $this->auth->update('mike@example.com', $user);
    }

    public function testNoGuestException()
    {
        $this->expectException(Exception::class);
        $this->auth->getGuest();
    }

    public function testGetGuest()
    {
        $this->addGuest();
        $guest = $this->auth->getGuest();

        $this->assertNotNull($guest);
    }

    public function testGetAllUsers()
    {
        $this->addAdmin();
        $this->addMike();

        $this->assertEquals(2, $this->auth->allUsers()->length());
    }
}
