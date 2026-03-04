<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit\Services\PathACL;

use Filegator\Services\Auth\User;
use Filegator\Services\PathACL\PathACL;
use Tests\TestCase;

/**
 * @internal
 */
class PathACLTest extends TestCase
{
    protected PathACL $pathAcl;

    protected function setUp(): void
    {
        parent::setUp();
        $this->pathAcl = new PathACL();
    }

    // ========== Helper Methods ==========

    protected function createUser(string $username = 'john.doe@example.com', string $role = 'user'): User
    {
        $user = new User();
        $user->setRole($role);
        $user->setUsername($username);
        $user->setName('John Doe');
        $user->setHomedir('/');
        $user->setPermissions([]);
        return $user;
    }

    protected function getBasicConfig(): array
    {
        return [
            'enabled' => true,
            'settings' => [
                'cache_enabled' => true,
                'cache_ttl' => 300,
                'fail_mode' => 'deny',
            ],
            'groups' => [
                'developers' => ['john.doe@example.com', 'jane.smith@example.com'],
                'managers' => ['alice.manager@example.com'],
            ],
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => ['192.168.1.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload', 'delete'],
                            'priority' => 10,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];
    }

    // ========== Initialization Tests ==========

    public function testInitializationWithEnabledConfig()
    {
        $config = $this->getBasicConfig();
        $this->pathAcl->init($config);

        $this->assertTrue($this->pathAcl->isEnabled());
    }

    public function testInitializationWithDisabledConfig()
    {
        $config = ['enabled' => false];
        $this->pathAcl->init($config);

        $this->assertFalse($this->pathAcl->isEnabled());
    }

    public function testIsEnabledReturnsFalseByDefault()
    {
        $acl = new PathACL();
        $this->assertFalse($acl->isEnabled());
    }

    // ========== Check Permission Tests ==========

    public function testCheckPermissionWhenDisabled()
    {
        $this->pathAcl->init(['enabled' => false]);

        $user = $this->createUser();
        $result = $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read');

        // When disabled, should return true (fall back to global permissions)
        $this->assertTrue($result);
    }

    public function testCheckPermissionWithMatchingRule()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('john.doe@example.com');
        $result = $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read');

