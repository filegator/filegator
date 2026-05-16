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
    protected $logger;

    protected $config = [];

    public function __construct(LoggerInterface $logger)
    {
        $this->logger = $logger;
    }

    public function init(array $config = [])
    {
        $this->config = $config;
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
        }
    }
}
