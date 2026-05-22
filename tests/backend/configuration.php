<?php

return [
    'public_path' => '',
    'public_dir' => __DIR__.'/../../dist',
    'overwrite_on_upload' => false,
    'timezone' => 'UTC', // https://www.php.net/manual/en/timezones.php
    'download_inline' => ['pdf'],

    'frontend_config' => [
        'app_name' => 'FileGator',
        'language' => 'english',
        'logo' => 'https://via.placeholder.com/263x55.png',
        'upload_max_size' => 2 * 1024 * 1024,
        'upload_chunk_size' => 1 * 1024 * 1024,
        'upload_simultaneous' => 3,
        'default_archive_name' => 'archive.zip',
        'editable' => ['.txt', '.css', '.js', '.ts', '.html', '.php', '.json', '.md'],
        'date_format' => 'YY/MM/DD hh:mm:ss',
        'guest_redirection' => '', // useful for external auth adapters
    ],

    'mfa_required_for_admins' => false, // tests override per-case
    'password_reset_token_ttl' => 3600,
    'password_reset_max_per_hour_per_ip' => 3,
    'password_reset_max_per_day_per_email' => 3,

    'services' => [
        'Filegator\Services\Logger\LoggerInterface' => [
            'handler' => '\Filegator\Services\Logger\Adapters\MonoLogger',
            'config' => [
                'monolog_handlers' => [
                    function () {
                        return new \Monolog\Handler\NullHandler();
                    },
                ],
            ],
        ],
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'handler' => function () {
                    return new \Symfony\Component\HttpFoundation\Session\Storage\MockFileSessionStorage();
                },
            ],
        ],
        'Filegator\Services\Security\Security' => [
            'handler' => '\Filegator\Services\Security\Security',
            'config' => [
                // CSRF is OFF in the default test config so existing tests do not
                // need to plumb tokens through every helper. The exempt-path
                // contract is exercised explicitly by tests that overrideConfig
                // to flip csrf_protection on.
                'csrf_protection' => false,
                'csrf_key' => 'test-csrf-key',
                'csrf_exempt_paths' => ['/password/forgot', '/password/reset/validate', '/password/reset'],
                'ip_allowlist' => [],
                'ip_denylist' => [],
                'allow_insecure_overlays' => true,
            ],
        ],
        'Filegator\Services\Tmpfs\TmpfsInterface' => [
            'handler' => '\Filegator\Services\Tmpfs\Adapters\Tmpfs',
            'config' => [
                'path' => TEST_TMP_PATH,
                'gc_probability_perc' => 10,
                'gc_older_than' => 60 * 60 * 24 * 2, // 2 days
            ],
        ],
        'Filegator\Services\View\ViewInterface' => [
            'handler' => '\Filegator\Services\View\Adapters\Vuejs',
            'config' => [
                'add_to_head' => '',
                'add_to_body' => '',
            ],
        ],
        'Filegator\Services\Storage\Filesystem' => [
            'handler' => '\Filegator\Services\Storage\Filesystem',
            'config' => [
                'separator' => '/',
                'adapter' => function () {
                    return new \League\Flysystem\Adapter\Local(
                        TEST_REPOSITORY
                    );
                },
            ],
        ],
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Tests\MockUsers',
        ],
        'Filegator\Services\Archiver\ArchiverInterface' => [
            'handler' => '\Filegator\Services\Archiver\Adapters\ZipArchiver',
            'config' => [],
        ],
        // Mailer / Mfa / PasswordReset must come BEFORE Router. Router::init
        // dispatches the route immediately, so any controller method (e.g.
        // ViewController::getFrontendConfig) that type-hints these services
        // would otherwise fail to resolve them.
        'Filegator\Services\Mailer\MailerInterface' => [
            'handler' => '\Tests\Fakes\InMemoryMailer',
            'config' => [],
        ],
        'Filegator\Services\Mfa\MfaService' => [
            'handler' => '\Filegator\Services\Mfa\MfaService',
            'config' => [
                'issuer' => 'FileGatorTest',
            ],
        ],
        'Filegator\Services\PasswordReset\PasswordResetService' => [
            'handler' => '\Filegator\Services\PasswordReset\PasswordResetService',
            'config' => [
                'token_file' => TEST_TMP_PATH.'password_resets.json',
                'reset_subject' => 'Reset your FileGator password',
                'reset_url_base' => 'https://files.example.com/',
            ],
        ],
        'Filegator\Services\Audit\AuditMailer' => [
            'handler' => '\Filegator\Services\Audit\AuditMailer',
            'config' => [
                'recipient' => 'audit@example.com',
                'from_email' => 'audit-from@example.com',
                'from_name' => 'Test Audit',
                'app_label' => 'Test Portal',
                'enabled' => true,
            ],
        ],
        'Filegator\Services\Router\Router' => [
            'handler' => '\Filegator\Services\Router\Router',
            'config' => [
                'query_param' => 'r',
                'routes_file' => __DIR__.'/../../backend/Controllers/routes.php',
            ],
        ],
    ],
];
