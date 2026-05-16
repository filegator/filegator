<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Services\Mfa\BackupCodeGenerator;
use Filegator\Utils\PasswordHash;
use OTPHP\TOTP;
use PHPUnit\Framework\TestCase;

/**
 * @internal
 */
class MfaServiceTest extends TestCase
{
    use PasswordHash;

    public function testBackupCodesAreUniqueAndCorrectShape()
    {
        $codes = BackupCodeGenerator::generate(10, 10);
        $this->assertCount(10, $codes);
        $this->assertCount(10, array_unique($codes));
        foreach ($codes as $c) {
            // Format: XXXXX-XXXXX
            $this->assertRegExp('/^[A-Z2-9]{5}-[A-Z2-9]{5}$/', $c);
        }
    }

    public function testHashedCodesVerifyWithBcrypt()
    {
        $plain = ['ABCDE-12345', 'WXYZH-98765'];
        $hashes = BackupCodeGenerator::hashAll($plain);
        $this->assertCount(2, $hashes);
        foreach ($hashes as $h) {
            $this->assertStringStartsWith('$2y$', $h);
        }
        $this->assertTrue($this->verifyPassword(BackupCodeGenerator::normalize('ABCDE-12345'), $hashes[0]));
        $this->assertFalse($this->verifyPassword('wrong', $hashes[0]));
    }

    public function testNormalizeStripsSeparators()
    {
        $this->assertSame('ABCDE12345', BackupCodeGenerator::normalize('abcde-12345'));
        $this->assertSame('ABCDE12345', BackupCodeGenerator::normalize(' abcde 12345 '));
    }

    public function testTotpVerifiesAtCurrentWindow()
    {
        $totp = TOTP::create();
        $code = $totp->now();
        $this->assertTrue($totp->verify($code, null, 1));
    }
}
