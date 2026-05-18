<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Services\PathACL;

use Filegator\Services\PathACL\PathMatcher;
use Tests\TestCase;

/**
 * @internal
 */
class PathMatcherTest extends TestCase
{
    protected PathMatcher $matcher;

    protected function setUp(): void
    {
        parent::setUp();
        $this->matcher = new PathMatcher();
    }

    // ========== Path Normalization Tests ==========

    public function testNormalizePathWithSimplePath()
    {
        $this->assertEquals('/projects', $this->matcher->normalizePath('/projects'));
        $this->assertEquals('/projects/alpha', $this->matcher->normalizePath('/projects/alpha'));
    }

    public function testNormalizePathWithRootPath()
    {
        $this->assertEquals('/', $this->matcher->normalizePath('/'));
        $this->assertEquals('/', $this->matcher->normalizePath('//'));
    }

    public function testNormalizePathWithTrailingSlash()
    {
        $this->assertEquals('/projects', $this->matcher->normalizePath('/projects/'));
        $this->assertEquals('/projects/alpha', $this->matcher->normalizePath('/projects/alpha/'));
    }

    public function testNormalizePathWithMultipleSlashes()
    {
        $this->assertEquals('/projects/alpha', $this->matcher->normalizePath('/projects//alpha'));
        $this->assertEquals('/projects/alpha/beta', $this->matcher->normalizePath('///projects///alpha//beta///'));
    }

    public function testNormalizePathWithBackslashes()
    {
        $this->assertEquals('/projects/alpha', $this->matcher->normalizePath('\\projects\\alpha'));
        $this->assertEquals('/projects/alpha/beta', $this->matcher->normalizePath('\\projects/alpha\\beta'));
    }

    public function testNormalizePathWithoutLeadingSlash()
    {
        $this->assertEquals('/projects', $this->matcher->normalizePath('projects'));
        $this->assertEquals('/projects/alpha', $this->matcher->normalizePath('projects/alpha'));
    }

    public function testNormalizePathWithCurrentDirectory()
    {
        $this->assertEquals('/projects', $this->matcher->normalizePath('/./projects'));
        $this->assertEquals('/projects/alpha', $this->matcher->normalizePath('/projects/./alpha'));
    }

    public function testNormalizePathRejectsDirectoryTraversal()
    {
        $this->expectException(\InvalidArgumentException::class);
        $this->expectExceptionMessage('Path traversal detected');
        $this->matcher->normalizePath('/projects/../etc/passwd');
    }

    public function testNormalizePathRejectsParentDirectoryReference()
    {
        $this->expectException(\InvalidArgumentException::class);
        $this->matcher->normalizePath('/projects/..');
    }

    public function testNormalizePathRejectsDoubleDotsInPath()
    {
        $this->expectException(\InvalidArgumentException::class);
        $this->matcher->normalizePath('/../etc/passwd');
    }

    // ========== Parent Path Tests ==========

    public function testGetParentPathOfRootPath()
    {
        $this->assertEquals('/', $this->matcher->getParentPath('/'));
    }

    public function testGetParentPathOfTopLevelPath()
    {
        $this->assertEquals('/', $this->matcher->getParentPath('/projects'));
        $this->assertEquals('/', $this->matcher->getParentPath('/uploads'));
    }

    public function testGetParentPathOfNestedPath()
    {
        $this->assertEquals('/projects', $this->matcher->getParentPath('/projects/alpha'));
        $this->assertEquals('/projects/alpha', $this->matcher->getParentPath('/projects/alpha/beta'));
    }

    public function testGetParentPathNormalizesInput()
    {
        $this->assertEquals('/projects', $this->matcher->getParentPath('/projects/alpha/'));
        $this->assertEquals('/projects', $this->matcher->getParentPath('//projects//alpha//'));
    }

    // ========== Get Parent Paths Tests ==========

    public function testGetParentPathsOfRootPath()
    {
        $parents = $this->matcher->getParentPaths('/');
        $this->assertEquals([], $parents);
    }

