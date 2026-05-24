<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Controllers;

use Filegator\Config\Config;
use Filegator\Controllers\FileController;
use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\MfaCapableInterface;
use Filegator\Services\Mfa\MfaService;
use Filegator\Services\Session\SessionStorageInterface;
use Filegator\Services\Tmpfs\TmpfsInterface;
use Filegator\Services\Logger\LoggerInterface;
use Rakit\Validation\Validator;

class AuthController
{
    const MFA_PENDING_KEY = 'mfa_pending';
    const MFA_PENDING_TTL = 300;

    protected $logger;

    public function __construct(LoggerInterface $logger)
    {
        $this->logger = $logger;
    }

    public function login(Request $request, Response $response, AuthInterface $auth, TmpfsInterface $tmpfs, Config $config, SessionStorageInterface $session, MfaService $mfa)
    {
        $username = (string) $request->input('username');
        $password = (string) $request->input('password');
        $ip = $request->getClientIp();

        if ($this->isLockedOut($tmpfs, $ip, $config)) {
            $this->logger->log("Too many login attempts for {$username} from IP ".$ip);
            return $response->json('Not Allowed', 429);
        }

        // Legacy path first: adapters without the MFA capability go through
        // the original single-step authenticate().
        $supportsMfa = ($auth instanceof MfaCapableInterface) && $mfa->isSupported();
        if (! $supportsMfa) {
            if ($auth->authenticate($username, $password)) {
                $this->seedActiveHomedirAfterLogin($session, $auth, $username);
                $this->logger->log("Logged in {$username} from IP ".$ip);
                return $response->json($this->userResponsePayload($auth->user(), $session));
            }
            return $this->failLogin($tmpfs, $response, $username, $ip);
        }

        // MFA-capable path: verify password without granting a session, then
        // branch on whether MFA is enabled, required-but-not-enrolled, or off.
        if (! $auth->verifyPasswordOnly($username, $password)) {
            return $this->failLogin($tmpfs, $response, $username, $ip);
        }

        $user = $auth->find($username);
        if (! $user) {
            return $this->failLogin($tmpfs, $response, $username, $ip);
        }

        try {
            $mfaState = $auth->getMfaState($username);
        } catch (\Throwable $e) {
            $this->logger->log("getMfaState failed for {$username}: ".$e->getMessage());
            return $this->failLogin($tmpfs, $response, $username, $ip);
        }

        if ($mfaState['enabled']) {
            $session->set(self::MFA_PENDING_KEY, [
                'username' => $username,
                'expires' => time() + self::MFA_PENDING_TTL,
                'phase' => 'verify',
            ]);
            $session->migrate(true);
            $this->logger->log("Password ok for {$username} from {$ip}; awaiting MFA");
            return $response->json(['mfa_required' => true]);
        }

        if ($mfa->isRequiredForUser($username, $user->getRole())) {
            $enrollment = $mfa->beginEnrollment($username);
            $session->set(self::MFA_PENDING_KEY, [
                'username' => $username,
                'expires' => time() + self::MFA_PENDING_TTL,
                'phase' => 'setup',
            ]);
            $session->migrate(true);
            $this->logger->log("Admin {$username} from {$ip} forced into MFA setup");
            return $response->json([
                'mfa_setup_required' => true,
                'enrollment' => $enrollment,
            ]);
        }

        // Plain login: password already verified above, so bypass authenticate()'s
        // bcrypt rerun and use establishSessionFor to finalise the session.
        $auth->establishSessionFor($username);
        $this->seedActiveHomedirAfterLogin($session, $auth, $username);
        $this->logger->log("Logged in {$username} from IP ".$ip);
        return $response->json($this->userResponsePayload($auth->user(), $session));
    }

    public function loginMfa(Request $request, Response $response, AuthInterface $auth, TmpfsInterface $tmpfs, Config $config, SessionStorageInterface $session, MfaService $mfa)
    {
        $ip = $request->getClientIp();
        $pending = $session->get(self::MFA_PENDING_KEY);

        // Single-use: clear immediately regardless of outcome.
        $session->set(self::MFA_PENDING_KEY, null);

        if (! is_array($pending) || empty($pending['username']) || ($pending['expires'] ?? 0) < time()) {
            return $response->json('MFA challenge expired or missing', 422);
        }
        if (($pending['phase'] ?? '') !== 'verify') {
            return $response->json('Invalid MFA phase', 422);
        }

        if ($this->isLockedOut($tmpfs, $ip, $config, 'mfa')) {
            return $response->json('Not Allowed', 429);
        }

        $username = (string) $pending['username'];
        $code = (string) $request->input('code', '');
        $useBackup = (bool) $request->input('use_backup', false);

        $ok = $useBackup
            ? $mfa->consumeBackupCode($username, $code)
            : $mfa->verifyTotp($username, $code);

        if (! $ok) {
            $tmpfs->write(md5($ip).'.mfa.lock', 'x', true);
            $this->logger->log("MFA failed for {$username} from {$ip}");
            return $response->json('Invalid code', 422);
        }

        $user = $auth->find($username);
        if (! $user) {
            return $response->json('User not found', 422);
        }

        // Complete login: bypass password check (already verified at step 1).
        $this->completeMfaLogin($auth, $session, $username);

        $this->logger->log("MFA login complete for {$username} from {$ip}");
        return $response->json($this->userResponsePayload($auth->user(), $session));
    }

