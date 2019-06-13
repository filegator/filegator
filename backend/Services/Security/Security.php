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
use Symfony\Component\Security\Csrf\CsrfToken;
use Symfony\Component\Security\Csrf\CsrfTokenManager;

/**
 * @codeCoverageIgnore
 */
class Security implements Service
{
    protected $request;

    protected $response;

    public function __construct(Request $request, Response $response)
    {
        $this->request = $request;
        $this->response = $response;
    }

    public function init(array $config = [])
    {
        if ($config['csrf_protection']) {
            $http_method = $this->request->getMethod();
            $csrfManager = new CsrfTokenManager();

            if (in_array($http_method, ['GET', 'HEAD', 'OPTIONS'])) {
                $this->response->headers->set('X-CSRF-Token', $csrfManager->getToken('protection'));
            } else {
                $token = new CsrfToken('protection', $this->request->headers->get('X-CSRF-Token'));

                if (! $csrfManager->isTokenValid($token)) {
                    throw new \Exception('Csrf token not valid');
                }
            }
        }

        if (! empty($config['ip_whitelist'])) {
            $pass = false;
            foreach ($config['ip_whitelist'] as $ip) {
                if ($this->request->getClientIp() == $ip) {
                    $pass = true;
                }
            }
            if (! $pass) {
                $this->response->setStatusCode(403);
                $this->response->send();
                die;
            }
        }

        if (! empty($config['ip_blacklist'])) {
            $pass = true;
            foreach ($config['ip_blacklist'] as $ip) {
                if ($this->request->getClientIp() == $ip) {
                    $pass = false;
                }
            }
            if (! $pass) {
                $this->response->setStatusCode(403);
                $this->response->send();
                die;
            }
        }
    }
}
