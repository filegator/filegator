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
use Filegator\Services\Audit\AuditMailer;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\MfaCapableInterface;
use Filegator\Services\Auth\MfaLockout;
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
            $nonce = bin2hex(random_bytes(8));
            $session->set(self::MFA_PENDING_KEY, [
                'username' => $username,
                'expires' => time() + self::MFA_PENDING_TTL,
                'phase' => 'verify',
                'binding' => $this->buildPendingBinding($request, $config),
                'nonce' => $nonce,
            ]);
            $session->migrate(true);
            $this->logger->log("Password ok for {$username} from {$ip}; awaiting MFA");
            return $response->json(['mfa_required' => true, 'mfa_nonce' => $nonce]);
        }

        if ($mfa->isRequiredForUser($username, $user->getRole())) {
            $enrollment = $mfa->beginEnrollment($username);
            $nonce = bin2hex(random_bytes(8));
            $session->set(self::MFA_PENDING_KEY, [
                'username' => $username,
                'expires' => time() + self::MFA_PENDING_TTL,
                'phase' => 'setup',
                'binding' => $this->buildPendingBinding($request, $config),
                'nonce' => $nonce,
            ]);
            $session->migrate(true);
            $this->logger->log("Admin {$username} from {$ip} forced into MFA setup");
            return $response->json([
                'mfa_setup_required' => true,
                'enrollment' => $enrollment,
                'mfa_nonce' => $nonce,
            ]);
        }

        // Plain login: password already verified above, so bypass authenticate()'s
        // bcrypt rerun and use establishSessionFor to finalise the session.
        $auth->establishSessionFor($username);
        $this->seedActiveHomedirAfterLogin($session, $auth, $username);
        $this->logger->log("Logged in {$username} from IP ".$ip);
        return $response->json($this->userResponsePayload($auth->user(), $session));
    }

    public function loginMfa(Request $request, Response $response, AuthInterface $auth, TmpfsInterface $tmpfs, Config $config, SessionStorageInterface $session, MfaService $mfa, MfaLockout $lockout, AuditMailer $audit)
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

        // Binding + nonce checks share the generic "expired or missing"
        // error so a probing client cannot distinguish "wrong UA" from
        // "no pending" from "stale nonce".
        if (! $this->pendingBindingMatches($pending, $request, $config)) {
            return $response->json('MFA challenge expired or missing', 422);
        }
        if (! $this->pendingNonceMatches($pending, (string) $request->input('mfa_nonce', ''))) {
            return $response->json('MFA challenge expired or missing', 422);
        }

        $username = (string) $pending['username'];

        // Per-IP AND per-username lockout. Per-username closes the rotating-IP
        // brute-force window the per-IP check alone leaves open.
        if ($lockout->isLocked($ip, $username)) {
            return $response->json('Not Allowed', 429);
        }

        $code = (string) $request->input('code', '');
        $useBackup = (bool) $request->input('use_backup', false);

        $ok = $useBackup
            ? $mfa->consumeBackupCode($username, $code)
            : $mfa->verifyTotp($username, $code);

        if (! $ok) {
            $lockout->recordFailure($ip, $username);
            $this->logger->log("MFA failed for {$username} from {$ip}");
            return $response->json('Invalid code', 422);
        }

        $user = $auth->find($username);
        if (! $user) {
            return $response->json('User not found', 422);
        }

        $lockout->clearForUsername($username);

        if ($useBackup && $auth instanceof MfaCapableInterface) {
            $remaining = (int) ($auth->getMfaState($username)['backup_codes_remaining'] ?? 0);
            $audit->mfaBackupCodeConsumed($username, $ip, $remaining);
            if ($remaining <= 2) {
                $this->logger->log("WARNING: {$username} has {$remaining} MFA backup codes remaining after consume from {$ip}");
            }
        }

        // Complete login: bypass password check (already verified at step 1).
        $this->completeMfaLogin($auth, $session, $username);

        $this->logger->log("MFA login complete for {$username} from {$ip}");
        return $response->json($this->userResponsePayload($auth->user(), $session));
    }

    public function loginMfaSetup(Request $request, Response $response, AuthInterface $auth, TmpfsInterface $tmpfs, Config $config, SessionStorageInterface $session, MfaService $mfa, MfaLockout $lockout)
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

        if (! $this->pendingBindingMatches($pending, $request, $config)) {
            return $response->json('Setup session expired', 422);
        }
        if (! $this->pendingNonceMatches($pending, (string) $request->input('mfa_nonce', ''))) {
            return $response->json('Setup session expired', 422);
        }

        $username = (string) $pending['username'];
        if ($lockout->isLocked($ip, $username)) {
            return $response->json('Not Allowed', 429);
        }

        $code = (string) $request->input('code', '');

        $backupCodes = $mfa->confirmEnrollment($username, $code);
        if ($backupCodes === null) {
            $lockout->recordFailure($ip, $username);
            return $response->json('Invalid code', 422);
        }

        $lockout->clearForUsername($username);

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

    /**
     * Derive a hash binding the pending-MFA state to the requesting
     * client. Defaults to User-Agent only — UA is more stable than IP
     * across NAT'd corporate egress and mobile-carrier rotation, and
     * still defeats the cookie-theft scenario (attacker likely on a
     * different OS/browser than the victim). Operators can opt into
     * an IP-prefix component for high-security deploys.
     */
    protected function buildPendingBinding(Request $request, Config $config): string
    {
        $components = [];

        if ((bool) $config->get('mfa_pending_bind_ua', true)) {
            $components[] = 'ua=' . (string) $request->headers->get('User-Agent', '');
        }

        $ipMode = $config->get('mfa_pending_bind_ip_prefix', null);
        if ($ipMode !== null && $ipMode !== '') {
            $components[] = 'ip=' . $this->normalizeIpForBinding((string) $request->getClientIp(), (string) $ipMode);
        }

        // No components configured → empty stable binding (effectively off).
        return hash('sha256', implode('|', $components));
    }

    protected function pendingBindingMatches(array $pending, Request $request, Config $config): bool
    {
        $stored = (string) ($pending['binding'] ?? '');
        $current = $this->buildPendingBinding($request, $config);
        return hash_equals($stored, $current);
    }

    /**
     * Compare stored vs request nonce with hash_equals to avoid timing
     * leaks. Treats absent/empty nonces as mismatch so legacy clients
     * that don't echo the nonce back are forced to upgrade.
     */
    protected function pendingNonceMatches(array $pending, string $supplied): bool
    {
        $stored = (string) ($pending['nonce'] ?? '');
        if ($stored === '' || $supplied === '') return false;
        return hash_equals($stored, $supplied);
    }

    protected function normalizeIpForBinding(string $ip, string $mode): string
    {
        if ($mode === 'exact') return $ip;

        // Accept "/24" or "/48"; reduce IP to its prefix.
        if (preg_match('#^/(\d+)$#', $mode, $m)) {
            $bits = (int) $m[1];
            $packed = @inet_pton($ip);
            if ($packed === false) return $ip;
            $bytes = strlen($packed);
            $fullBytes = intdiv($bits, 8);
            $remainBits = $bits % 8;
            $masked = substr($packed, 0, $fullBytes);
            if ($remainBits > 0 && $fullBytes < $bytes) {
                $mask = chr((0xFF << (8 - $remainBits)) & 0xFF);
                $masked .= ($packed[$fullBytes] & $mask);
            }
            // Zero-pad remaining bytes for canonical form.
            $masked = str_pad($masked, $bytes, "\0");
            $back = @inet_ntop($masked);
            return $back === false ? $ip : ($back . '/' . $bits);
        }

        // Unknown mode — fall back to exact (safe-by-default).
        return $ip;
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