    public function loginMfaSetup(Request $request, Response $response, AuthInterface $auth, TmpfsInterface $tmpfs, Config $config, SessionStorageInterface $session, MfaService $mfa)
    {
        $ip = $request->getClientIp();
        $pending = $session->get(self::MFA_PENDING_KEY);

        // Single-use: clear immediately regardless of outcome (mirrors loginMfa).
        $session->set(self::MFA_PENDING_KEY, null);

        if (! is_array($pending) || empty($pending['username']) || ($pending['expires'] ?? 0) < time()) {
            return $response->json('Setup session expired', 422);
        }
        if (($pending['phase'] ?? '') !== 'setup') {
            return $response->json('Invalid MFA phase', 422);
        }
        if ($this->isLockedOut($tmpfs, $ip, $config, 'mfa')) {
            return $response->json('Not Allowed', 429);
        }

        $username = (string) $pending['username'];
        $code = (string) $request->input('code', '');

        $backupCodes = $mfa->confirmEnrollment($username, $code);
        if ($backupCodes === null) {
            $tmpfs->write(md5($ip).'.mfa.lock', 'x', true);
            return $response->json('Invalid code', 422);
        }

        $this->completeMfaLogin($auth, $session, $username);

        $this->logger->log("MFA setup complete for {$username} from {$ip}");
        return $response->json([
            'user' => $this->userResponsePayload($auth->user(), $session),
            'backup_codes' => $backupCodes,
        ]);
    }

    public function loginMfaCancel(Response $response, SessionStorageInterface $session)
    {
        $session->set(self::MFA_PENDING_KEY, null);
        return $response->json('ok');
    }

    public function logout(Response $response, AuthInterface $auth)
    {
        return $response->json($auth->forget());
    }

    public function getUser(Response $response, AuthInterface $auth, SessionStorageInterface $session)
    {
        $user = $auth->user() ?: $auth->getGuest();

        return $response->json($this->userResponsePayload($user, $session));
    }

    /**
     * Build the public user-response payload. Merges User::jsonSerialize
     * with the session-stored active_homedir so the frontend bootstrap
     * (and Login.vue post-auth path) can route multi-folder users
     * straight into '/' when their previously-picked folder is still
     * valid, instead of bouncing back to the picker on every reload.
     */
    protected function userResponsePayload($user, SessionStorageInterface $session): array
    {
        $payload = $user->jsonSerialize();
        $payload['active_homedir'] = $session->get(FileController::SESSION_ACTIVE_HOMEDIR, null);
        return $payload;
    }

    public function changePassword(Request $request, Response $response, AuthInterface $auth, Validator $validator)
    {
        $validator->setMessage('required', 'This field is required');
        $validation = $validator->validate($request->all(), [
            'oldpassword' => 'required',
            'newpassword' => 'required',
        ]);

        if ($validation->fails()) {
            $errors = $validation->errors();

            return $response->json($errors->firstOfAll(), 422);
        }

        if (! $auth->authenticate($auth->user()->getUsername(), $request->input('oldpassword'))) {
            return $response->json(['oldpassword' => 'Wrong password'], 422);
        }

        return $response->json($auth->update($auth->user()->getUsername(), $auth->user(), $request->input('newpassword')));
    }

    protected function completeMfaLogin(AuthInterface $auth, SessionStorageInterface $session, string $username): void
    {
        if ($auth instanceof MfaCapableInterface) {
            $auth->establishSessionFor($username);
        } else {
            $user = $auth->find($username);
            if ($user) $auth->store($user);
            $session->migrate(true);
        }

        // Both branches above migrate the session (preserving data); we
        // then seed the active homedir for single-folder users so the
        // first file-op request after login doesn't need a picker step.
        $this->seedActiveHomedirAfterLogin($session, $auth, $username);
    }

    /**
     * For single-folder users, write SESSION_ACTIVE_HOMEDIR and
     * SESSION_CWD now so file-op requests can act immediately. For
     * multi-folder users, leave the keys unset — the frontend routes
     * them through SelectFolder.vue, which calls POST /selectfolder to
     * populate the session.
     *
     * Reads live homedirs via auth->find() so a mid-deploy admin edit
     * is honoured on the very first post-login request.
     */
    protected function seedActiveHomedirAfterLogin(SessionStorageInterface $session, AuthInterface $auth, string $username): void
    {
        $fresh = $auth->find($username);
        if (! $fresh) return;

        $homedirs = $fresh->getHomeDirs();
        if (count($homedirs) === 1) {
            $session->set(FileController::SESSION_ACTIVE_HOMEDIR, $homedirs[0]);
            $session->set(FileController::SESSION_CWD, '/');
        }
    }

    protected function failLogin(TmpfsInterface $tmpfs, Response $response, string $username, string $ip)
    {
        $this->logger->log("Login failed for {$username} from IP ".$ip);
        $tmpfs->write(md5($ip).'.lock', 'x', true);
        return $response->json('Login failed, please try again', 422);
    }

    protected function isLockedOut(TmpfsInterface $tmpfs, string $ip, Config $config, string $namespace = ''): bool
    {
        $suffix = $namespace ? '.'.$namespace.'.lock' : '.lock';
        $lockfile = md5($ip).$suffix;
        $lockout_attempts = (int) $config->get('lockout_attempts', 5);
        $lockout_timeout = (int) $config->get('lockout_timeout', 15);

        foreach ($tmpfs->findAll($lockfile) as $flock) {
            if (time() - $flock['time'] >= $lockout_timeout) {
                $tmpfs->remove($flock['name']);
            }
        }

        return $tmpfs->exists($lockfile) && strlen($tmpfs->read($lockfile)) >= $lockout_attempts;
    }
}
