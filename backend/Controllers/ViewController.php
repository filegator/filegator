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
use Filegator\Kernel\Response;
use Filegator\Services\Mailer\MailerInterface;
use Filegator\Services\PasswordReset\PasswordResetService;
use Filegator\Services\View\ViewInterface;

class ViewController
{
    public function index(Response $response, ViewInterface $view)
    {
        return $response->html($view->getIndexPage());
    }

    public function getFrontendConfig(Response $response, Config $config, MailerInterface $mailer, PasswordResetService $reset)
    {
        $frontend = (array) $config->get('frontend_config', []);
        // Both mailer AND reset_url_base must be configured before the feature is exposed.
        // reset_url_base is required to prevent host-header injection in reset links.
        $frontend['password_reset_enabled'] = $mailer->isConfigured() && $reset->isConfigured();
        $frontend['password_reset_token_ttl'] = (int) $config->get('password_reset_token_ttl', 3600);
        $frontend['mfa_required_for_admins'] = (bool) $config->get('mfa_required_for_admins', true);
        return $response->json($frontend);
    }
}
