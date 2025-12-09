<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Services\PathACL;

use Filegator\Services\PathACL\IpMatcher;
use Tests\TestCase;

/**
 * @internal
 */
class IpMatcherTest extends TestCase
{
    protected IpMatcher $matcher;

    protected function setUp(): void
    {
        parent::setUp();
        $this->matcher = new IpMatcher();
    }

    // ========== IP Validation Tests ==========

    public function testIsValidIpWithValidIPv4()
    {
        $this->assertTrue($this->matcher->isValidIp('192.168.1.1'));
        $this->assertTrue($this->matcher->isValidIp('10.0.0.1'));
        $this->assertTrue($this->matcher->isValidIp('172.16.0.1'));
        $this->assertTrue($this->matcher->isValidIp('8.8.8.8'));
    }

    public function testIsValidIpWithValidIPv6()
    {
        $this->assertTrue($this->matcher->isValidIp('2001:0db8:85a3:0000:0000:8a2e:0370:7334'));
        $this->assertTrue($this->matcher->isValidIp('2001:db8::1'));
        $this->assertTrue($this->matcher->isValidIp('::1'));
        $this->assertTrue($this->matcher->isValidIp('fe80::1'));
    }

    public function testIsValidIpWithInvalidIP()
    {
        $this->assertFalse($this->matcher->isValidIp('256.1.1.1'));
        $this->assertFalse($this->matcher->isValidIp('192.168.1'));
        $this->assertFalse($this->matcher->isValidIp('not-an-ip'));
        $this->assertFalse($this->matcher->isValidIp(''));
    }

    // ========== Pattern Validation Tests ==========

    public function testIsValidPatternWithWildcard()
    {
        $this->assertTrue($this->matcher->isValidPattern('*'));
    }

    public function testIsValidPatternWithSingleIP()
    {
        $this->assertTrue($this->matcher->isValidPattern('192.168.1.1'));
        $this->assertTrue($this->matcher->isValidPattern('2001:db8::1'));
    }

    public function testIsValidPatternWithCIDRv4()
    {
        $this->assertTrue($this->matcher->isValidPattern('192.168.1.0/24'));
        $this->assertTrue($this->matcher->isValidPattern('10.0.0.0/8'));
        $this->assertTrue($this->matcher->isValidPattern('172.16.0.0/12'));
        $this->assertTrue($this->matcher->isValidPattern('192.168.1.1/32'));
    }

    public function testIsValidPatternWithCIDRv6()
    {
        $this->assertTrue($this->matcher->isValidPattern('2001:db8::/32'));
        $this->assertTrue($this->matcher->isValidPattern('fe80::/10'));
        $this->assertTrue($this->matcher->isValidPattern('::1/128'));
    }

    public function testIsValidPatternWithInvalidCIDR()
    {
        $this->assertFalse($this->matcher->isValidPattern('192.168.1.0/33'));  // Invalid mask for IPv4
        $this->assertFalse($this->matcher->isValidPattern('192.168.1.0/-1'));  // Negative mask
        $this->assertFalse($this->matcher->isValidPattern('192.168.1.0/'));    // Missing mask
        $this->assertFalse($this->matcher->isValidPattern('2001:db8::/129')); // Invalid mask for IPv6
    }

    public function testIsValidPatternWithInvalidIP()
    {
        $this->assertFalse($this->matcher->isValidPattern('256.1.1.1'));
        $this->assertFalse($this->matcher->isValidPattern('not-an-ip'));
    }

    // ========== Allowlist Matching Tests ==========

    public function testMatchesAllowListWithEmptyList()
    {
        // Empty allowlist means allow all
        $this->assertTrue($this->matcher->matchesAllowList('192.168.1.50', []));
        $this->assertTrue($this->matcher->matchesAllowList('10.0.0.1', []));
    }

    public function testMatchesAllowListWithWildcard()
    {
        $allowlist = ['*'];
        $this->assertTrue($this->matcher->matchesAllowList('192.168.1.50', $allowlist));
        $this->assertTrue($this->matcher->matchesAllowList('10.0.0.1', $allowlist));
        $this->assertTrue($this->matcher->matchesAllowList('2001:db8::1', $allowlist));
    }

    public function testMatchesAllowListWithExactIPMatch()
    {
        $allowlist = ['192.168.1.50', '10.8.0.50'];

        $this->assertTrue($this->matcher->matchesAllowList('192.168.1.50', $allowlist));
        $this->assertTrue($this->matcher->matchesAllowList('10.8.0.50', $allowlist));
        $this->assertFalse($this->matcher->matchesAllowList('192.168.1.51', $allowlist));
        $this->assertFalse($this->matcher->matchesAllowList('172.16.0.1', $allowlist));
    }

    public function testMatchesAllowListWithCIDRNotation()
    {
        $allowlist = ['192.168.1.0/24'];

        $this->assertTrue($this->matcher->matchesAllowList('192.168.1.1', $allowlist));
        $this->assertTrue($this->matcher->matchesAllowList('192.168.1.50', $allowlist));
        $this->assertTrue($this->matcher->matchesAllowList('192.168.1.254', $allowlist));
        $this->assertFalse($this->matcher->matchesAllowList('192.168.2.1', $allowlist));
        $this->assertFalse($this->matcher->matchesAllowList('10.0.0.1', $allowlist));
    }

    public function testMatchesAllowListWithMultipleCIDRRanges()
    {
        $allowlist = ['192.168.1.0/24', '10.8.0.0/24'];

        $this->assertTrue($this->matcher->matchesAllowList('192.168.1.50', $allowlist));
        $this->assertTrue($this->matcher->matchesAllowList('10.8.0.50', $allowlist));
        $this->assertFalse($this->matcher->matchesAllowList('172.16.0.1', $allowlist));
    }

