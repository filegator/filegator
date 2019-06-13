<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Services\Auth\User;
use Filegator\Services\Auth\UsersCollection;
use Filegator\Services\Storage\DirectoryCollection;
use Filegator\Utils\Collection;
use Tests\TestCase;

/**
 * @internal
 */
class CollectionTest extends TestCase
{
    public function testAddToCollection()
    {
        $mock = $this->getMockForTrait(Collection::class);
        $mock->add('one');
        $mock->add('two');

        $this->assertEquals($mock->length(), 2);
    }

    public function testDeleteFromCollection()
    {
        $mock = $this->getMockForTrait(Collection::class);
        $mock->add('one');
        $mock->delete('one');

        $this->assertEquals($mock->length(), 0);
    }

    public function testSort()
    {
        $mock = $this->getMockForTrait(Collection::class);
        $mock->add(['val' => 'b']);
        $mock->add(['val' => 'a']);
        $mock->add(['val' => 'c']);

        $this->assertEquals($mock->all()[0]['val'], 'b');

        $mock->sortByValue('val');

        $this->assertEquals($mock->all()[0]['val'], 'a');

        $mock->sortByValue('val', true);

        $this->assertEquals($mock->all()[0]['val'], 'c');
    }

    public function testUsersCollection()
    {
        $mock = new UsersCollection();

        $user1 = new User();
        $user2 = new User();
        $user3 = new User();

        $mock->addUser($user1);
        $mock->addUser($user2);
        $mock->addUser($user3);

        $this->assertEquals($mock->length(), 3);
    }

    public function testUserSerialization()
    {
        $mock = new UsersCollection();
        $mock->add(['val' => 'b']);
        $mock->add(['val' => 'a']);
        $mock->add(['val' => 'c']);

        $json = json_encode($mock);

        $this->assertEquals($json, '[{"val":"b"},{"val":"a"},{"val":"c"}]');
    }

    public function testDirectoryCollection()
    {
        $dir = new DirectoryCollection('/sub1/sub2');

        $dir->addFile('back', '/sub1', '..', 0, 1558942228);
        $dir->addFile('file', '/sub1/sub2/test.txt', 'test.txt', 30000, 1558942228);
        $dir->addFile('file', '/sub1/sub2/test2.txt', 'test.txt', 30000, 1558942228);
        $dir->addFile('dir', '/sub1/sub2/sub3', 'sub3', 0, 1558942228);

        $json = json_encode($dir);

        $this->assertEquals($json, '{"location":"\/sub1\/sub2","files":[{"type":"back","path":"\/sub1","name":"..","size":0,"time":1558942228},{"type":"dir","path":"\/sub1\/sub2\/sub3","name":"sub3","size":0,"time":1558942228},{"type":"file","path":"\/sub1\/sub2\/test.txt","name":"test.txt","size":30000,"time":1558942228},{"type":"file","path":"\/sub1\/sub2\/test2.txt","name":"test.txt","size":30000,"time":1558942228}]}');

        $this->expectException(\Exception::class);
        $dir->addFile('badType', 'aaa', 'aa', 0, 1558942228);
    }

    public function testUserCollection()
    {
        $user = new UsersCollection();

        $user->addUser(new User());
        $user->addUser(new User());

        $json = json_encode($user);

        $this->assertEquals($json, '[{"role":"guest","permissions":[],"homedir":"","username":"","name":""},{"role":"guest","permissions":[],"homedir":"","username":"","name":""}]');
    }
}
