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
 * - Allowlist and denylist evaluation
 *
 * Uses Symfony's IpUtils for robust IP matching.
 */
class IpMatcher
{
    /**
     * Check if an IP address matches any pattern in the allowlist.
     *
     * @param string $ip IP address to check
     * @param array $allowlist Array of allowed IP patterns (CIDR, single IPs, or '*')
     * @return bool True if IP matches allowlist (or allowlist contains '*')
     */
    public function matchesAllowList(string $ip, array $allowlist): bool
    {
        // Empty allowlist means allow all (no restriction)
        if (empty($allowlist)) {
            return true;
        }

        // Wildcard matches everything
        if (in_array('*', $allowlist, true)) {
            return true;
        }

        // Use Symfony IpUtils for CIDR and exact matching
        return IpUtils::checkIp($ip, $allowlist);
    }

    /**
     * Check if an IP address matches any pattern in the denylist.
     *
     * @param string $ip IP address to check
     * @param array $denylist Array of denied IP patterns (CIDR, single IPs, or '*')
     * @return bool True if IP matches denylist (should be blocked)
     */
    public function matchesDenyList(string $ip, array $denylist): bool
    {
        // Empty denylist means deny nothing
        if (empty($denylist)) {
            return false;
        }

        // Wildcard denies everything
        if (in_array('*', $denylist, true)) {
            return true;
        }

        // Use Symfony IpUtils for CIDR and exact matching
        return IpUtils::checkIp($ip, $denylist);
    }

    /**
     * Determine if IP is allowed based on allowlist and denylist.
     *
     * Evaluation order (security-first):
     * 1. Check denylist - if match found, immediately deny
     * 2. Check allowlist - if non-empty, IP must be in allowlist
     * 3. If allowlist is empty, allow by default (denylist-only mode)
     *
     * @param string $ip IP address to check
     * @param array $allowlist Array of allowed IP patterns
     * @param array $denylist Array of denied IP patterns
     * @return bool True if IP is allowed (passes both checks)
     */
    public function isAllowed(string $ip, array $allowlist, array $denylist): bool
    {
        // Step 1: Deny always wins - check denylist first
        if ($this->matchesDenyList($ip, $denylist)) {
            return false;
        }

        // Step 2: Check allowlist (if non-empty, IP must be in list)
        if (!empty($allowlist)) {
            return $this->matchesAllowList($ip, $allowlist);
        }

        // Step 3: Empty allowlist = allow all (unless denied above)
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
