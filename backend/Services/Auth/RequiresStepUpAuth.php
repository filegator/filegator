<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Auth;

use Filegator\Kernel\Response;
use Filegator\Services\Audit\AuditMailer;
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Mfa\MfaService;

/**
 * Shared step-up gate for sensitive controller actions that should not be
 * authorised by session cookie alone (admin user CRUD, MFA disable, MFA
 * reset of another user, backup-code regeneration). Re-verifies the
 * acting user's password + current TOTP (or a backup code). Honours the
 * same per-IP + per-username brute-force lockouts as the MFA login step,
 * so a stolen admin session cannot just guess codes at the step-up
 * endpoint either.
 *
 * Degrades to a no-op when the acting user has no MFA enrolled: there
 * is no second factor to prove, and forcing password-only reauth on
 * every admin write would be a meaningful UX regression for deployments
 * that have deliberately not enabled admin MFA. The security model
 * assumes deploys that care about session-theft enable MFA for the
 * roles that can do destructive things — which `mfa_required_for_admins`
 * accomplishes for the admin role by default.
 *
 * On success returns ['ok' => true, 'used_backup' => bool, 'username' => string]
 * plus 'backup_codes_remaining' (int) when used_backup is true. On failure
 * mutates $response to 422 or 429 and returns ['ok' => false]; callers
 * must return immediately when ok is false.
 */
trait RequiresStepUpAuth
{
    /**
     * Caller extracts $password / $code / $useBackup from whichever request
     * fields it owns (admin endpoints use `stepup_password` / `stepup_code`
     * to avoid colliding with their existing `password` field; MfaController
     * uses `password` / `code` for backward compatibility).
     */
    protected function stepUpVerify(
        Response $response,
        AuthInterface $auth,
        MfaService $mfa,
        MfaLockout $lockout,
        string $username,
        string $ip,
        string $password,
        string $code,
        bool $useBackup
    ): array {
        if (! $auth instanceof MfaCapableInterface) {
            // Adapter has no MFA primitive — nothing to step up against.
            return ['ok' => true, 'used_backup' => false];
        }

        $state = $auth->getMfaState($username);
        if (empty($state['enabled'])) {
            // No second factor enrolled — see class docblock.
            return ['ok' => true, 'used_backup' => false];
        }

        if ($lockout->isLocked($ip, $username)) {
            $response->json('Not Allowed', 429);
            return ['ok' => false, 'used_backup' => false];
        }

        if ($password === '' || $code === '') {
            $response->json('Password and current MFA code required', 422);
            return ['ok' => false, 'used_backup' => false];
        }

        if (! $auth->verifyPasswordOnly($username, $password)) {
            $lockout->recordFailure($ip, $username);
            $response->json(['password' => 'Wrong password'], 422);
            return ['ok' => false, 'used_backup' => false];
        }

        $ok = $useBackup
            ? $mfa->consumeBackupCode($username, $code)
            : $mfa->verifyTotp($username, $code);

        if (! $ok) {
            $lockout->recordFailure($ip, $username);
            $response->json(['code' => 'Invalid code'], 422);
            return ['ok' => false, 'used_backup' => false];
        }

        $lockout->clearForUsername($username);

        $result = ['ok' => true, 'used_backup' => $useBackup, 'username' => $username];
        if ($useBackup) {
            // post-consume count: pre-consume count - 1 (consumeBackupCode
            // is single-use on success). Avoids a second getMfaState read.
            $preCount = (int) ($state['backup_codes_remaining'] ?? 0);
            $result['backup_codes_remaining'] = max(0, $preCount - 1);
        }
        return $result;
    }

    /**
     * Fire the backup-code-consumed audit + threshold warning log when
     * the step-up trait reports a backup code was used. No-op otherwise.
     * Reads `username` and `backup_codes_remaining` directly from the
     * $check payload — stepUpVerify pre-populates both on the
     * used_backup path, so no extra getMfaState read here.
     */
    protected function auditBackupCodeIfUsed(
        array $check,
        AuditMailer $audit,
        LoggerInterface $logger,
        string $ip
    ): void {
        if (empty($check['used_backup'])) return;

        $username = (string) ($check['username'] ?? '');
        $remaining = (int) ($check['backup_codes_remaining'] ?? 0);
        if ($username === '') return;

        $audit->mfaBackupCodeConsumed($username, $ip, $remaining);
        if ($remaining <= 2) {
            $logger->log("WARNING: {$username} has {$remaining} MFA backup codes remaining after consume from {$ip}");
        }
    }
}