    public function testGetParentPathsOfTopLevelPath()
    {
        $parents = $this->matcher->getParentPaths('/projects');
        $this->assertEquals(['/'], $parents);
    }

    public function testGetParentPathsOfNestedPath()
    {
        $parents = $this->matcher->getParentPaths('/projects/alpha/file.txt');
        $this->assertEquals(['/projects/alpha', '/projects', '/'], $parents);
    }

    public function testGetParentPathsReturnsMostSpecificFirst()
    {
        $parents = $this->matcher->getParentPaths('/a/b/c/d');
        $this->assertEquals(['/a/b/c', '/a/b', '/a', '/'], $parents);
    }

    public function testGetParentPathsExcludesPathItself()
    {
        $parents = $this->matcher->getParentPaths('/projects/alpha');
        $this->assertNotContains('/projects/alpha', $parents);
    }

    // ========== Path Depth Tests ==========

    public function testGetPathDepthOfRoot()
    {
        $this->assertEquals(0, $this->matcher->getPathDepth('/'));
    }

    public function testGetPathDepthOfTopLevel()
    {
        $this->assertEquals(1, $this->matcher->getPathDepth('/projects'));
        $this->assertEquals(1, $this->matcher->getPathDepth('/uploads'));
    }

    public function testGetPathDepthOfNestedPaths()
    {
        $this->assertEquals(2, $this->matcher->getPathDepth('/projects/alpha'));
        $this->assertEquals(3, $this->matcher->getPathDepth('/projects/alpha/file.txt'));
        $this->assertEquals(4, $this->matcher->getPathDepth('/a/b/c/d'));
    }

    public function testGetPathDepthNormalizesPath()
    {
        $this->assertEquals(2, $this->matcher->getPathDepth('/projects/alpha/'));
        $this->assertEquals(2, $this->matcher->getPathDepth('//projects//alpha//'));
    }

    // ========== Pattern Matching Tests ==========

    public function testMatchesPatternWithExactMatch()
    {
        $this->assertTrue($this->matcher->matchesPattern('/projects', '/projects'));
        $this->assertTrue($this->matcher->matchesPattern('/projects/alpha', '/projects/alpha'));
    }

    public function testMatchesPatternWithNormalization()
    {
        $this->assertTrue($this->matcher->matchesPattern('/projects/', '/projects'));
        $this->assertTrue($this->matcher->matchesPattern('//projects//', '/projects'));
    }

    public function testMatchesPatternWithNoMatch()
    {
        $this->assertFalse($this->matcher->matchesPattern('/projects', '/uploads'));
        $this->assertFalse($this->matcher->matchesPattern('/projects/alpha', '/projects/beta'));
    }

    public function testMatchesPatternIsCaseSensitive()
    {
        $this->assertFalse($this->matcher->matchesPattern('/Projects', '/projects'));
        $this->assertFalse($this->matcher->matchesPattern('/PROJECTS', '/projects'));
    }

    public function testMatchesPatternDoesNotMatchSubpaths()
    {
        $this->assertFalse($this->matcher->matchesPattern('/projects/alpha', '/projects'));
        $this->assertFalse($this->matcher->matchesPattern('/projects', '/projects/alpha'));
    }

    // ========== Is Within Path Tests ==========

    public function testIsWithinPathWithChildPath()
    {
        $this->assertTrue($this->matcher->isWithinPath('/projects/alpha', '/projects'));
        $this->assertTrue($this->matcher->isWithinPath('/projects/alpha/beta', '/projects'));
        $this->assertTrue($this->matcher->isWithinPath('/projects/alpha/beta', '/projects/alpha'));
    }

    public function testIsWithinPathWithRootAsParent()
    {
        $this->assertTrue($this->matcher->isWithinPath('/projects', '/'));
        $this->assertTrue($this->matcher->isWithinPath('/uploads', '/'));
        $this->assertTrue($this->matcher->isWithinPath('/a/b/c', '/'));
    }

