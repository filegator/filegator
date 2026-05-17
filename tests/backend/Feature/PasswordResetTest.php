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

    public function testBrandingValuesRenderIntoEmail()
    {
        $this->overrideConfig([
            'services' => [
                'Filegator\\Services\\PasswordReset\\PasswordResetService' => [
                    'config' => [
                        'branding' => [
                            'app_label'     => 'Acme Tax Portal',
                            'logo_url'      => 'https://cdn.example.com/acme.png',
                            'primary_color' => '#bada55',
                            'support_email' => 'help@acme.test',
                        ],
                    ],
                ],
            ],
        ]);
        $this->setEmail('john@example.com', 'john@reset.test');

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $this->assertOk();

        $msg = InMemoryMailer::last();
        $this->assertNotNull($msg);
        // Branded label shown in both text + HTML bodies, logo + brand color
        // in HTML, support email in footer.
        $this->assertStringContainsString('Acme Tax Portal', $msg['text']);
        $this->assertStringContainsString('Acme Tax Portal', $msg['html']);
        $this->assertStringContainsString('https://cdn.example.com/acme.png', $msg['html']);
        $this->assertStringContainsString('#bada55', $msg['html']);
        $this->assertStringContainsString('help@acme.test', $msg['html']);
        $this->assertStringContainsString('help@acme.test', $msg['text']);
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

    public function testPerEmailRateLimit()
    {
        // Tighten the per-email limit and loosen per-IP so the IP throttle never fires first.
        $this->overrideConfig([
            'password_reset_max_per_hour_per_ip' => 100,
            'password_reset_max_per_day_per_email' => 2,
        ]);
        $this->setEmail('john@example.com', 'john@reset.test');

        // Three different IPs all targeting the same email. The third should
        // bypass the per-IP throttle but trip the per-email throttle, which
        // returns generic 200 (no mail sent) — NOT 429 — to preserve the
        // anti-enumeration property.
        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test'], [], ['REMOTE_ADDR' => '1.1.1.1']);
        $this->assertOk();
        $this->assertNotNull(InMemoryMailer::last(), 'first attempt should send');

        $sentSoFar = count(InMemoryMailer::$messages);
        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test'], [], ['REMOTE_ADDR' => '2.2.2.2']);
        $this->assertOk();
        $this->assertCount($sentSoFar + 1, InMemoryMailer::$messages, 'second attempt should send');

        $sentBefore = count(InMemoryMailer::$messages);
        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test'], [], ['REMOTE_ADDR' => '3.3.3.3']);
        $this->assertOk(); // generic OK, NOT 429
        $this->assertCount($sentBefore, InMemoryMailer::$messages, 'third attempt over per-email limit must not send mail');
    }

    public function testRateLimitAtMaxStillAllowsLastRequest()
    {
        // Regression for off-by-one on the per-IP threshold: with max=2, the
        // 2nd request must succeed and the 3rd must fail.
        $this->overrideConfig([
            'password_reset_max_per_hour_per_ip' => 2,
            'password_reset_max_per_day_per_email' => 100,
        ]);
        $ip = ['REMOTE_ADDR' => '7.7.7.7'];

        $this->sendRequest('POST', '/password/forgot', ['email' => 'a@example.com'], [], $ip);
        $this->assertOk();
        // Second request should still pass (==max, not > max).
        $this->sendRequest('POST', '/password/forgot', ['email' => 'b@example.com'], [], $ip);
        $this->assertOk();
        // Third must be blocked.
        $this->sendRequest('POST', '/password/forgot', ['email' => 'c@example.com'], [], $ip);
        $this->assertStatus(429);
    }

    public function testForgotPasswordIsCsrfExempt()
    {
        // Enable CSRF for this test only. The default test config has it off
        // so other suites do not have to plumb tokens through every helper.
        $this->overrideConfig(['services' => ['Filegator\\Services\\Security\\Security' => ['config' => ['csrf_protection' => true]]]]);

        // No X-CSRF-Token header — but /password/forgot is on the exempt list,
        // so it should still succeed instead of returning 403.
        $this->sendRequest('POST', '/password/forgot', ['email' => 'nobody@example.com']);
        $this->assertOk();
    }

    public function testNonExemptPostIsBlockedWithoutCsrfToken()
    {
        // With CSRF protection on and no token in the request, a non-exempt
        // POST must be refused with 403. This is the negative side of the
        // CSRF-exemption contract — without this assertion the exempt test
        // above could pass vacuously even if the middleware never ran.
        $this->overrideConfig(['services' => ['Filegator\\Services\\Security\\Security' => ['config' => ['csrf_protection' => true]]]]);

        $this->sendRequest('POST', '/login', ['username' => 'john@example.com', 'password' => 'john123']);
        $this->assertStatus(403);
    }

    public function testResetLinkIgnoresAttackerSuppliedHostHeader()
    {
        $this->setEmail('john@example.com', 'john@reset.test');

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test'], [], [
            'HTTP_HOST' => 'attacker.example.com',
            'HTTP_X_FORWARDED_HOST' => 'attacker.example.com',
        ]);
        $this->assertOk();

        $msg = InMemoryMailer::last();
        $this->assertNotNull($msg);
        // Link must use the configured reset_url_base, never the attacker's Host.
        $this->assertStringContainsString('https://files.example.com/', $msg['text']);
        $this->assertStringNotContainsString('attacker.example.com', $msg['text']);
        $this->assertStringNotContainsString('attacker.example.com', (string) $msg['html']);
    }

    public function testResetWhenResetUrlBaseNotConfiguredReturnsGenericOk()
    {
        $this->overrideConfig([
            'services' => [
                'Filegator\Services\PasswordReset\PasswordResetService' => [
                    'config' => ['reset_url_base' => null],
                ],
            ],
        ]);
        $this->setEmail('john@example.com', 'john@reset.test');

        $this->sendRequest('POST', '/password/forgot', ['email' => 'john@reset.test']);
        $this->assertOk();
        $this->assertNull(InMemoryMailer::last());
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
