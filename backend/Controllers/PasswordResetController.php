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
use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Mailer\MailerInterface;
use Filegator\Services\PasswordReset\PasswordResetService;
use Filegator\Services\Session\SessionStorageInterface;

class PasswordResetController
{
    const GENERIC_OK = 'If that email matches an account, a reset link has been sent.';

    protected $logger;

    public function __construct(LoggerInterface $logger)
    {
        $this->logger = $logger;
    }

    public function request(Request $request, Response $response, PasswordResetService $service, MailerInterface $mailer)
    {
        $email = (string) $request->input('email', '');
        $ip = $request->getClientIp();

        if (! filter_var($email, FILTER_VALIDATE_EMAIL)) {
            // Same generic response — do not signal validation.
            return $response->json(['message' => self::GENERIC_OK]);
        }

        if (! $service->rateLimitIp($ip)) {
            $this->logger->log('Password reset rate-limited (ip='.$ip.')');
            return $response->json('Too many requests', 429);
        }
        if (! $service->rateLimitEmail($email)) {
            $this->logger->log('Password reset rate-limited (email)');
            return $response->json(['message' => self::GENERIC_OK]);
        }

        if (! $mailer->isConfigured() || ! $service->isConfigured()) {
            $this->logger->log('Password reset requested but feature not fully configured (mailer or reset_url_base)');
            return $response->json(['message' => self::GENERIC_OK]);
        }

        $service->requestReset($email, $ip);

        return $response->json(['message' => self::GENERIC_OK]);
    }

    public function validateToken(Request $request, Response $response, PasswordResetService $service)
    {
        $token = (string) $request->input('token', '');
        if ($token === '') {
            $token = (string) ($request->query->get('token') ?? '');
        }
        if ($token === '') {
            return $response->json(['valid' => false]);
        }
        $row = $service->validateToken($token);
        return $response->json(['valid' => $row !== null]);
    }

    public function confirm(Request $request, Response $response, PasswordResetService $service, SessionStorageInterface $session)
    {
        $token = (string) $request->input('token', '');
        $newPassword = (string) $request->input('new_password', '');

        if ($token === '' || $newPassword === '') {
            return $response->json(['message' => 'Token and new password required'], 422);
        }
        if (strlen($newPassword) < 8) {
            return $response->json(['new_password' => 'Password must be at least 8 characters'], 422);
        }

        $ok = $service->confirmReset($token, $newPassword);
        if (! $ok) {
            return $response->json(['message' => 'Invalid or expired token'], 422);
        }

        $session->migrate(true);
        return $response->json(['message' => 'Password updated']);
    }
}