    public function testIsWithinPathWithSamePath()
    {
        // Same path is NOT "within"
        $this->assertFalse($this->matcher->isWithinPath('/projects', '/projects'));
        $this->assertFalse($this->matcher->isWithinPath('/', '/'));
    }

    public function testIsWithinPathWithUnrelatedPaths()
    {
        $this->assertFalse($this->matcher->isWithinPath('/projects', '/uploads'));
        $this->assertFalse($this->matcher->isWithinPath('/uploads/files', '/projects'));
    }

    public function testIsWithinPathWithSimilarNamedPaths()
    {
        // /projects-backup should NOT be within /projects
        $this->assertFalse($this->matcher->isWithinPath('/projects-backup', '/projects'));
        $this->assertFalse($this->matcher->isWithinPath('/projectsalpha', '/projects'));
    }

    public function testIsWithinPathWithParentAsChild()
    {
        $this->assertFalse($this->matcher->isWithinPath('/projects', '/projects/alpha'));
    }

    // ========== Get Path Ancestors Tests ==========

    public function testGetPathAncestorsOfRootPath()
    {
        $ancestors = $this->matcher->getPathAncestors('/');
        $this->assertEquals(['/'], $ancestors);
    }

    public function testGetPathAncestorsOfTopLevelPath()
    {
        $ancestors = $this->matcher->getPathAncestors('/projects');
        $this->assertEquals(['/projects', '/'], $ancestors);
    }

    public function testGetPathAncestorsOfNestedPath()
    {
        $ancestors = $this->matcher->getPathAncestors('/projects/alpha/file.txt');
        $this->assertEquals(['/projects/alpha/file.txt', '/projects/alpha', '/projects', '/'], $ancestors);
    }

    public function testGetPathAncestorsIncludesPathItself()
    {
        $ancestors = $this->matcher->getPathAncestors('/projects/alpha');
        $this->assertEquals('/projects/alpha', $ancestors[0]);
    }

    public function testGetPathAncestorsReturnsMostSpecificFirst()
    {
        $ancestors = $this->matcher->getPathAncestors('/a/b/c/d');
        $this->assertEquals(['/a/b/c/d', '/a/b/c', '/a/b', '/a', '/'], $ancestors);
    }

    public function testGetPathAncestorsEndsWithRoot()
    {
        $ancestors = $this->matcher->getPathAncestors('/projects/alpha');
        $this->assertEquals('/', end($ancestors));
    }

    // ========== Edge Cases ==========

    public function testNormalizePathWithEmptyString()
    {
        $this->assertEquals('/', $this->matcher->normalizePath(''));
    }

    public function testNormalizePathWithOnlySlashes()
    {
        $this->assertEquals('/', $this->matcher->normalizePath('/'));
        $this->assertEquals('/', $this->matcher->normalizePath('//'));
        $this->assertEquals('/', $this->matcher->normalizePath('///'));
    }

    public function testNormalizePathWithSpacesInPath()
    {
        $this->assertEquals('/my folder', $this->matcher->normalizePath('/my folder'));
        $this->assertEquals('/my folder/subfolder', $this->matcher->normalizePath('/my folder/subfolder'));
    }

    public function testNormalizePathWithSpecialCharacters()
    {
        $this->assertEquals('/projects/2024-plan', $this->matcher->normalizePath('/projects/2024-plan'));
        $this->assertEquals('/files_backup', $this->matcher->normalizePath('/files_backup'));
        $this->assertEquals('/user@domain', $this->matcher->normalizePath('/user@domain'));
    }

    public function testNormalizePathWithUnicodeCharacters()
    {
        $this->assertEquals('/проекты', $this->matcher->normalizePath('/проекты'));
        $this->assertEquals('/文件', $this->matcher->normalizePath('/文件'));
    }

    public function testPathOperationsAreConsistent()
    {
        $path = '/projects/alpha/beta/file.txt';

        $ancestors = $this->matcher->getPathAncestors($path);
        $this->assertEquals($path, $ancestors[0]);

        $parents = $this->matcher->getParentPaths($path);
        $this->assertEquals($parents, array_slice($ancestors, 1));
    }
}
