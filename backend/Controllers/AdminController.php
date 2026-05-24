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
use Filegator\Services\Audit\AuditMailer;
use Filegator\Services\Audit\WeeklyDigest;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\MfaCapableInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Storage\Filesystem;
use Rakit\Validation\Validator;

class AdminController
{
    protected $auth;

    protected $storage;

    protected $logger;

    public function __construct(AuthInterface $auth, Filesystem $storage, LoggerInterface $logger)
    {
        $this->auth = $auth;
        $this->storage = $storage;
        $this->logger = $logger;
    }

    public function listUsers(Request $request, Response $response, WeeklyDigest $digest)
    {
        $collection = $this->auth->allUsers();
        // Adapter-specific batch read of MFA metadata in a single file scan,
        // avoiding 2N getMfaState+getEmail calls.
        $meta = method_exists($this->auth, 'allUsersMeta') ? $this->auth->allUsersMeta() : [];

        $rows = [];
        foreach ($collection->all() as $user) {
            $row = $user->jsonSerialize();
            $u = $meta[$user->getUsername()] ?? null;
            if ($u !== null) {
                $row['email'] = $u['email'];
                $row['mfa_enabled'] = (bool) $u['enabled'];
                $row['backup_codes_remaining'] = (int) $u['backup_codes_remaining'];
            }
            $rows[] = $row;
        }

        // Piggy-back the weekly digest check on the admin-panel entry point.
        // Admins almost always open the user list when administering, so this
        // is the natural place to wake the scheduler without polluting every
        // file-listing or upload request with a state-file stat. Cheap when
        // not due (one flock + JSON decode).
        $digest->maybeFire($this->auth);

        return $response->json($rows);
    }

    public function storeUser(User $user, Request $request, Response $response, Validator $validator, AuditMailer $audit)
    {
        $validator->setMessage('required', 'This field is required');
        $validation = $validator->validate($request->all(), [
            'name' => 'required',
            'username' => 'required',
            'password' => 'required',
        ]);

        if ($validation->fails()) {
            $errors = $validation->errors();

            return $response->json($errors->firstOfAll(), 422);
        }

        $homedirs = $this->normaliseHomedirsInput($request);
        if (empty($homedirs)) {
            return $response->json(['homedir' => 'This field is required'], 422);
        }

        $email = $request->input('email', null);
        if (! $this->emailValid($email)) {
            return $response->json(['email' => 'Invalid email address'], 422);
        }

        if ($this->auth->find($request->input('username'))) {
            return $response->json(['username' => 'Username already taken'], 422);
        }

        try {
            $user->setName($request->input('name'));
            $user->setUsername($request->input('username'));
            // Apply the admin-prefix join to EACH supplied homedir. Same
            // shape as the pre-refactor single-string join, just looped.
            // Admin is assumed single-folder (Elliff CPA invariant); we
            // use the first homedir of whoever is acting as admin.
            $adminBase = rtrim((string) ($this->auth->user()->getHomeDirs()[0] ?? ''), $this->storage->getSeparator());
            $separator = $this->storage->getSeparator();
            $user->setHomedirs(array_map(function ($h) use ($adminBase, $separator) {
                return $adminBase . $separator . ltrim((string) $h, $separator);
            }, $homedirs));
            $user->setRole($request->input('role', 'user'));
            $user->setPermissions($request->input('permissions'));
            $ret = $this->auth->add($user, $request->input('password'));

            if ($email !== null && $this->auth instanceof MfaCapableInterface) {
                $this->auth->setEmail($user->getUsername(), $email === '' ? null : $email);
            }

            $audit->userCreated(
                $this->currentAdminUsername(),
                $user->jsonSerialize(),
                ($email === null || $email === '') ? null : $email
            );
        } catch (\Exception $e) {
            return $response->json($e->getMessage(), 422);
        }

        return $response->json($ret);
    }

