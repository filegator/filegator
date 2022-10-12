<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Controllers;

use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Storage\Filesystem;
use Rakit\Validation\Validator;

class AdminController
{
    protected $auth;

    protected $storage;

    public function __construct(AuthInterface $auth, Filesystem $storage)
    {
        $this->auth = $auth;
        $this->storage = $storage;
    }

    public function listUsers(Request $request, Response $response)
    {
        return $response->json($this->auth->allUsers());
    }

    public function storeUser(User $user, Request $request, Response $response, Validator $validator)
    {
        $validator->setMessage('required', 'This field is required');
        $validation = $validator->validate($request->all(), [
            'name' => 'required',
            'username' => 'required',
            'homedir' => 'required',
            'password' => 'required',
        ]);

        if ($validation->fails()) {
            $errors = $validation->errors();

            return $response->json($errors->firstOfAll(), 422);
        }

        if ($this->auth->find($request->input('username'))) {
            return $response->json(['username' => 'Username already taken'], 422);
        }

        try {
            $user->setName($request->input('name'));
            $user->setUsername($request->input('username'));
            $user->setHomedir(
                rtrim($this->auth->user()->getHomeDir(), $this->storage->getSeparator())
                .$this->storage->getSeparator()
                .ltrim($request->input('homedir'), $this->storage->getSeparator())
            );
            $user->setRole($request->input('role', 'user'));
            $user->setPermissions($request->input('permissions'));
            $ret = $this->auth->add($user, $request->input('password'));
        } catch (\Exception $e) {
            return $response->json($e->getMessage(), 422);
        }

        return $response->json($ret);
    }

    public function updateUser($username, Request $request, Response $response, Validator $validator)
    {
        $user = $this->auth->find($username);

        if (! $user) {
            return $response->json('User not found', 422);
        }

        $validator->setMessage('required', 'This field is required');
        $validation = $validator->validate($request->all(), [
            'name' => 'required',
            'username' => 'required',
            'homedir' => 'required',
        ]);

        if ($validation->fails()) {
            $errors = $validation->errors();

            return $response->json($errors->firstOfAll(), 422);
        }

        if ($username != $request->input('username') && $this->auth->find($request->input('username'))) {
            return $response->json(['username' => 'Username already taken'], 422);
        }

        try {
            $user->setName($request->input('name'));
            $user->setUsername($request->input('username'));
            $user->setHomedir($request->input('homedir'));
            $user->setRole($request->input('role', 'user'));
            $user->setPermissions($request->input('permissions'));

            return $response->json($this->auth->update($username, $user, $request->input('password', '')));
        } catch (\Exception $e) {
            return $response->json($e->getMessage(), 422);
        }
    }

    public function deleteUser($username, Request $request, Response $response)
    {
        $user = $this->auth->find($username);

        if (! $user) {
            return $response->json('User not found', 422);
        }

        return $response->json($this->auth->delete($user));
    }
}
