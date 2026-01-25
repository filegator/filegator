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

    // ========== Inclusions Matching Tests ==========

    public function testMatchesInclusionsWithEmptyList()
    {
        // Empty inclusions means include all
        $this->assertTrue($this->matcher->matchesInclusions('192.168.1.50', []));
        $this->assertTrue($this->matcher->matchesInclusions('10.0.0.1', []));
    }

    public function testMatchesInclusionsWithWildcard()
    {
        $inclusions = ['*'];
        $this->assertTrue($this->matcher->matchesInclusions('192.168.1.50', $inclusions));
        $this->assertTrue($this->matcher->matchesInclusions('10.0.0.1', $inclusions));
        $this->assertTrue($this->matcher->matchesInclusions('2001:db8::1', $inclusions));
    }

    public function testMatchesInclusionsWithExactIPMatch()
    {
        $inclusions = ['192.168.1.50', '10.8.0.50'];

        $this->assertTrue($this->matcher->matchesInclusions('192.168.1.50', $inclusions));
        $this->assertTrue($this->matcher->matchesInclusions('10.8.0.50', $inclusions));
        $this->assertFalse($this->matcher->matchesInclusions('192.168.1.51', $inclusions));
        $this->assertFalse($this->matcher->matchesInclusions('172.16.0.1', $inclusions));
    }

    public function testMatchesInclusionsWithCIDRNotation()
    {
        $inclusions = ['192.168.1.0/24'];

        $this->assertTrue($this->matcher->matchesInclusions('192.168.1.1', $inclusions));
        $this->assertTrue($this->matcher->matchesInclusions('192.168.1.50', $inclusions));
        $this->assertTrue($this->matcher->matchesInclusions('192.168.1.254', $inclusions));
        $this->assertFalse($this->matcher->matchesInclusions('192.168.2.1', $inclusions));
        $this->assertFalse($this->matcher->matchesInclusions('10.0.0.1', $inclusions));
    }

    public function testMatchesInclusionsWithMultipleCIDRRanges()
    {
        $inclusions = ['192.168.1.0/24', '10.8.0.0/24'];

        $this->assertTrue($this->matcher->matchesInclusions('192.168.1.50', $inclusions));
        $this->assertTrue($this->matcher->matchesInclusions('10.8.0.50', $inclusions));
        $this->assertFalse($this->matcher->matchesInclusions('172.16.0.1', $inclusions));
    }

    public function testMatchesInclusionsWithIPv6()
    {
        $inclusions = ['2001:db8::/32'];

        $this->assertTrue($this->matcher->matchesInclusions('2001:db8::1', $inclusions));
        $this->assertTrue($this->matcher->matchesInclusions('2001:db8:1::1', $inclusions));
        $this->assertFalse($this->matcher->matchesInclusions('2001:db9::1', $inclusions));
    }

    // ========== Exclusions Matching Tests ==========

    public function testMatchesExclusionsWithEmptyList()
    {
        // Empty exclusions means exclude nothing
        $this->assertFalse($this->matcher->matchesExclusions('192.168.1.50', []));
        $this->assertFalse($this->matcher->matchesExclusions('10.0.0.1', []));
    }

    public function testMatchesExclusionsWithWildcard()
    {
        $exclusions = ['*'];
        $this->assertTrue($this->matcher->matchesExclusions('192.168.1.50', $exclusions));
        $this->assertTrue($this->matcher->matchesExclusions('10.0.0.1', $exclusions));
    }

    public function testMatchesExclusionsWithExactIPMatch()
    {
        $exclusions = ['192.168.1.100', '10.0.0.1'];

        $this->assertTrue($this->matcher->matchesExclusions('192.168.1.100', $exclusions));
        $this->assertTrue($this->matcher->matchesExclusions('10.0.0.1', $exclusions));
        $this->assertFalse($this->matcher->matchesExclusions('192.168.1.50', $exclusions));
    }

    public function testMatchesExclusionsWithCIDRNotation()
    {
        $exclusions = ['172.16.0.0/12'];

        $this->assertTrue($this->matcher->matchesExclusions('172.16.0.1', $exclusions));
        $this->assertTrue($this->matcher->matchesExclusions('172.31.255.254', $exclusions));
        $this->assertFalse($this->matcher->matchesExclusions('192.168.1.1', $exclusions));
    }

    // ========== Combined Inclusions/Exclusions Tests ==========

    public function testIsAllowedWithEmptyLists()
    {
        // Empty lists = allow all
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', [], []));
        $this->assertTrue($this->matcher->isAllowed('10.0.0.1', [], []));
    }

    public function testIsAllowedWithExclusionsOnly()
    {
        $exclusions = ['192.168.1.100', '10.0.0.0/24'];

        // Not in exclusions = allowed
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', [], $exclusions));
        $this->assertTrue($this->matcher->isAllowed('172.16.0.1', [], $exclusions));

        // In exclusions = denied
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', [], $exclusions));
        $this->assertFalse($this->matcher->isAllowed('10.0.0.50', [], $exclusions));
    }

    public function testIsAllowedWithInclusionsOnly()
    {
        $inclusions = ['192.168.1.0/24', '10.8.0.50'];

        // In inclusions = allowed
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $inclusions, []));
        $this->assertTrue($this->matcher->isAllowed('10.8.0.50', $inclusions, []));

        // Not in inclusions = denied
        $this->assertFalse($this->matcher->isAllowed('172.16.0.1', $inclusions, []));
        $this->assertFalse($this->matcher->isAllowed('10.8.0.51', $inclusions, []));
    }

    public function testIsAllowedWithBothLists()
    {
        $inclusions = ['192.168.1.0/24'];
        $exclusions = ['192.168.1.100'];

        // In inclusions but NOT in exclusions = allowed
        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $inclusions, $exclusions));

        // In BOTH inclusions AND exclusions = denied (exclusions win)
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', $inclusions, $exclusions));

        // Not in inclusions = denied
        $this->assertFalse($this->matcher->isAllowed('10.0.0.1', $inclusions, $exclusions));
    }

    public function testIsAllowedExclusionsTakesPrecedence()
    {
        // Exclusions should always take precedence over inclusions
        $inclusions = ['*'];
        $exclusions = ['192.168.1.100'];

        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $inclusions, $exclusions));
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', $inclusions, $exclusions));
    }

    // ========== Edge Cases ==========

    public function testMatchingWithLocalhostIPv4()
    {
        $inclusions = ['127.0.0.1'];
        $this->assertTrue($this->matcher->matchesInclusions('127.0.0.1', $inclusions));
    }

    public function testMatchingWithLocalhostIPv6()
    {
        $inclusions = ['::1'];
        $this->assertTrue($this->matcher->matchesInclusions('::1', $inclusions));
    }

    public function testIsAllowedWithWildcardInclusionsAndSpecificExclusions()
    {
        // Allow everyone except specific IPs
        $inclusions = ['*'];
        $exclusions = ['192.168.1.100', '10.0.0.0/8'];

        $this->assertTrue($this->matcher->isAllowed('192.168.1.50', $inclusions, $exclusions));
        $this->assertFalse($this->matcher->isAllowed('192.168.1.100', $inclusions, $exclusions));
        $this->assertFalse($this->matcher->isAllowed('10.5.5.5', $inclusions, $exclusions));
    }

    public function testIsAllowedWithOverlappingRanges()
    {
        // Overlapping CIDR ranges
        $inclusions = ['192.168.0.0/16'];
        $exclusions = ['192.168.1.0/24'];

        $this->assertTrue($this->matcher->isAllowed('192.168.2.50', $inclusions, $exclusions));
        $this->assertFalse($this->matcher->isAllowed('192.168.1.50', $inclusions, $exclusions));
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
