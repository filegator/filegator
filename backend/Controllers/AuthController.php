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
use Filegator\Services\Tmpfs\TmpfsInterface;
use Filegator\Services\Logger\LoggerInterface;
use Rakit\Validation\Validator;

class AuthController
{
    protected $logger;

    public function __construct(LoggerInterface $logger)
    {
        $this->logger = $logger;
    }

    public function login(Request $request, Response $response, AuthInterface $auth, TmpfsInterface $tmpfs, Config $config)
    {
        $username = $request->input('username');
        $password = $request->input('password');
        $ip = $request->getClientIp();

        $lockout_attempts = $config->get('lockout_attempts', 5);
        $lockout_timeout_seconds = $config->get('lockout_timeout', 15);
        $lockfile = md5($ip) . '.lock';

        // Check if IP is currently locked
        if ($tmpfs->exists($lockfile)) {
            $content = $tmpfs->read($lockfile);
            $data = json_decode($content, true);

            if (is_array($data)) {
                // If lockout period active, reject request
                if (isset($data['locked_until']) && time() < $data['locked_until']) {
                    $locked_until = $data['locked_until'];
                    $remaining = $locked_until - time();
                    return $response->json('Not Allowed', 429);
                } else {
                    // Reset lockout if timeout expired
                    $data['locked_until'] = null;
                    $tmpfs->write($lockfile, json_encode($data));
                }
            } else {
                // Initialize lock file with default values
                $tmpfs->write($lockfile, json_encode(['attempts' => 1, 'locked_until' => null]));
            }
        } else {
            // Initialize lock file for new IP
            $tmpfs->write($lockfile, json_encode(['attempts' => 1, 'locked_until' => null]));
        }

        // Attempt authentication
        $auth_result = $auth->authenticate($username, $password);

        if ($auth_result) {
            $this->logger->log("User $username logged in from IP $ip");

            // Clear lock file after successful login
            if ($tmpfs->exists($lockfile)) {
                $tmpfs->remove($lockfile);
            }

            return $response->json($auth->user());
        }

        // Handle failed login attempt
        $content = $tmpfs->exists($lockfile) ? $tmpfs->read($lockfile) : json_encode(['attempts' => 0, 'locked_until' => null]);
        $data = json_decode($content, true);

        // Update attempt count
        $attempts = 1;
        if (is_array($data) && isset($data['attempts'])) {
            $attempts = $data['attempts'] + 1;
        }

        // Check if lockout threshold reached
        $is_locked = $attempts >= $lockout_attempts;
        $locked_until = $is_locked ? time() + $lockout_timeout_seconds : null;

        // Update lock file with new state
        $tmpfs->write($lockfile, json_encode([
            'attempts' => $attempts,
            'locked_until' => $locked_until,
        ]));

        return $response->json('Login failed, please try again', 422);
    }

    public function logout(Response $response, AuthInterface $auth)
    {
        return $response->json($auth->forget());
    }

    public function getUser(Response $response, AuthInterface $auth)
    {
        $user = $auth->user() ?: $auth->getGuest();

        return $response->json($user);
    }

    public function changePassword(
        Request $request,
        Response $response,
        AuthInterface $auth,
        Validator $validator,
        TmpfsInterface $tmpfs,
        Config $config // Configuration object to read password policy settings
    ) {
        // Get password policy from config with fallback defaults
        $policy = $config->get('frontend_config.password_policy', [
            'min_length' => 8,
            'regex' => '/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[_\W]).+$/',
            'error_messages' => [
                'required' => 'This field is required',
                'min_length' => 'Password must be at least 8 characters',
                'regex' => 'Password must contain uppercase, lowercase, numbers and special characters',
            ],
            'enabled' => true,
        ]);
    
        // Set basic required field error message
        $validator->setMessage('required', $policy['error_messages']['required']);
    
        // Conditional validation rules based on password policy settings
        if ($policy['enabled']) {
            // Enable complex password validation
            $validator->setMessage('min', $policy['error_messages']['min_length']);
            $validator->setMessage('regex', $policy['error_messages']['regex']);
        }
    
        // Build validation rules array dynamically
        $rules = ['oldpassword' => 'required'];
        
        // Apply password complexity rules only if policy is enabled
        if ($policy['enabled']) {
            $rules['newpassword'] = "required|min:{$policy['min_length']}|regex:{$policy['regex']}";
        } else {
            $rules['newpassword'] = 'required'; // Simple password validation when policy disabled
        }
    
        // Execute validation
        $validation = $validator->validate($request->all(), $rules);
    
        // Return validation errors if any
        if ($validation->fails()) {
            $errors = $validation->errors();
            return $response->json($errors->firstOfAll(), 422);
        }
    
        // Verify current password against stored credentials
        if (!$auth->authenticate($auth->user()->getUsername(), $request->input('oldpassword'))) {
            return $response->json(['oldpassword' => 'Wrong password'], 422);
        }
    
        // Clear IP lock file after successful authentication
        $ip = $request->getClientIp();
        $lockfile = md5($ip) . '.lock';
    
        if ($tmpfs->exists($lockfile)) {
            $tmpfs->remove($lockfile);
        }
    
        // Update password in storage and return response
        return $response->json($auth->update(
            $auth->user()->getUsername(),
            $auth->user(),
            $request->input('newpassword')
        ));
    }

 
}
