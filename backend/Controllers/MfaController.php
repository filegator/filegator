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
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\MfaCapableInterface;
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Mfa\MfaService;

class MfaController
{
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
            'email' => $auth->getEmail($username),
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
        $enrollment = $mfa->beginEnrollment($user->getUsername());

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

        $this->logger->log("MFA enrolled for {$user->getUsername()}");
        return $response->json(['backup_codes' => $codes]);
    }

    public function disable(Request $request, Response $response, AuthInterface $auth, MfaService $mfa, Config $config)
    {
        if ($this->mfaUnsupported($auth, $mfa)) {
            return $response->json('MFA not supported', 501);
        }

        $user = $auth->user();
        $username = $user->getUsername();

        $password = (string) $request->input('password', '');
        $code = (string) $request->input('code', '');
        $useBackup = (bool) $request->input('use_backup', false);

        if ($password === '' || $code === '') {
            return $response->json('Password and current MFA code required', 422);
        }

        if ($mfa->isRequiredForUser($username, $user->getRole())) {
            return $response->json('MFA is required for your role and cannot be disabled', 422);
        }

        if (! $auth->verifyPasswordOnly($username, $password)) {
            return $response->json(['password' => 'Wrong password'], 422);
        }

        $ok = $useBackup ? $mfa->consumeBackupCode($username, $code) : $mfa->verifyTotp($username, $code);
        if (! $ok) {
            return $response->json(['code' => 'Invalid code'], 422);
        }

        $mfa->disable($username);
        $this->logger->log("MFA disabled for {$username}");

        return $response->json('ok');
    }

    public function regenerateBackupCodes(Request $request, Response $response, AuthInterface $auth, MfaService $mfa)
    {
        if ($this->mfaUnsupported($auth, $mfa)) {
            return $response->json('MFA not supported', 501);
        }

        $user = $auth->user();
        $username = $user->getUsername();

        $password = (string) $request->input('password', '');
        $code = (string) $request->input('code', '');
        $useBackup = (bool) $request->input('use_backup', false);

        if ($password === '' || $code === '') {
            return $response->json('Password and current MFA code required', 422);
        }
        if (! $auth->verifyPasswordOnly($username, $password)) {
            return $response->json(['password' => 'Wrong password'], 422);
        }
        $ok = $useBackup ? $mfa->consumeBackupCode($username, $code) : $mfa->verifyTotp($username, $code);
        if (! $ok) {
            return $response->json(['code' => 'Invalid code'], 422);
        }

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

        if ($email !== null && $email !== '' && ! filter_var($email, FILTER_VALIDATE_EMAIL)) {
            return $response->json(['email' => 'Invalid email address'], 422);
        }

        try {
            $auth->setEmail($username, $email === '' ? null : $email);
        } catch (\Exception $e) {
            return $response->json(['email' => $e->getMessage()], 422);
        }

        return $response->json(['email' => $auth->getEmail($username)]);
    }

    protected function mfaUnsupported(AuthInterface $auth, MfaService $mfa): bool
    {
        return ! ($auth instanceof MfaCapableInterface) || ! $mfa->isSupported();
    }
}
