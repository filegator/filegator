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
use Symfony\Component\Security\Csrf\TokenGenerator\UriSafeTokenGenerator;
use Symfony\Component\Security\Csrf\TokenStorage\SessionTokenStorage;

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
            // Back the CSRF manager with the request's SessionInterface instead of
            // raw $_SESSION. CsrfTokenManager's default storage calls session_start()
            // natively, which conflicts with the framework's Session abstraction
            // (and crashes outright when headers have already been sent — which is
            // what happens under PHPUnit). Storage location is unchanged in
            // production because the underlying NativeFileSessionHandler is the same.
            $csrfManager = new CsrfTokenManager(
                new UriSafeTokenGenerator(),
                new SessionTokenStorage($this->request->getSession())
            );

            $exempt_paths = isset($config['csrf_exempt_paths']) && is_array($config['csrf_exempt_paths'])
                ? $config['csrf_exempt_paths']
                : ['/password/forgot', '/password/reset/validate', '/password/reset'];

            $route_id = (string) $this->request->query->get('r', '');
            $is_exempt = in_array($route_id, $exempt_paths, true);

            if (in_array($http_method, ['GET', 'HEAD', 'OPTIONS'])) {
                $this->response->headers->set('X-CSRF-Token', $csrfManager->getToken($key));
            } elseif (! $is_exempt) {
                $token = new CsrfToken($key, $this->request->headers->get('X-CSRF-Token'));

                if (! $csrfManager->isTokenValid($token)) {
                    $this->logger->log("Csrf token not valid");
                    $this->response->setStatusCode(403);
                    $this->response->setContent(json_encode(['data' => 'CSRF token invalid']));
                    $this->response->headers->set('Content-Type', 'application/json');
                    if (defined('APP_ENV') && APP_ENV === 'test') {
                        // Test harness will catch this in sendRequest() so it can
                        // read $this->response without PHPUnit aborting on exit.
                        throw new CsrfFailedException('CSRF token not valid');
                    }
                    $this->response->send();
                    exit;
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
