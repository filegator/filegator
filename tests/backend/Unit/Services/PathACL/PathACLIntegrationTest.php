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
 * Integration tests for PathACL system
 * Tests complete real-world scenarios with complex rule configurations
 *
 * @internal
 */
class PathACLIntegrationTest extends TestCase
{
    protected PathACL $pathAcl;

    protected function setUp(): void
    {
        parent::setUp();
        $this->pathAcl = new PathACL();
    }

    // ========== Helper Methods ==========

    protected function createUser(string $username, string $role = 'user'): User
    {
        $user = new User();
        $user->setRole($role);
        $user->setUsername($username);
        $user->setName(ucfirst(explode('@', $username)[0]));
        $user->setHomedir('/');
        $user->setPermissions([]);
        return $user;
    }

    // ========== Scenario: Multi-user, Multi-folder, IP-based Access ==========

    /**
     * Test the primary use case from the requirements:
     * - John from 192.168.1.50 accessing /projects -> full access
     * - John from 10.8.0.50 accessing /projects -> read only
     * - John from any IP accessing /john-private -> full access
     * - John from any IP accessing /restricted -> denied
     */
    public function testCompleteMultiUserMultiFolderScenario()
    {
        $config = [
            'enabled' => true,
            'settings' => [
                'cache_enabled' => true,
                'fail_mode' => 'deny',
            ],
            'groups' => [
                'developers' => ['john@example.com', 'jane@example.com'],
                'admins' => ['alice@example.com'],
            ],
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john@example.com'],
                            'ip_inclusions' => ['192.168.1.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload', 'delete'],
                            'priority' => 20,
                            'override_inherited' => false,
                        ],
                        [
                            'users' => ['john@example.com'],
                            'ip_inclusions' => ['10.8.0.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 10,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/john-private' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john@example.com'],
                            'ip_inclusions' => ['*'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload', 'delete'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/restricted' => [
                    'inherit' => false,
                    'rules' => [
                        [
                            'users' => ['@admins'],
                            'ip_inclusions' => ['*'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload', 'delete'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);
        $john = $this->createUser('john@example.com');

        // Scenario 1: John from 192.168.1.50 accessing /projects -> full access
        $this->assertTrue($this->pathAcl->checkPermission($john, '192.168.1.50', '/projects', 'read'));
        $this->assertTrue($this->pathAcl->checkPermission($john, '192.168.1.50', '/projects', 'write'));
        $this->assertTrue($this->pathAcl->checkPermission($john, '192.168.1.50', '/projects', 'upload'));
        $this->assertTrue($this->pathAcl->checkPermission($john, '192.168.1.50', '/projects', 'delete'));

        // Scenario 2: John from 10.8.0.50 accessing /projects -> read only
        $this->assertTrue($this->pathAcl->checkPermission($john, '10.8.0.50', '/projects', 'read'));
        $this->assertFalse($this->pathAcl->checkPermission($john, '10.8.0.50', '/projects', 'write'));
        $this->assertFalse($this->pathAcl->checkPermission($john, '10.8.0.50', '/projects', 'upload'));
        $this->assertFalse($this->pathAcl->checkPermission($john, '10.8.0.50', '/projects', 'delete'));

        // Scenario 3: John from any IP accessing /john-private -> full access
        $this->assertTrue($this->pathAcl->checkPermission($john, '203.0.113.1', '/john-private', 'read'));
        $this->assertTrue($this->pathAcl->checkPermission($john, '203.0.113.1', '/john-private', 'write'));
        $this->assertTrue($this->pathAcl->checkPermission($john, '203.0.113.1', '/john-private', 'upload'));
        $this->assertTrue($this->pathAcl->checkPermission($john, '192.168.1.50', '/john-private', 'delete'));

        // Scenario 4: John from any IP accessing /restricted -> denied
        $this->assertFalse($this->pathAcl->checkPermission($john, '192.168.1.50', '/restricted', 'read'));
        $this->assertFalse($this->pathAcl->checkPermission($john, '10.8.0.50', '/restricted', 'write'));
        $this->assertFalse($this->pathAcl->checkPermission($john, '203.0.113.1', '/restricted', 'delete'));
    }

    // ========== Scenario: Nested Path Inheritance ==========

    public function testNestedPathInheritanceWithOverrides()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/projects' => [
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
                '/projects/confidential' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['@admins'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'delete'],
                            'priority' => 10,
                            'override_inherited' => true,
                        ],
                    ],
                ],
            ],
            'groups' => [
                'admins' => ['alice@example.com'],
            ],
        ];

        $this->pathAcl->init($config);

        $john = $this->createUser('john@example.com');
        $alice = $this->createUser('alice@example.com');

        // John can read /projects
        $this->assertTrue($this->pathAcl->checkPermission($john, '10.0.0.1', '/projects', 'read'));

        // John can read /projects/public (inherits from parent)
        $this->assertTrue($this->pathAcl->checkPermission($john, '10.0.0.1', '/projects/public/file.txt', 'read'));

        // John CANNOT read /projects/confidential (override_inherited)
        $this->assertFalse($this->pathAcl->checkPermission($john, '10.0.0.1', '/projects/confidential', 'read'));

        // Alice CAN read /projects/confidential
        $this->assertTrue($this->pathAcl->checkPermission($alice, '10.0.0.1', '/projects/confidential', 'read'));
        $this->assertTrue($this->pathAcl->checkPermission($alice, '10.0.0.1', '/projects/confidential', 'write'));
    }

    // ========== Scenario: Complex Group-based Permissions ==========

    public function testComplexGroupBasedPermissions()
    {
        $config = [
            'enabled' => true,
            'groups' => [
                'developers' => ['john@example.com', 'jane@example.com'],
                'qa-team' => ['bob@example.com', 'jane@example.com'],
                'managers' => ['alice@example.com'],
            ],
            'path_rules' => [
                '/development' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['@developers'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/testing' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['@qa-team'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/reports' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['@managers'],
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

        $john = $this->createUser('john@example.com');
        $jane = $this->createUser('jane@example.com');
        $bob = $this->createUser('bob@example.com');
        $alice = $this->createUser('alice@example.com');

        // John (developer) can access /development
        $this->assertTrue($this->pathAcl->checkPermission($john, '10.0.0.1', '/development', 'write'));
        $this->assertFalse($this->pathAcl->checkPermission($john, '10.0.0.1', '/testing', 'read'));
        $this->assertFalse($this->pathAcl->checkPermission($john, '10.0.0.1', '/reports', 'read'));

        // Jane (developer AND qa-team) can access both /development and /testing
        $this->assertTrue($this->pathAcl->checkPermission($jane, '10.0.0.1', '/development', 'write'));
        $this->assertTrue($this->pathAcl->checkPermission($jane, '10.0.0.1', '/testing', 'write'));
        $this->assertFalse($this->pathAcl->checkPermission($jane, '10.0.0.1', '/reports', 'read'));

        // Bob (qa-team) can only access /testing
        $this->assertFalse($this->pathAcl->checkPermission($bob, '10.0.0.1', '/development', 'read'));
        $this->assertTrue($this->pathAcl->checkPermission($bob, '10.0.0.1', '/testing', 'write'));
        $this->assertFalse($this->pathAcl->checkPermission($bob, '10.0.0.1', '/reports', 'read'));

        // Alice (manager) can read /reports
        $this->assertFalse($this->pathAcl->checkPermission($alice, '10.0.0.1', '/development', 'read'));
        $this->assertFalse($this->pathAcl->checkPermission($alice, '10.0.0.1', '/testing', 'read'));
        $this->assertTrue($this->pathAcl->checkPermission($alice, '10.0.0.1', '/reports', 'read'));
    }

    // ========== Scenario: IP-based Access with CIDR Ranges ==========

    public function testIPBasedAccessWithCIDRRanges()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/internal' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['*'],
                            'ip_inclusions' => ['192.168.0.0/16', '10.0.0.0/8'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/public' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['*'],
                            'ip_inclusions' => ['*'],
                            'ip_exclusions' => ['192.0.2.0/24'],
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

        // Access /internal from internal IPs
        $this->assertTrue($this->pathAcl->checkPermission($user, '192.168.1.50', '/internal', 'read'));
        $this->assertTrue($this->pathAcl->checkPermission($user, '10.5.5.5', '/internal', 'write'));

        // Access /internal from external IP (denied)
        $this->assertFalse($this->pathAcl->checkPermission($user, '203.0.113.1', '/internal', 'read'));

        // Access /public from most IPs (allowed)
        $this->assertTrue($this->pathAcl->checkPermission($user, '203.0.113.1', '/public', 'read'));
        $this->assertTrue($this->pathAcl->checkPermission($user, '192.168.1.50', '/public', 'read'));

        // Access /public from denied range (blocked)
        $this->assertFalse($this->pathAcl->checkPermission($user, '192.0.2.50', '/public', 'read'));
    }

    // ========== Scenario: Permission Merging from Multiple Rules ==========

    public function testPermissionMergingFromMultipleRules()
    {
        $config = [
            'enabled' => true,
            'groups' => [
                'readers' => ['john@example.com'],
                'writers' => ['john@example.com'],
            ],
            'path_rules' => [
                '/shared' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['@readers'],
                            'ip_inclusions' => [],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                        [
                            'users' => ['@writers'],
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

        $john = $this->createUser('john@example.com');

        // John should have all permissions from both groups
        $perms = $this->pathAcl->getEffectivePermissions($john, '10.0.0.1', '/shared');

        $this->assertContains('read', $perms);
        $this->assertContains('write', $perms);
        $this->assertContains('upload', $perms);
    }

    // ========== Scenario: Priority-based Rule Selection ==========

    public function testPriorityBasedRuleSelection()
    {
        $config = [
            'enabled' => true,
            'path_rules' => [
                '/documents' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['john@example.com'],
                            'ip_inclusions' => ['192.168.1.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'delete'],
                            'priority' => 100,
                            'override_inherited' => false,
                        ],
                        [
                            'users' => ['john@example.com'],
                            'ip_inclusions' => ['10.0.0.0/8'],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 50,
                            'override_inherited' => false,
                        ],
                        [
                            'users' => ['john@example.com'],
                            'ip_inclusions' => ['*'],
                            'ip_exclusions' => [],
                            'permissions' => [],
                            'priority' => 1,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $john = $this->createUser('john@example.com');

        // From 192.168.1.50 - highest priority rule (100)
        $perms1 = $this->pathAcl->getEffectivePermissions($john, '192.168.1.50', '/documents');
        $this->assertContains('delete', $perms1);

        // From 10.0.0.1 - medium priority rule (50)
        $perms2 = $this->pathAcl->getEffectivePermissions($john, '10.0.0.1', '/documents');
        $this->assertContains('read', $perms2);
        $this->assertNotContains('delete', $perms2);

        // From 203.0.113.1 - lowest priority rule (1)
        $perms3 = $this->pathAcl->getEffectivePermissions($john, '203.0.113.1', '/documents');
        $this->assertEmpty($perms3);
    }

    // ========== Scenario: Real-world Corporate File Server ==========

    public function testCorporateFileServerScenario()
    {
        $config = [
            'enabled' => true,
            'groups' => [
                'hr' => ['hr-manager@example.com', 'hr-assistant@example.com'],
                'finance' => ['cfo@example.com', 'accountant@example.com'],
                'employees' => ['john@example.com', 'jane@example.com'],
            ],
            'path_rules' => [
                '/' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['*'],
                            'ip_inclusions' => ['192.168.0.0/16'],
                            'ip_exclusions' => [],
                            'permissions' => ['read'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/hr/confidential' => [
                    'inherit' => false,
                    'rules' => [
                        [
                            'users' => ['@hr'],
                            'ip_inclusions' => ['192.168.1.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload', 'delete'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/finance/reports' => [
                    'inherit' => false,
                    'rules' => [
                        [
                            'users' => ['@finance'],
                            'ip_inclusions' => ['192.168.2.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
                '/shared/documents' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['@employees'],
                            'ip_inclusions' => ['192.168.0.0/16'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write', 'upload'],
                            'priority' => 0,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $hrManager = $this->createUser('hr-manager@example.com');
        $cfo = $this->createUser('cfo@example.com');
        $employee = $this->createUser('john@example.com');

        // HR Manager can access HR confidential from HR network
        $this->assertTrue($this->pathAcl->checkPermission($hrManager, '192.168.1.10', '/hr/confidential', 'delete'));

        // HR Manager CANNOT access HR confidential from Finance network
        $this->assertFalse($this->pathAcl->checkPermission($hrManager, '192.168.2.10', '/hr/confidential', 'read'));

        // CFO can access Finance reports from Finance network
        $this->assertTrue($this->pathAcl->checkPermission($cfo, '192.168.2.10', '/finance/reports', 'write'));

        // CFO CANNOT access HR confidential
        $this->assertFalse($this->pathAcl->checkPermission($cfo, '192.168.1.10', '/hr/confidential', 'read'));

        // Regular employee can access shared documents
        $this->assertTrue($this->pathAcl->checkPermission($employee, '192.168.5.10', '/shared/documents', 'write'));

        // Regular employee CANNOT access HR or Finance folders
        $this->assertFalse($this->pathAcl->checkPermission($employee, '192.168.1.10', '/hr/confidential', 'read'));
        $this->assertFalse($this->pathAcl->checkPermission($employee, '192.168.2.10', '/finance/reports', 'read'));
    }

    // ========== Scenario: Permission Explanation for Debugging ==========

    public function testPermissionExplanationForComplexScenario()
    {
        $config = [
            'enabled' => true,
            'groups' => [
                'developers' => ['john@example.com'],
            ],
            'path_rules' => [
                '/projects' => [
                    'inherit' => true,
                    'rules' => [
                        [
                            'users' => ['@developers'],
                            'ip_inclusions' => ['192.168.1.0/24'],
                            'ip_exclusions' => [],
                            'permissions' => ['read', 'write'],
                            'priority' => 10,
                            'override_inherited' => false,
                        ],
                    ],
                ],
            ],
        ];

        $this->pathAcl->init($config);

        $john = $this->createUser('john@example.com');

        // Get explanation for allowed access
        $explanation = $this->pathAcl->explainPermission($john, '192.168.1.50', '/projects/alpha/file.txt', 'read');

        $this->assertTrue($explanation['allowed']);
        $this->assertNotEmpty($explanation['matched_rules']);
        $this->assertContains('read', $explanation['effective_permissions']);
        $this->assertEquals('read', $explanation['requested_permission']);

        // Get explanation for denied access
        $explanation2 = $this->pathAcl->explainPermission($john, '10.0.0.1', '/projects/alpha/file.txt', 'read');

        $this->assertFalse($explanation2['allowed']);
    }
}
