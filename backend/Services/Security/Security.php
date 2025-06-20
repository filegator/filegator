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
use Symfony\Component\Security\Csrf\TokenStorage\SessionTokenStorage;
use Symfony\Component\Security\Csrf\TokenGenerator\UriSafeTokenGenerator;

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
        // 1. Handle CSRF protection
        if (!empty($config['csrf_protection'])) {
            $key = $config['csrf_key'] ?? 'protection';

            $tokenStorage = new SessionTokenStorage($this->request->getSession());
            $tokenGenerator = new UriSafeTokenGenerator();
            $csrfManager = new CsrfTokenManager($tokenGenerator, $tokenStorage);

            $httpMethod = $this->request->getMethod();

            // 2. Restrict state-changing endpoints (e.g., /logout) in GET/HEAD/OPTIONS methods
            $stateChangingEndpoints = ['/logout', '/delete', '/reset'];
            $rValue = $this->request->get('r');  // Get r value from GET parameters

            if (in_array($httpMethod, ['GET', 'HEAD', 'OPTIONS']) &&
                $rValue !== null &&
                in_array($rValue, $stateChangingEndpoints)) {
                // Prohibit using GET method for state-changing operations
                $this->response->setStatusCode(405);
                $this->response->send('Method Not Allowed');
                $this->logger->log("GET method used for state-changing endpoint: " . $rValue);
                exit;
            }

            // 3. POST/PUT/DELETE requests must include a valid X-CSRF-Token
            if (!in_array($httpMethod, ['GET', 'HEAD', 'OPTIONS'])) {
                $token = new CsrfToken($key, $this->request->headers->get('X-CSRF-Token'));

                if (!$csrfManager->isTokenValid($token)) {
                    $this->response->setStatusCode(403);
                    $this->response->send('Invalid CSRF Token');
                    $this->logger->log("Invalid CSRF Token for request: " . ($rValue ?? 'unknown'));
                    exit;
                }
            } else {
                // Only generate token for frontend use
                $this->response->headers->set('X-CSRF-Token', $csrfManager->getToken($key));
            }
        }

        // 4. IP allowlist validation
        if (!empty($config['ip_whitelist'])) {
            $config['ip_allowlist'] = $config['ip_whitelist'];
        }
        if (!empty($config['ip_allowlist'])) {
            $allowed = false;
            foreach ($config['ip_allowlist'] as $ip) {
                if ($this->request->getClientIp() == $ip) {
                    $allowed = true;
                    break;
                }
            }
            if (!$allowed) {
                $this->response->setStatusCode(403);
                $this->response->send();
                $this->logger->log("Forbidden - IP not found in allowlist: " . $this->request->getClientIp());
                exit;
            }
        }

        // 5. IP denylist validation
        if (!empty($config['ip_blacklist'])) {
            $config['ip_denylist'] = $config['ip_blacklist'];
        }
        if (!empty($config['ip_denylist'])) {
            foreach ($config['ip_denylist'] as $ip) {
                if ($this->request->getClientIp() == $ip) {
                    $this->response->setStatusCode(403);
                    $this->response->send();
                    $this->logger->log("Forbidden - IP matched in denylist: " . $this->request->getClientIp());
                    exit;
                }
            }
        }

        // 6. Set security response headers
        $this->response->headers->set('X-Frame-Options', 'DENY');
        $this->response->headers->set('Content-Security-Policy', "frame-ancestors 'none'");
        $this->response->headers->set('X-Content-Type-Options', 'nosniff');
        $this->response->headers->set('X-Permitted-Cross-Domain-Policies', 'none');
    }
}
