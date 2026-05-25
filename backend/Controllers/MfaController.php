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
use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Services\Audit\AuditMailer;
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\MfaCapableInterface;
use Filegator\Services\Auth\MfaLockout;
use Filegator\Services\Auth\RequiresStepUpAuth;
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Mfa\MfaService;

class MfaController
{
    use RequiresStepUpAuth;

    protected $logger;

    public function __construct(LoggerInterface $logger)
    {
        $this->logger = $logger;
    }

    public function state(Response $response, AuthInterface $auth, MfaService $mfa, Config $config)
    {
        if ($this->mfaUnsupported($auth, $mfa)) {
            return $response->json('MFA not supported', 501);
        }

        $user = $auth->user();
        $username = $user->getUsername();

        $state = $auth->getMfaState($username);

        return $response->json([
            'enabled' => $state['enabled'],
            'has_email' => $state['has_email'],
            'backup_codes_remaining' => $state['backup_codes_remaining'],
            'enrolled_at' => $state['enrolled_at'],
            'email' => $state['email'],
            'required_by_role' => $mfa->isRequiredForUser($username, $user->getRole()),
            'mfa_required_for_admins' => (bool) $config->get('mfa_required_for_admins', true),
        ]);
    }

    public function beginEnroll(Response $response, AuthInterface $auth, MfaService $mfa)
    {
        if ($this->mfaUnsupported($auth, $mfa)) {
            return $response->json('MFA not supported', 501);
        }

        $user = $auth->user();

        try {
            $enrollment = $mfa->beginEnrollment($user->getUsername());
        } catch (\RuntimeException $e) {
            // Already enrolled — refuse rather than overwriting the existing secret.
            return $response->json($e->getMessage(), 422);
        }

        return $response->json($enrollment);
    }

    public function confirmEnroll(Request $request, Response $response, AuthInterface $auth, MfaService $mfa)
    {
        if ($this->mfaUnsupported($auth, $mfa)) {
            return $response->json('MFA not supported', 501);
        }

        $user = $auth->user();
        $code = (string) $request->input('code', '');

        if ($code === '') {
            return $response->json(['code' => 'This field is required'], 422);
        }

        $codes = $mfa->confirmEnrollment($user->getUsername(), $code);

        if ($codes === null) {
            return $response->json(['code' => 'Invalid code'], 422);
        }

        // The mutation flipped mfa_enabled, which is part of buildSessionHash —
        // without re-establishing, the current session's hash is now stale and
        // the user gets silently logged out on the next request.
        $auth->establishSessionFor($user->getUsername());

        $this->logger->log("MFA enrolled for {$user->getUsername()}");
        return $response->json(['backup_codes' => $codes]);
    }

    public function disable(Request $request, Response $response, AuthInterface $auth, MfaService $mfa, MfaLockout $lockout, AuditMailer $audit)
    {
        if ($this->mfaUnsupported($auth, $mfa)) {
            return $response->json('MFA not supported', 501);
        }

        $user = $auth->user();
        $username = $user->getUsername();

        if ($mfa->isRequiredForUser($username, $user->getRole())) {
            return $response->json('MFA is required for your role and cannot be disabled', 422);
        }

        $check = $this->stepUpVerify(
            $response, $auth, $mfa, $lockout, $username, $request->getClientIp(),
            (string) $request->input('password', ''),
            (string) $request->input('code', ''),
            (bool) $request->input('use_backup', false)
        );
        if (! $check['ok']) return;

        $this->auditBackupCodeIfUsed($check, $audit, $this->logger, $auth, $username, $request->getClientIp());

        $mfa->disable($username);
        // mfa_enabled flipped — refresh the session hash so we don't log
        // ourselves out on the next request.
        $auth->establishSessionFor($username);
        $this->logger->log("MFA disabled for {$username}");

        $audit->userSelfDisabledMfa($username, $user->getRole());

        return $response->json('ok');
    }

    public function regenerateBackupCodes(Request $request, Response $response, AuthInterface $auth, MfaService $mfa, MfaLockout $lockout, AuditMailer $audit)
    {
        if ($this->mfaUnsupported($auth, $mfa)) {
            return $response->json('MFA not supported', 501);
        }

        $user = $auth->user();
        $username = $user->getUsername();

        $check = $this->stepUpVerify(
            $response, $auth, $mfa, $lockout, $username, $request->getClientIp(),
            (string) $request->input('password', ''),
            (string) $request->input('code', ''),
            (bool) $request->input('use_backup', false)
        );
        if (! $check['ok']) return;

        // Audit BEFORE regeneration (which would itself change the count)
        // so the "remaining" count reflects what the user actually saw.
        $this->auditBackupCodeIfUsed($check, $audit, $this->logger, $auth, $username, $request->getClientIp());

        $codes = $mfa->regenerateBackupCodes($username);
        $this->logger->log("MFA backup codes regenerated for {$username}");

        return $response->json(['backup_codes' => $codes]);
    }

    public function updateEmail(Request $request, Response $response, AuthInterface $auth)
    {
        if (! $auth instanceof MfaCapableInterface) {
            return $response->json('Not supported', 501);
        }
        $user = $auth->user();
        $username = $user->getUsername();
        $email = $request->input('email', null);

        if (! $this->emailValid($email)) {
            return $response->json(['email' => 'Invalid email address'], 422);
        }

        $normalized = ($email === '' || $email === null) ? null : strtolower(trim($email));

        try {
            $auth->setEmail($username, $normalized);
        } catch (\Exception $e) {
            return $response->json(['email' => $e->getMessage()], 422);
        }

        // email is part of the session-hash tamper check; refresh so we
        // don't log ourselves out on the next request.
        $auth->establishSessionFor($username);

        // Echo back the normalized value; no second read of users.json.
        return $response->json(['email' => $normalized]);
    }

    protected function mfaUnsupported(AuthInterface $auth, MfaService $mfa): bool
    {
        return ! ($auth instanceof MfaCapableInterface) || ! $mfa->isSupported();
    }

    protected function emailValid($email): bool
    {
        if ($email === null || $email === '') return true;
        return (bool) filter_var($email, FILTER_VALIDATE_EMAIL);
    }
}