        $this->assertTrue($result);
    }

    public function testCheckPermissionWithNonMatchingIP()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('john.doe@example.com');
        $result = $this->pathAcl->checkPermission($user, '10.0.0.1', '/projects', 'read');

        $this->assertFalse($result);
    }

    public function testCheckPermissionWithNonMatchingUser()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('unauthorized@example.com');
        $result = $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read');

        $this->assertFalse($result);
    }

    public function testCheckPermissionWithNonExistentPermission()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('john.doe@example.com');
        $result = $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'admin');

        $this->assertFalse($result);
    }

    public function testCheckPermissionWithWildcardUser()
    {
        $config = $this->getBasicConfig();
        $config['path_rules']['/public'] = [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['*'],
                    'ip_inclusions' => [],
                    'ip_exclusions' => [],
                    'permissions' => ['read'],
                    'priority' => 0,
                    'override_inherited' => false,
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('anyone@example.com');
        $result = $this->pathAcl->checkPermission($user, '192.168.1.50', '/public', 'read');

        $this->assertTrue($result);
    }

    public function testCheckPermissionWithGroupMatch()
    {
        $config = $this->getBasicConfig();
        $config['path_rules']['/dev'] = [
            'inherit' => true,
            'rules' => [
                [
                    'users' => ['@developers'],
                    'ip_inclusions' => [],
                    'ip_exclusions' => [],
                    'permissions' => ['read', 'write'],
                    'priority' => 0,
                    'override_inherited' => false,
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('jane.smith@example.com');
        $result = $this->pathAcl->checkPermission($user, '10.0.0.1', '/dev', 'read');

        $this->assertTrue($result);
    }

    // ========== Priority and Override Tests ==========

    public function testHigherPriorityRuleWins()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 5,
                            'override_inherited' => false,
                        ],
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'delete'],
                            'priority' => 10,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');
        $perms = $this->pathAcl->getEffectivePermissions($user, '10.0.0.1', '/projects');

        $this->assertContains('read', $perms);
        $this->assertContains('write', $perms);
        $this->assertContains('delete', $perms);
    }

    public function testOverrideInheritedStopsInheritance()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['*'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload', 'delete'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 10,
                            'override_inherited' => true,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');
        $perms = $this->pathAcl->getEffectivePermissions($user, '10.0.0.1', '/projects');

        // Should only have 'read', not inherited permissions
        $this->assertEquals(['read'], $perms);
    }

    // ========== Inheritance Tests ==========

    public function testInheritanceFromParentPath()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');
        $result = $this->pathAcl->checkPermission($user, '10.0.0.1', '/projects/alpha/file.txt', 'read');

        $this->assertTrue($result);
    }

    public function testInheritanceFalseStopsInheritance()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['*'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/restricted' => [
                    'inherit' => false,
                    'rules' => [
                        [
                            'users' => ['admin@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');
        $result = $this->pathAcl->checkPermission($user, '10.0.0.1', '/restricted', 'read');

        // Inheritance disabled, and user is not in /restricted rules
        $this->assertFalse($result);
    }

    // ========== IP Inclusions/Exclusions Tests ==========

    public function testIPExclusionsBlocksAccess()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => ['*'],
                            'ip_exclusions' => ['192.168.1.100'],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');

        // From allowed IP
        $this->assertTrue($this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read'));

        // From denied IP
        $this->assertFalse($this->pathAcl->checkPermission($user, '192.168.1.100', '/projects', 'read'));
    }

    public function testIPInclusionsRestrictsAccess()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => ['192.168.1.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');

        // From allowed range
        $this->assertTrue($this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read'));

        // From outside allowed range
        $this->assertFalse($this->pathAcl->checkPermission($user, '10.0.0.1', '/projects', 'read'));
    }

    // ========== Get Effective Permissions Tests ==========

    public function testGetEffectivePermissionsReturnsAllGrantedPermissions()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('john.doe@example.com');
        $perms = $this->pathAcl->getEffectivePermissions($user, '192.168.1.50', '/projects');

        $this->assertContains('read', $perms);
        $this->assertContains('write', $perms);
        $this->assertContains('upload', $perms);
        $this->assertContains('delete', $perms);
    }

    public function testGetEffectivePermissionsReturnsEmptyWhenNoMatch()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('unauthorized@example.com');
        $perms = $this->pathAcl->getEffectivePermissions($user, '192.168.1.50', '/projects');

        $this->assertEmpty($perms);
    }

    public function testGetEffectivePermissionsMergesMultipleRules()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['write', 'upload'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');
        $perms = $this->pathAcl->getEffectivePermissions($user, '10.0.0.1', '/projects');

        $this->assertContains('read', $perms);
        $this->assertContains('write', $perms);
        $this->assertContains('upload', $perms);
    }

    // ========== Cache Tests ==========

    public function testCachingImprovesPerformance()
    {
        $config = $this->getBasicConfig();
        $config['settings']['cache_enabled'] = true;

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');

        // First call - should be cached
        $result1 = $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read');

        // Second call - should use cache
        $result2 = $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read');

        $this->assertEquals($result1, $result2);
        $this->assertTrue($result1);
    }

    public function testClearCacheRemovesCachedResults()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('john.doe@example.com');

        // Cache a result
        $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read');

        // Clear cache
        $this->pathAcl->clearCache();

        // This should work without issues (re-evaluate from scratch)
        $result = $this->pathAcl->checkPermission($user, '192.168.1.50', '/projects', 'read');
        $this->assertTrue($result);
    }

    // ========== Explain Permission Tests ==========

    public function testExplainPermissionReturnsDetailedInfo()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('john.doe@example.com');
        $explanation = $this->pathAcl->explainPermission($user, '192.168.1.50', '/projects', 'read');

        $this->assertArrayHasKey('allowed', $explanation);
        $this->assertArrayHasKey('reason', $explanation);
        $this->assertArrayHasKey('matched_rules', $explanation);
        $this->assertArrayHasKey('effective_permissions', $explanation);
        $this->assertArrayHasKey('requested_permission', $explanation);
        $this->assertArrayHasKey('user_ip_check', $explanation);
        $this->assertArrayHasKey('evaluation_path', $explanation);

        $this->assertTrue($explanation['allowed']);
        $this->assertEquals('read', $explanation['requested_permission']);
    }

    public function testExplainPermissionWhenDenied()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('unauthorized@example.com');
        $explanation = $this->pathAcl->explainPermission($user, '192.168.1.50', '/projects', 'read');

        $this->assertFalse($explanation['allowed']);
        $this->assertStringContainsString('No matching ACL rules', $explanation['reason']);
    }

    public function testExplainPermissionShowsEvaluationPath()
    {
        $this->pathAcl->init($this->getBasicConfig());

        $user = $this->createUser('john.doe@example.com');
        $explanation = $this->pathAcl->explainPermission($user, '192.168.1.50', '/projects/alpha/file.txt', 'read');

        $this->assertIsArray($explanation['evaluation_path']);
        $this->assertContains('/projects/alpha/file.txt', $explanation['evaluation_path']);
        $this->assertContains('/projects/alpha', $explanation['evaluation_path']);
        $this->assertContains('/projects', $explanation['evaluation_path']);
        $this->assertContains('/', $explanation['evaluation_path']);
    }

    // ========== Edge Cases ==========

    public function testCheckPermissionWithRootPath()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['*'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('anyone@example.com');
        $result = $this->pathAcl->checkPermission($user, '10.0.0.1', '/', 'read');

        $this->assertTrue($result);
    }

    public function testCheckPermissionWithDeepNestedPath()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john.doe@example.com'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('john.doe@example.com');
        $result = $this->pathAcl->checkPermission($user, '10.0.0.1', '/projects/a/b/c/d/e/f/file.txt', 'read');

        $this->assertTrue($result);
    }

    public function testDefaultPermissionsFallbackWhenNoRulesMatch()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [],
        ];

        $this->pathAcl->init($config);

        $user = $this->createUser('anyone@example.com');
        $result = $this->pathAcl->checkPermission($user, '10.0.0.1', '/any/path', 'read');

        // No rules match, should deny
        $this->assertFalse($result);
    }
}
