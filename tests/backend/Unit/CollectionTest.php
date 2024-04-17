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
use Exception;

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

        $this->assertEquals(2, $mock->length());
    }

    public function testDeleteFromCollection()
    {
        $mock = $this->getMockForTrait(Collection::class);
        $mock->add('one');
        $mock->delete('one');

        $this->assertEquals(0, $mock->length());
    }

    public function testSort()
    {
        $mock = $this->getMockForTrait(Collection::class);
        $mock->add(['val' => 'b']);
        $mock->add(['val' => 'a']);
        $mock->add(['val' => 'c']);

        $this->assertEquals('b', $mock->all()[0]['val']);

        $mock->sortByValue('val');

        $this->assertEquals('a', $mock->all()[0]['val']);

        $mock->sortByValue('val', true);

        $this->assertEquals('c', $mock->all()[0]['val']);
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

        $this->assertEquals(3, $mock->length());
    }

    public function testUserSerialization()
    {
        $mock = new UsersCollection();
        $mock->add(['val' => 'b']);
        $mock->add(['val' => 'a']);
        $mock->add(['val' => 'c']);

        $json = json_encode($mock);

        $this->assertEquals('[{"val":"b"},{"val":"a"},{"val":"c"}]', $json);
    }

    public function testDirectoryCollection()
    {
        $dir = new DirectoryCollection('/sub1/sub2');

        $dir->addFile('back', '/sub1', '..', 0, 1558942228, 644);
        $dir->addFile('file', '/sub1/sub2/test.txt', 'test.txt', 30000, 1558942228, 644);
        $dir->addFile('file', '/sub1/sub2/test2.txt', 'test.txt', 30000, 1558942228, 644);
        $dir->addFile('dir', '/sub1/sub2/sub3', 'sub3', 0, 1558942228, 644);

        $json = json_encode($dir);

        $this->assertEquals('{"location":"\/sub1\/sub2","files":[{"type":"back","path":"\/sub1","name":"..","size":0,"time":1558942228,"permissions":644},{"type":"dir","path":"\/sub1\/sub2\/sub3","name":"sub3","size":0,"time":1558942228,"permissions":644},{"type":"file","path":"\/sub1\/sub2\/test.txt","name":"test.txt","size":30000,"time":1558942228,"permissions":644},{"type":"file","path":"\/sub1\/sub2\/test2.txt","name":"test.txt","size":30000,"time":1558942228,"permissions":644}]}', $json);

        $this->expectException(Exception::class);
        $dir->addFile('badType', 'aaa', 'aa', 0, 1558942228, 644);
    }

    public function testUserCollection()
    {
        $user = new UsersCollection();

        $user->addUser(new User());
        $user->addUser(new User());

        $json = json_encode($user);

        $this->assertEquals('[{"role":"guest","permissions":[],"homedir":"","username":"","name":""},{"role":"guest","permissions":[],"homedir":"","username":"","name":""}]', $json);
    }
}
