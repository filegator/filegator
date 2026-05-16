<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Feature;

use Filegator\Services\Auth\AuthInterface;
use Tests\Fakes\InMemoryMailer;
use Tests\TestCase;

/**
 * @internal
 */
class PasswordResetTest extends TestCase
{
    protected function setEmail(string $username, string $email): void
    {
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $auth->setEmail($username, $email);
    }

    protected function tokenFromLastMessage(): ?string
    {
        $msg = InMemoryMailer::last();
        if (! $msg) return null;
        if (preg_match('/token=([a-f0-9]{64})/', $msg['text'], $m)) {
            return $m[1];
        }
        return null;
    }

    public function testRequestResetForUnknownEmailReturnsGenericOk()
    {
        $this->sendRequest('POST', '/password/forgot', ['email' => 'nobody@example.com']);
        $this->assertOk();
        $this->assertNull(InMemoryMailer::last());
    }

    public function testRequestResetForKnownEmailSendsLink()
    {
        $this->setEmail('john@example.com', 'john@reset.test');

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $this->assertOk();

        $msg = InMemoryMailer::last();
        $this->assertNotNull($msg);
        $this->assertSame('john@reset.test', $msg['to']);
        $this->assertNotNull($this->tokenFromLastMessage());
    }

    public function testTokenIsHashedOnDisk()
    {
        $this->setEmail('john@example.com', 'john@reset.test');
        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);

        $token = $this->tokenFromLastMessage();
        $this->assertNotNull($token);

        $file = TEST_TMP_PATH.'password_resets.json';
        $this->assertFileExists($file);
        $raw = file_get_contents($file);
        $this->assertStringNotContainsString($token, $raw);
        $this->assertStringContainsString(hash('sha256', $token), $raw);
    }

    public function testValidateTokenEndpoint()
    {
        $this->setEmail('john@example.com', 'john@reset.test');
        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $token = $this->tokenFromLastMessage();

        $this->sendRequest('POST', '/password/reset/validate', ['token' => $token]);
        $this->assertOk();
        $this->assertResponseJsonHas(['data' => ['valid' => true]]);

        $this->sendRequest('POST', '/password/reset/validate', ['token' => 'not-a-real-token']);
        $this->assertOk();
        $this->assertResponseJsonHas(['data' => ['valid' => false]]);
    }

    public function testConfirmResetRotatesPasswordAndIsSingleUse()
    {
        $this->setEmail('john@example.com', 'john@reset.test');
        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $token = $this->tokenFromLastMessage();

        $this->sendRequest('POST', '/password/reset', [
            'token' => $token,
            'new_password' => 'newSecret123',
        ]);
        $this->assertOk();

        // Old password no longer works.
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'john123',
        ]);
        $this->assertUnprocessable();

        // New one does.
        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'newSecret123',
        ]);
        $this->assertOk();

        // Token cannot be reused.
        $this->sendRequest('POST', '/password/reset', [
            'token' => $token,
            'new_password' => 'anotherOne123',
        ]);
        $this->assertUnprocessable();
    }

    public function testNewTokenInvalidatesPreviousUnusedTokens()
    {
        $this->setEmail('john@example.com', 'john@reset.test');

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $token1 = $this->tokenFromLastMessage();

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $token2 = $this->tokenFromLastMessage();

        $this->assertNotEquals($token1, $token2);

        // Older token is now invalid.
        $this->sendRequest('POST', '/password/reset/validate', ['token' => $token1]);
        $this->assertResponseJsonHas(['data' => ['valid' => false]]);

        // Newer one still works.
        $this->sendRequest('POST', '/password/reset/validate', ['token' => $token2]);
        $this->assertResponseJsonHas(['data' => ['valid' => true]]);
    }

    public function testPerIpRateLimit()
    {
        $this->overrideConfig([
            'password_reset_max_per_hour_per_ip' => 2,
            'password_reset_max_per_day_per_email' => 100,
        ]);

        $this->sendRequest('POST', '/password/forgot', ['email' => 'a@example.com'], [], ['REMOTE_ADDR' => '5.5.5.5']);
        $this->assertOk();
        $this->sendRequest('POST', '/password/forgot', ['email' => 'b@example.com'], [], ['REMOTE_ADDR' => '5.5.5.5']);
        $this->assertOk();
        $this->sendRequest('POST', '/password/forgot', ['email' => 'c@example.com'], [], ['REMOTE_ADDR' => '5.5.5.5']);
        $this->assertStatus(429);
    }

    public function testForgotPasswordIsCsrfExempt()
    {
        // No CSRF token is sent. The route should still succeed.
        $this->sendRequest('POST', '/password/forgot', ['email' => 'nobody@example.com']);
        $this->assertOk();
    }

    public function testResetWhenMailerNotConfiguredStillReturnsOk()
    {
        InMemoryMailer::$configured = false;
        $this->setEmail('john@example.com', 'john@reset.test');

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $this->assertOk();
        $this->assertNull(InMemoryMailer::last());
    }

    public function testResetDoesNotBypassMfa()
    {
        $this->setEmail('john@example.com', 'john@reset.test');

        // Enroll MFA on this user, then reset their password.
        $app = $this->sendRequest('GET', '/getuser');
        $auth = $app->resolve(AuthInterface::class);
        $secret = \OTPHP\TOTP::create()->getSecret();
        $auth->setMfaSecret('john@example.com', $secret);
        $auth->enableMfa('john@example.com', \Filegator\Services\Mfa\BackupCodeGenerator::hashAll(['AAAAA-11111']));

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $token = $this->tokenFromLastMessage();
        $this->sendRequest('POST', '/password/reset', [
            'token' => $token,
            'new_password' => 'newSecret123',
        ]);
        $this->assertOk();

        $this->sendRequest('POST', '/login', [
            'username' => 'john@example.com',
            'password' => 'newSecret123',
        ]);
        $this->assertOk();
        $data = $this->decodeResponseJson()['data'];
        $this->assertTrue($data['mfa_required'] ?? false);
    }

    public function testShortPasswordRejected()
    {
        $this->setEmail('john@example.com', 'john@reset.test');
        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $token = $this->tokenFromLastMessage();

        $this->sendRequest('POST', '/password/reset', [
            'token' => $token,
            'new_password' => 'short',
        ]);
        $this->assertUnprocessable();
    }
}
