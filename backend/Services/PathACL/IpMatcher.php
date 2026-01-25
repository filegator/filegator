<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\PathACL;

use Symfony\Component\HttpFoundation\IpUtils;

/**
 * IP Matcher Utility
 *
 * Provides IP address matching functionality with support for:
 * - Single IPv4/IPv6 addresses
 * - CIDR notation (e.g., 192.168.1.0/24)
 * - Wildcard (*) for all IPs
 * - Inclusions and exclusions evaluation
 *
 * Uses Symfony's IpUtils for robust IP matching.
 */
class IpMatcher
{
    /**
     * Check if an IP address matches any pattern in the inclusions list.
     *
     * @param string $ip IP address to check
     * @param array $inclusions Array of included IP patterns (CIDR, single IPs, or '*')
     * @return bool True if IP matches inclusions (or inclusions contains '*')
     */
    public function matchesInclusions(string $ip, array $inclusions): bool
    {
        // Empty inclusions means include all (no restriction)
        if (empty($inclusions)) {
            return true;
        }

        // Wildcard matches everything
        if (in_array('*', $inclusions, true)) {
            return true;
        }

        // Use Symfony IpUtils for CIDR and exact matching
        return IpUtils::checkIp($ip, $inclusions);
    }

    /**
     * Check if an IP address matches any pattern in the exclusions list.
     *
     * @param string $ip IP address to check
     * @param array $exclusions Array of excluded IP patterns (CIDR, single IPs, or '*')
     * @return bool True if IP matches exclusions (should be blocked)
     */
    public function matchesExclusions(string $ip, array $exclusions): bool
    {
        // Empty exclusions means exclude nothing
        if (empty($exclusions)) {
            return false;
        }

        // Wildcard excludes everything
        if (in_array('*', $exclusions, true)) {
            return true;
        }

        // Use Symfony IpUtils for CIDR and exact matching
        return IpUtils::checkIp($ip, $exclusions);
    }

    /**
     * Determine if IP is allowed based on inclusions and exclusions.
     *
     * Evaluation order (security-first):
     * 1. Check exclusions - if match found, immediately deny
     * 2. Check inclusions - if non-empty, IP must be in inclusions
     * 3. If inclusions is empty, allow by default (exclusions-only mode)
     *
     * @param string $ip IP address to check
     * @param array $inclusions Array of included IP patterns
     * @param array $exclusions Array of excluded IP patterns
     * @return bool True if IP is allowed (passes both checks)
     */
    public function isAllowed(string $ip, array $inclusions, array $exclusions): bool
    {
        // Step 1: Exclusions always win - check exclusions first
        if ($this->matchesExclusions($ip, $exclusions)) {
            return false;
        }

        // Step 2: Check inclusions (if non-empty, IP must be in list)
        if (!empty($inclusions)) {
            return $this->matchesInclusions($ip, $inclusions);
        }

        // Step 3: Empty inclusions = allow all (unless excluded above)
        return true;
    }

    /**
     * Validate IP address format.
     *
     * @param string $ip IP address to validate
     * @return bool True if valid IPv4 or IPv6 address
     */
    public function isValidIp(string $ip): bool
    {
        return filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV4 | FILTER_FLAG_IPV6) !== false;
    }

    /**
     * Validate IP pattern (single IP or CIDR).
     *
     * @param string $pattern IP pattern to validate
     * @return bool True if valid IP or CIDR notation
     */
    public function isValidPattern(string $pattern): bool
    {
        // Wildcard is always valid
        if ($pattern === '*') {
            return true;
        }

        // Check if it's a CIDR range
        if (strpos($pattern, '/') !== false) {
            list($ip, $mask) = explode('/', $pattern, 2);

            // Validate IP part
            if (!$this->isValidIp($ip)) {
                return false;
            }

            // Validate mask (0-32 for IPv4, 0-128 for IPv6)
            $maxMask = filter_var($ip, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6) !== false ? 128 : 32;
            return is_numeric($mask) && $mask >= 0 && $mask <= $maxMask;
        }

        // Single IP address
        return $this->isValidIp($pattern);
    }

    /**
     * Anonymize IP address for logging (GDPR compliance).
     *
     * @param string $ip IP address to anonymize
     * @return string Anonymized IP (last octet/segment removed)
     */
    public function anonymizeIp(string $ip): string
    {
        return IpUtils::anonymize($ip);
    }
}
