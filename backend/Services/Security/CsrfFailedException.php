<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Security;

/**
 * Thrown by the Security middleware in test mode when CSRF validation fails.
 * The middleware has already set the response to 403 + JSON; production mode
 * sends-then-exits, but test mode throws this so the test harness can read
 * the response status without crashing PHPUnit.
 */
class CsrfFailedException extends \RuntimeException
{
}
