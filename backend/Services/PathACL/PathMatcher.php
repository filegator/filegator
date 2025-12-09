<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\PathACL;

/**
 * Path Matcher Utility
 *
 * Provides path manipulation and matching functionality for ACL evaluation:
 * - Path normalization (security-critical)
 * - Parent path traversal
 * - Path depth calculation
 * - Pattern matching
 */
class PathMatcher
{
    /**
     * Normalize path to canonical form.
     *
     * Security-critical method that ensures paths are in a consistent format
     * and prevents directory traversal attacks.
     *
     * Operations:
     * - Converts backslashes to forward slashes
     * - Removes duplicate slashes
     * - Removes trailing slashes (except root)
     * - Resolves . and .. components
     * - Ensures path starts with /
     *
     * @param string $path Path to normalize
     * @return string Normalized path
     * @throws \InvalidArgumentException If path contains directory traversal
     */
    public function normalizePath(string $path): string
    {
        // Convert backslashes to forward slashes
        $path = str_replace('\\', '/', $path);

        // Remove duplicate slashes
        $path = preg_replace('#/+#', '/', $path);

        // Ensure path starts with /
        if (!str_starts_with($path, '/')) {
            $path = '/' . $path;
        }

        // Security: Detect and reject obvious directory traversal attempts
        if (strpos($path, '..') !== false) {
            throw new \InvalidArgumentException('Path traversal detected: ' . $path);
        }

        // Split path into segments
        $segments = explode('/', trim($path, '/'));
        $normalized = [];

        foreach ($segments as $segment) {
            // Skip empty segments and current directory references
            if ($segment === '' || $segment === '.') {
                continue;
            }

            // Double-check for directory traversal (shouldn't reach here due to earlier check)
            if ($segment === '..') {
                throw new \InvalidArgumentException('Path traversal detected: ' . $path);
            }

            $normalized[] = $segment;
        }

        // Reconstruct path
        $result = '/' . implode('/', $normalized);

        // Remove trailing slash (except for root)
        if ($result !== '/' && str_ends_with($result, '/')) {
            $result = rtrim($result, '/');
        }

        return $result;
    }

    /**
     * Get all parent paths from most specific to root.
     *
     * Example: /projects/alpha/file.txt returns:
     * ['/projects/alpha', '/projects', '/']
     *
     * @param string $path Path to get parents for
     * @return array Array of parent paths (excluding the path itself)
     */
    public function getParentPaths(string $path): array
    {
        $normalized = $this->normalizePath($path);

        // Root has no parents
        if ($normalized === '/') {
            return [];
        }

        $parents = [];
        $current = $normalized;

        while ($current !== '/') {
            $parent = $this->getParentPath($current);
            $parents[] = $parent;
            $current = $parent;
        }

        return $parents;
    }

    /**
     * Get immediate parent path.
     *
     * @param string $path Path to get parent for
     * @return string Parent path (returns '/' for top-level paths)
     */
    public function getParentPath(string $path): string
    {
        $normalized = $this->normalizePath($path);

        // Root has no parent
        if ($normalized === '/') {
            return '/';
        }

        $lastSlash = strrpos($normalized, '/');

        // If slash is at position 0, parent is root
        if ($lastSlash === 0) {
            return '/';
        }

        return substr($normalized, 0, $lastSlash);
    }

    /**
     * Calculate path depth (number of segments).
     *
     * Examples:
     * - '/' = 0
     * - '/projects' = 1
     * - '/projects/alpha' = 2
     * - '/projects/alpha/file.txt' = 3
     *
     * @param string $path Path to calculate depth for
     * @return int Path depth (0 for root)
     */
    public function getPathDepth(string $path): int
    {
        $normalized = $this->normalizePath($path);

        if ($normalized === '/') {
            return 0;
        }

        return substr_count($normalized, '/');
    }

    /**
     * Check if a path matches a pattern.
     *
     * Currently supports exact matching only.
     * Future versions may support glob patterns (*.txt, /projects/*, etc.)
     *
     * @param string $path Path to check
     * @param string $pattern Pattern to match against
     * @return bool True if path matches pattern
     */
    public function matchesPattern(string $path, string $pattern): bool
    {
        $normalizedPath = $this->normalizePath($path);
        $normalizedPattern = $this->normalizePath($pattern);

        // Exact match
        return $normalizedPath === $normalizedPattern;
    }

    /**
     * Check if a path is within a parent path (used for inheritance checks).
     *
     * Examples:
     * - isWithinPath('/projects/alpha', '/projects') = true
     * - isWithinPath('/projects/alpha', '/public') = false
     * - isWithinPath('/projects', '/projects') = false (same path, not within)
     *
     * @param string $path Path to check
     * @param string $parentPath Parent path to check against
     * @return bool True if path is within parent path
     */
    public function isWithinPath(string $path, string $parentPath): bool
    {
        $normalizedPath = $this->normalizePath($path);
        $normalizedParent = $this->normalizePath($parentPath);

        // Same path is not "within"
        if ($normalizedPath === $normalizedParent) {
            return false;
        }

        // Root contains everything except itself
        if ($normalizedParent === '/') {
            return true;
        }

        // Check if path starts with parent path followed by /
        return str_starts_with($normalizedPath, $normalizedParent . '/');
    }

    /**
     * Get all ancestor paths including the path itself (for ACL traversal).
     *
     * Example: /projects/alpha/file.txt returns:
     * ['/projects/alpha/file.txt', '/projects/alpha', '/projects', '/']
     *
     * @param string $path Path to get ancestors for
     * @return array Array of paths from most specific to root
     */
    public function getPathAncestors(string $path): array
    {
        $normalized = $this->normalizePath($path);
        $ancestors = [$normalized];

        if ($normalized !== '/') {
            $ancestors = array_merge($ancestors, $this->getParentPaths($normalized));
        }

        return $ancestors;
    }
}