    public function updateUser($username, Request $request, Response $response, Validator $validator, AuditMailer $audit)
    {
        $user = $this->auth->find($username);

        if (! $user) {
            return $response->json('User not found', 422);
        }

        $validator->setMessage('required', 'This field is required');
        $validation = $validator->validate($request->all(), [
            'name' => 'required',
            'username' => 'required',
        ]);

        if ($validation->fails()) {
            $errors = $validation->errors();

            return $response->json($errors->firstOfAll(), 422);
        }

        $homedirs = $this->normaliseHomedirsInput($request);
        if (empty($homedirs)) {
            return $response->json(['homedir' => 'This field is required'], 422);
        }

        if ($username != $request->input('username') && $this->auth->find($request->input('username'))) {
            return $response->json(['username' => 'Username already taken'], 422);
        }

        $email = $request->input('email', null);
        if (! $this->emailValid($email)) {
            return $response->json(['email' => 'Invalid email address'], 422);
        }

        $beforeSnapshot = $user->jsonSerialize();
        $beforeEmail = null;
        if ($this->auth instanceof MfaCapableInterface) {
            $state = $this->auth->getMfaState($username);
            $beforeEmail = $state['email'] ?? null;
        }
        $passwordChanged = $request->input('password', '') !== '';

        try {
            $user->setName($request->input('name'));
            $user->setUsername($request->input('username'));
            // updateUser preserves the existing asymmetry: NO admin-prefix
            // join. Supplied homedirs are stored verbatim (matches the
            // pre-refactor scalar updateUser behaviour, pinned in Phase 1).
            $user->setHomedirs($homedirs);
            $user->setRole($request->input('role', 'user'));
            $user->setPermissions($request->input('permissions'));

            $ret = $this->auth->update($username, $user, $request->input('password', ''));

            if ($email !== null && $this->auth instanceof MfaCapableInterface) {
                $this->auth->setEmail($user->getUsername(), $email === '' ? null : $email);
            }

            // If the request omitted the email field, the previous value
            // is preserved; only an explicit empty string clears it.
            $afterEmail = $email === null ? $beforeEmail : ($email === '' ? null : $email);

            $audit->userUpdated(
                $this->currentAdminUsername(),
                $username,
                $beforeSnapshot,
                $user->jsonSerialize(),
                $beforeEmail,
                $afterEmail,
                $passwordChanged
            );

            return $response->json($ret);
        } catch (\Exception $e) {
            return $response->json($e->getMessage(), 422);
        }
    }

    public function deleteUser($username, Request $request, Response $response, AuditMailer $audit)
    {
        $user = $this->auth->find($username);

        if (! $user || $user->getUsername() == 'guest') {
            return $response->json('User not found', 422);
        }

        $snapshot = $user->jsonSerialize();
        $email = null;
        if ($this->auth instanceof MfaCapableInterface) {
            $state = $this->auth->getMfaState($username);
            $email = $state['email'] ?? null;
        }

        $ret = $this->auth->delete($user);

        if ($ret) {
            $audit->userDeleted($this->currentAdminUsername(), $snapshot, $email);
        }

        return $response->json($ret);
    }

    public function resetMfa($username, Request $request, Response $response, AuditMailer $audit)
    {
        if (! $this->auth instanceof MfaCapableInterface) {
            return $response->json('Not supported', 501);
        }

        $current = $this->auth->user();
        if ($current && $current->getUsername() === $username) {
            return $response->json('Cannot reset your own MFA from the admin panel', 422);
        }

        $target = $this->auth->find($username);
        if (! $target) {
            return $response->json('User not found', 422);
        }

        $this->auth->disableMfa($username);
        $this->logger->log(sprintf(
            'Admin %s reset MFA for user %s from IP %s',
            $current ? $current->getUsername() : 'unknown',
            $username,
            $request->getClientIp()
        ));

        $audit->mfaResetByAdmin($this->currentAdminUsername(), $username);

        return $response->json('ok');
    }

    protected function emailValid($email): bool
    {
        if ($email === null || $email === '') return true;
        return (bool) filter_var($email, FILTER_VALIDATE_EMAIL);
    }

    protected function currentAdminUsername(): string
    {
        $current = $this->auth->user();
        return $current ? $current->getUsername() : 'unknown';
    }

    /**
     * Read the homedirs list from the request, supporting both shapes
     * during the transition:
     * - new: `homedirs` (array of strings)
     * - legacy: `homedir` (single string) — wrapped into a 1-element array
     *
     * Returns a clean array (trimmed, non-empty entries, re-indexed) or
     * empty array if nothing usable was provided.
     */
    protected function normaliseHomedirsInput(Request $request): array
    {
        $raw = $request->input('homedirs', null);
        if (is_array($raw)) {
            $clean = [];
            foreach ($raw as $h) {
                if (! is_string($h)) continue;
                $t = trim($h);
                if ($t === '') continue;
                $clean[] = $t;
            }
            return $clean;
        }

        // Legacy single scalar — accept for one release so the existing
        // frontend can keep submitting `homedir` until Phase 7 lands.
        $legacy = $request->input('homedir', null);
        if (is_string($legacy)) {
            $t = trim($legacy);
            if ($t !== '') return [$t];
        }

        return [];
    }
}
