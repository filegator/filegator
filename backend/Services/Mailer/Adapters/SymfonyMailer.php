<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Mailer\Adapters;

use Filegator\Services\Logger\LoggerInterface;
use Filegator\Services\Mailer\MailerInterface;
use Filegator\Services\Service;
use Symfony\Component\Mailer\Mailer;
use Symfony\Component\Mailer\Transport;
use Symfony\Component\Mime\Address;
use Symfony\Component\Mime\Email;

/**
 * @codeCoverageIgnore
 */
class SymfonyMailer implements Service, MailerInterface
{
    const DEFAULT_TIMEOUT_SECONDS = 5;

    protected $logger;

    protected $config = [];

    /** Holds the previous default_socket_timeout for the duration of send(). */
    protected $previousSocketTimeout = null;

    public function __construct(LoggerInterface $logger)
    {
        $this->logger = $logger;
    }

    public function init(array $config = [])
    {
        $this->config = $config;
        // Normalise the DSN once: if the operator forgot to set ?timeout=N,
        // append our configured default so PHP's default_socket_timeout (60s)
        // never gets a chance to hang a PHP-FPM worker on unreachable SMTP.
        if (! empty($config['dsn']) && is_string($config['dsn'])) {
            $this->config['dsn'] = $this->ensureTimeoutInDsn($config['dsn']);
        }
    }

    public function isConfigured(): bool
    {
        $dsn = $this->config['dsn'] ?? '';
        if (! is_string($dsn) || $dsn === '') return false;
        if (strpos($dsn, 'null://') === 0) return false;
        return ! empty($this->config['from_email']);
    }

    public function send(string $to, string $subject, string $textBody, ?string $htmlBody = null): bool
    {
        $dsn = $this->config['dsn'] ?? '';
        if (! is_string($dsn) || $dsn === '') {
            $this->logger->log('Mailer not configured; dropping message to '.$to);
            return false;
        }

        // Defence in depth: force PHP's default_socket_timeout to our cap for
        // the duration of the send so non-DSN-aware transports cannot stall.
        $this->clampSocketTimeout();

        try {
            $transport = Transport::fromDsn($dsn);
            $mailer = new Mailer($transport);

            $email = (new Email())
                ->from(new Address(
                    (string) ($this->config['from_email'] ?? 'no-reply@localhost'),
                    (string) ($this->config['from_name'] ?? '')
                ))
                ->to($to)
                ->subject($subject)
                ->text($textBody);

            if ($htmlBody !== null) {
                $email->html($htmlBody);
            }

            $mailer->send($email);
            return true;
        } catch (\Throwable $e) {
            $this->logger->log('Mailer send failed: '.$e->getMessage());
            return false;
        } finally {
            $this->restoreSocketTimeout();
        }
    }

    protected function timeoutSeconds(): int
    {
        $v = $this->config['timeout'] ?? self::DEFAULT_TIMEOUT_SECONDS;
        $v = (int) $v;
        return $v > 0 ? $v : self::DEFAULT_TIMEOUT_SECONDS;
    }

    protected function ensureTimeoutInDsn(string $dsn): string
    {
        // Skip the null transport — there is no socket to time out on.
        if (strpos($dsn, 'null://') === 0) return $dsn;

        // Parse query string portion of the DSN, if any.
        $parts = parse_url($dsn);
        if ($parts === false) return $dsn;

        parse_str($parts['query'] ?? '', $query);
        if (isset($query['timeout'])) return $dsn; // operator already chose a value

        $query['timeout'] = $this->timeoutSeconds();
        $rebuilt = (isset($parts['scheme']) ? $parts['scheme'].'://' : '')
            .(isset($parts['user']) ? rawurlencode($parts['user']).(isset($parts['pass']) ? ':'.rawurlencode($parts['pass']) : '').'@' : '')
            .($parts['host'] ?? '')
            .(isset($parts['port']) ? ':'.$parts['port'] : '')
            .($parts['path'] ?? '')
            .'?'.http_build_query($query);

        return $rebuilt;
    }

    protected function clampSocketTimeout(): void
    {
        $this->previousSocketTimeout = ini_get('default_socket_timeout');
        ini_set('default_socket_timeout', (string) $this->timeoutSeconds());
    }

    protected function restoreSocketTimeout(): void
    {
        if ($this->previousSocketTimeout !== null && $this->previousSocketTimeout !== false) {
            ini_set('default_socket_timeout', (string) $this->previousSocketTimeout);
        }
        $this->previousSocketTimeout = null;
    }
}
