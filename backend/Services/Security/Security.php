<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Security;

use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Services\Service;
use Filegator\Services\Logger\LoggerInterface;
use Symfony\Component\Security\Csrf\CsrfToken;
use Symfony\Component\Security\Csrf\CsrfTokenManager;

/**
 * @codeCoverageIgnore
 */
class Security implements Service
{
    protected $request;

    protected $response;

    protected $logger;

    public function __construct(Request $request, Response $response, LoggerInterface $logger)
    {
        $this->request = $request;
        $this->response = $response;
        $this->logger = $logger;
    }

    public function init(array $config = [])
    {
        if ($config['csrf_protection']) {

            $key = isset($config['csrf_key']) ? $config['csrf_key'] : 'protection';

            $http_method = $this->request->getMethod();
            $csrfManager = new CsrfTokenManager();

            if (in_array($http_method, ['GET', 'HEAD', 'OPTIONS'])) {
                $this->response->headers->set('X-CSRF-Token', $csrfManager->getToken($key));
            } else {
                $token = new CsrfToken($key, $this->request->headers->get('X-CSRF-Token'));

                if (! $csrfManager->isTokenValid($token)) {
                    $this->logger->log("Csrf token not valid");
                    die;
                }
            }
        }

        if (! empty($config['ip_whitelist'])) $config['ip_allowlist'] = $config['ip_whitelist']; // deprecated, compatibility

        if (! empty($config['ip_allowlist'])) {
            $pass = false;
            foreach ($config['ip_allowlist'] as $ip) {
                if ($this->request->getClientIp() == $ip) {
                    $pass = true;
                }
            }
            if (! $pass) {
                $this->response->setStatusCode(403);
                $this->response->send();
                $this->logger->log("Forbidden - IP not found in allowlist ".$this->request->getClientIp());
                die;
            }
        }

        if (! empty($config['ip_blacklist'])) $config['ip_denylist'] = $config['ip_blacklist']; // deprecated, compatibility

        if (! empty($config['ip_denylist'])) {
            $pass = true;
            foreach ($config['ip_denylist'] as $ip) {
                if ($this->request->getClientIp() == $ip) {
                    $pass = false;
                }
            }
            if (! $pass) {
                $this->response->setStatusCode(403);
                $this->response->send();
                $this->logger->log("Forbidden - IP matched against denylist ".$this->request->getClientIp());
                die;
            }
        }


        if (empty($config['allow_insecure_overlays']) || !$config['allow_insecure_overlays']) {
            $this->response->headers->set('X-Frame-Options', 'sameorigin');
            $this->response->headers->set('Content-Security-Policy', 'frame-ancestors \'self\'');
        }
    }
}