    public function testMatchesAllowListWithIPv6()
    {
        $allowlist = ['2001:db8::/32'];

        $this->assertTrue($this->matcher->matchesAllowList('2001:db8::1', $allowlist));
        $this->assertTrue($this->matcher->matchesAllowList('2001:db8:1::1', $allowlist));
        $this->assertFalse($this->matcher->matchesAllowList('2001:db9::1', $allowlist));
    }

    // ========== Denylist Matching Tests ==========

    public function testMatchesDenyListWithEmptyList()
    {
        // Empty denylist means deny nothing
        $this->assertFalse($this->matcher->matchesDenyList('192.168.1.50', []));
        $this->assertFalse($this->matcher->matchesDenyList('10.0.0.1', []));
    }

    public function testMatchesDenyListWithWildcard()
    {
        $denylist = ['*'];
        $this->assertTrue($this->matcher->matchesDenyList('192.168.1.50', $denylist));
        $this->assertTrue($this->matcher->matchesDenyList('10.0.0.1', $denylist));
    }

    public function testMatchesDenyListWithExactIPMatch()
    {
        $denylist = ['192.168.1.100', '10.0.0.1'];

        $this->assertTrue($this->matcher->matchesDenyList('192.168.1.100', $denylist));
        $this->assertTrue($this->matcher->matchesDenyList('10.0.0.1', $denylist));
        $this->assertFalse($this->matcher->matchesDenyList('192.168.1.50', $denylist));
    }

    public function testMatchesDenyListWithCIDRNotation()
    {
        $denylist = ['172.16.0.0/12'];

        $this->assertTrue($this->matcher->matchesDenyList('172.16.0.1', $denylist));
        $this->assertTrue($this->matcher->matchesDenyList('172.31.255.254', $denylist));
        $this->assertFalse($this->matcher->matchesDenyList('192.168.1.1', $denylist));
    }

    // ========== Combined Allowlist/Denylist Tests ==========

    public function testIsAllowedWithEmptyLists()
    {
        // Empty lists = allow all
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', [], []));
        $this->assertTrue($this->matcher->isAllowed('10.0.0.1', [], []));
    }

    public function testIsAllowedWithDenylistOnly()
    {
        $denylist = ['192.168.1.100', '10.0.0.0/24'];

        // Not in denylist = allowed
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', [], $denylist));
        $this->assertTrue($this->matcher->isAllowed('172.16.0.1', [], $denylist));

        // In denylist = denied
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', [], $denylist));
        $this->assertFalse($this->matcher->isAllowed('10.0.0.50', [], $denylist));
    }

    public function testIsAllowedWithAllowlistOnly()
    {
        $allowlist = ['192.168.1.0/24', '10.8.0.50'];

        // In allowlist = allowed
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $allowlist, []));
        $this->assertTrue($this->matcher->isAllowed('10.8.0.50', $allowlist, []));

        // Not in allowlist = denied
        $this->assertFalse($this->matcher->isAllowed('172.16.0.1', $allowlist, []));
        $this->assertFalse($this->matcher->isAllowed('10.8.0.51', $allowlist, []));
    }

    public function testIsAllowedWithBothLists()
    {
        $allowlist = ['192.168.1.0/24'];
        $denylist = ['192.168.1.100'];

        // In allowlist but NOT in denylist = allowed
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $allowlist, $denylist));

        // In BOTH allowlist AND denylist = denied (denylist wins)
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', $allowlist, $denylist));

        // Not in allowlist = denied
        $this->assertFalse($this->matcher->isAllowed('10.0.0.1', $allowlist, $denylist));
    }

    public function testIsAllowedDenylistTakesPrecedence()
    {
        // Denylist should always take precedence over allowlist
        $allowlist = ['*'];
        $denylist = ['192.168.1.100'];

        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $allowlist, $denylist));
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', $allowlist, $denylist));
    }

    // ========== Edge Cases ==========

    public function testMatchingWithLocalhostIPv4()
    {
        $allowlist = ['127.0.0.1'];
        $this->assertTrue($this->matcher->matchesAllowList('127.0.0.1', $allowlist));
    }

    public function testMatchingWithLocalhostIPv6()
    {
        $allowlist = ['::1'];
        $this->assertTrue($this->matcher->matchesAllowList('::1', $allowlist));
    }

    public function testIsAllowedWithWildcardAllowlistAndSpecificDenylist()
    {
        // Allow everyone except specific IPs
        $allowlist = ['*'];
        $denylist = ['192.168.1.100', '10.0.0.0/8'];

        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $allowlist, $denylist));
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', $allowlist, $denylist));
        $this->assertFalse($this->matcher->isAllowed('10.5.5.5', $allowlist, $denylist));
    }

    public function testIsAllowedWithOverlappingRanges()
    {
        // Overlapping CIDR ranges
        $allowlist = ['192.168.0.0/16'];
        $denylist = ['192.168.1.0/24'];

        $this->assertTrue($this->matcher->isAllowed('192.168.2.50', $allowlist, $denylist));
        $this->assertFalse($this->matcher->isAllowed('192.168.1.50', $allowlist, $denylist));
    }

    // ========== Anonymize IP Tests ==========

    public function testAnonymizeIpWithIPv4()
    {
        $anonymized = $this->matcher->anonymizeIp('192.168.1.50');
        $this->assertStringStartsWith('192.168.1.', $anonymized);
        $this->assertNotEquals('192.168.1.50', $anonymized);
    }

    public function testAnonymizeIpWithIPv6()
    {
        $anonymized = $this->matcher->anonymizeIp('2001:db8::1');
        $this->assertNotEmpty($anonymized);
        $this->assertNotEquals('2001:db8::1', $anonymized);
    }
}
