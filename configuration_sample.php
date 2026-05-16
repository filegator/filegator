<?php

return [
    'public_path' => APP_PUBLIC_PATH,
    'public_dir' => APP_PUBLIC_DIR,
    'overwrite_on_upload' => false,
    'timezone' => 'UTC', // https://www.php.net/manual/en/timezones.php
    'download_inline' => ['pdf'], // download inline in the browser, array of extensions, use * for all
    'lockout_attempts' => 5, // max failed login attempts before ip lockout
    'lockout_timeout' => 15, // ip lockout timeout in seconds

    'mfa_required_for_admins' => true,           // admins must enroll TOTP on first login
    'password_reset_token_ttl' => 3600,          // seconds the reset link stays valid
    'password_reset_max_per_hour_per_ip' => 3,   // throttle per IP
    'password_reset_max_per_day_per_email' => 3, // throttle per email

    'frontend_config' => [
        'app_name' => 'FileGator',
        'app_version' => APP_VERSION,
        'language' => 'english',
        'logo' => 'https://filegator.io/filegator_logo.svg',
        'upload_max_size' => 100 * 1024 * 1024, // 100MB
        'upload_chunk_size' => 1 * 1024 * 1024, // 1MB
        'upload_simultaneous' => 3,
        'default_archive_name' => 'archive.zip',
        'editable' => ['.txt', '.css', '.js', '.ts', '.html', '.php', '.json', '.md'],
        'date_format' => 'YY/MM/DD hh:mm:ss', // see: https://momentjs.com/docs/#/displaying/format/
        'guest_redirection' => '', // useful for external auth adapters
        'search_simultaneous' => 5,
        'filter_entries' => [],
        'pagination' => ['', 5, 10, 15],
    ],

    'services' => [
        'Filegator\Services\Logger\LoggerInterface' => [
            'handler' => '\Filegator\Services\Logger\Adapters\MonoLogger',
            'config' => [
                'monolog_handlers' => [
                    function () {
                        return new \Monolog\Handler\StreamHandler(
                            __DIR__.'/private/logs/app.log',
                            \Monolog\Logger::DEBUG
                        );
                    },
                ],
            ],
        ],
        'Filegator\Services\Session\SessionStorageInterface' => [
            'handler' => '\Filegator\Services\Session\Adapters\SessionStorage',
            'config' => [
                'handler' => function () {
                    $save_path = null; // use default system path
                    //$save_path = __DIR__.'/private/sessions';
                    $handler = new \Symfony\Component\HttpFoundation\Session\Storage\Handler\NativeFileSessionHandler($save_path);

                    return new \Symfony\Component\HttpFoundation\Session\Storage\NativeSessionStorage([
                            "cookie_samesite" => "Lax",
                            "cookie_secure" => null,
                            "cookie_httponly" => true,
                        ], $handler);
                },
            ],
        ],
        'Filegator\Services\Cors\Cors' => [
            'handler' => '\Filegator\Services\Cors\Cors',
            'config' => [
                'enabled' => APP_ENV == 'production' ? false : true,
            ],
        ],
        'Filegator\Services\Tmpfs\TmpfsInterface' => [
            'handler' => '\Filegator\Services\Tmpfs\Adapters\Tmpfs',
            'config' => [
                'path' => __DIR__.'/private/tmp/',
                'gc_probability_perc' => 10,
                'gc_older_than' => 60 * 60 * 24 * 2, // 2 days
            ],
        ],
        'Filegator\Services\Security\Security' => [
            'handler' => '\Filegator\Services\Security\Security',
            'config' => [
                'csrf_protection' => true,
                'csrf_key' => "123456", // randomize this
                'csrf_exempt_paths' => ['/password/forgot', '/password/reset/validate', '/password/reset'],
                'ip_allowlist' => [],
                'ip_denylist' => [],
                'allow_insecure_overlays' => false,
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
                'config' => [],
                'adapter' => function () {
                    return new \League\Flysystem\Adapter\Local(
                        __DIR__.'/repository'
                    );
                },
            ],
        ],
        'Filegator\Services\Archiver\ArchiverInterface' => [
            'handler' => '\Filegator\Services\Archiver\Adapters\ZipArchiver',
            'config' => [],
        ],
        'Filegator\Services\Auth\AuthInterface' => [
            'handler' => '\Filegator\Services\Auth\Adapters\JsonFile',
            'config' => [
                'file' => __DIR__.'/private/users.json',
            ],
        ],
        // Mailer / Mfa / PasswordReset must come BEFORE Router. Router::init
        // dispatches the route immediately, so any controller method that
        // type-hints these services (e.g. ViewController::getFrontendConfig)
        // would otherwise fail to resolve them.
        'Filegator\Services\Mailer\MailerInterface' => [
            'handler' => '\Filegator\Services\Mailer\Adapters\SymfonyMailer',
            'config' => [
                // Symfony Mailer DSN. Use 'null://null' to disable sending (feature stays hidden).
                // Examples:
                //   'smtp://user:pass@smtp.example.com:587?encryption=tls'
                //   'sendmail://default'
                'dsn' => 'null://null',
                'from_email' => 'no-reply@example.com',
                'from_name' => 'FileGator',
                // Hard cap (seconds) we force on every SMTP socket so a slow / unreachable
                // mail server cannot hang a PHP-FPM worker for PHP's default_socket_timeout
                // (60s by default). Appended to the DSN automatically if not already set,
                // and also enforced via a per-request default_socket_timeout clamp. Tune up
                // for very slow servers; do not set to 0.
                'timeout' => 5,
            ],
        ],
        'Filegator\Services\Mfa\MfaService' => [
            'handler' => '\Filegator\Services\Mfa\MfaService',
            'config' => [
                'issuer' => 'FileGator',
            ],
        ],
        'Filegator\Services\PasswordReset\PasswordResetService' => [
            'handler' => '\Filegator\Services\PasswordReset\PasswordResetService',
            'config' => [
                'token_file' => __DIR__.'/private/password_resets.json',
                'reset_subject' => 'Reset your FileGator password',
                // REQUIRED for password reset to work. Must be the full public URL
                // operators want reset links to point to (scheme + host + base path).
                // We deliberately do NOT derive this from the request Host header,
                // because doing so allows an attacker to send victims reset links
                // pointing at an attacker-controlled host.
                // Set to null (default) to disable the password-reset feature.
                'reset_url_base' => null, // e.g. 'https://files.example.com/'
            ],
        ],
        'Filegator\Services\Router\Router' => [
            'handler' => '\Filegator\Services\Router\Router',
            'config' => [
                'query_param' => 'r',
                'routes_file' => __DIR__.'/backend/Controllers/routes.php',
            ],
        ],
    ],
];
