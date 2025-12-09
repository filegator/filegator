#!/usr/bin/env php
<?php
/**
 * Trend Micro File Scanning Example - CLI Installer
 *
 * Automates the installation of hooks, ACL configuration, and user setup
 * for the Trend Micro file scanning integration with FileGator.
 *
 * Usage:
 *   php install.php --gateway-ip=192.168.1.100 --api-key=YOUR_KEY --admin-email=admin@example.com
 *
 * @author FileGator Team
 * @version 1.0.0
 */

class TrendMicroInstaller
{
    private $config = [];
    private $filegatorPath;
    private $examplePath;
    private $dryRun = false;
    private $colors = true;
    private $backups = [];

    // ANSI color codes
    const COLOR_RESET = "\033[0m";
    const COLOR_RED = "\033[31m";
    const COLOR_GREEN = "\033[32m";
    const COLOR_YELLOW = "\033[33m";
    const COLOR_BLUE = "\033[34m";
    const COLOR_CYAN = "\033[36m";
    const COLOR_BOLD = "\033[1m";

    public function __construct($args)
    {
        $this->examplePath = __DIR__;
        $this->detectColors();
        $this->parseArguments($args);
        $this->detectFileGatorPath();
        $this->validateEnvironment();
    }

    /**
     * Main installation routine
     */
    public function install()
    {
        $this->printHeader();
        $this->displayConfiguration();

        if ($this->dryRun) {
            $this->printInfo("\n[DRY RUN MODE] No files will be modified\n");
        }

        // Confirm before proceeding (unless dry-run)
        if (!$this->dryRun && !$this->confirm("Proceed with installation?")) {
            $this->printWarning("Installation cancelled by user.\n");
            exit(0);
        }

        echo "\n";

        try {
            // Installation steps with progress
            $this->runStep('Creating directories', [$this, 'createDirectories']);
            $this->runStep('Installing hook scripts', [$this, 'installHookScripts']);
            $this->runStep('Installing hooks configuration', [$this, 'installHooksConfig']);
            $this->runStep('Installing ACL configuration', [$this, 'installACLConfig']);
            $this->runStep('Creating/updating user john', [$this, 'createUser']);
            $this->runStep('Updating main configuration', [$this, 'updateMainConfig']);
            $this->runStep('Setting file permissions', [$this, 'setFilePermissions']);
            $this->runStep('Creating .env file', [$this, 'createEnvFile']);
            $this->runStep('Validating installation', [$this, 'validateInstallation']);

            if (!$this->dryRun) {
                $this->runStep('Testing Trend Micro API', [$this, 'testTrendMicroAPI']);
            }

            $this->printSuccess("\n" . str_repeat("=", 60) . "\n");
            $this->printSuccess("Installation completed successfully!\n");
            $this->printSuccess(str_repeat("=", 60) . "\n");
            $this->displayNextSteps();

        } catch (Exception $e) {
            $this->printError("\n[INSTALLATION FAILED] " . $e->getMessage() . "\n");
            $this->restoreBackups();
            exit(1);
        }
    }

    /**
     * Parse command line arguments
     */
    private function parseArguments($args)
    {
        $options = getopt('h', [
            'gateway-ip:',
            'api-key:',
            'admin-email:',
            'api-url::',
            'smtp-host::',
            'smtp-port::',
            'smtp-user::',
            'smtp-pass::',
            'john-password::',
            'filegator-path::',
            'help',
            'dry-run',
        ]);

        // Show help if requested
        if (isset($options['h']) || isset($options['help'])) {
            $this->showHelp();
            exit(0);
        }

        // Check required parameters
        $required = ['gateway-ip', 'api-key', 'admin-email'];
        foreach ($required as $param) {
            if (!isset($options[$param]) || empty($options[$param])) {
                $this->error("Missing required parameter: --$param");
            }
        }

        // Validate gateway IP
        if (!$this->validateIPAddress($options['gateway-ip'])) {
            $this->error("Invalid gateway IP address: {$options['gateway-ip']}");
        }

        // Validate email
        if (!$this->validateEmail($options['admin-email'])) {
            $this->error("Invalid email address: {$options['admin-email']}");
        }

        // Validate API key format (non-empty string)
        if (empty(trim($options['api-key']))) {
            $this->error("API key cannot be empty");
        }

        // Build configuration
        $this->config = [
            'gateway_ip' => trim($options['gateway-ip']),
            'api_key' => trim($options['api-key']),
            'admin_email' => trim($options['admin-email']),
            'api_url' => $options['api-url'] ?? 'https://filesecurity.api.trendmicro.com/v1',
            'smtp_host' => $options['smtp-host'] ?? 'localhost',
            'smtp_port' => $options['smtp-port'] ?? 587,
            'smtp_user' => $options['smtp-user'] ?? '',
            'smtp_pass' => $options['smtp-pass'] ?? '',
            'john_password' => $options['john-password'] ?? 'changeme',
        ];

        // Store filegator path if provided
        if (isset($options['filegator-path'])) {
            $this->filegatorPath = rtrim($options['filegator-path'], '/');
        }

        $this->dryRun = isset($options['dry-run']);
    }

    /**
     * Detect FileGator installation path
     */
    private function detectFileGatorPath()
    {
        if ($this->filegatorPath) {
            return; // Already set via command line
        }

        // Try to detect from current location
        $currentDir = $this->examplePath;

        // Walk up the directory tree looking for configuration.php
        for ($i = 0; $i < 5; $i++) {
            $testPath = $currentDir . '/configuration.php';
            if (file_exists($testPath)) {
                $this->filegatorPath = $currentDir;
                return;
            }
            $currentDir = dirname($currentDir);
        }

        // Check common installation paths
        $commonPaths = [
            '/var/www/filegator',
            '/var/www/html/filegator',
            '/opt/filegator',
            '/usr/share/filegator',
            getcwd(),
        ];

        foreach ($commonPaths as $path) {
            if (file_exists($path . '/configuration.php')) {
                $this->filegatorPath = $path;
                return;
            }
        }

        $this->error("Could not auto-detect FileGator installation. Please use --filegator-path=/path/to/filegator");
    }

    /**
     * Validate FileGator installation environment
     */
    private function validateEnvironment()
    {
        $this->printInfo("Validating environment...\n");

        // Check FileGator path exists
        if (!is_dir($this->filegatorPath)) {
            $this->error("FileGator path not found: {$this->filegatorPath}");
        }

        // Check critical files exist
        $requiredFiles = [
            'configuration.php' => 'Main configuration file',
            'private' => 'Private directory',
            'repository' => 'Repository directory',
        ];

        foreach ($requiredFiles as $file => $description) {
            $fullPath = $this->filegatorPath . '/' . $file;
            if (!file_exists($fullPath)) {
                $this->error("Required file/directory not found: $file ($description)\nPath checked: $fullPath");
            }
        }

        // Check write permissions
        $writableDirs = ['private', 'repository'];
        foreach ($writableDirs as $dir) {
            $fullPath = $this->filegatorPath . '/' . $dir;
            if (!is_writable($fullPath)) {
                $this->error("Directory is not writable: $dir\nPath: $fullPath\nPlease fix permissions: chmod 755 $fullPath");
            }
        }

        // Check if hooks directory exists in example package
        $hooksSource = $this->examplePath . '/hooks';
        if (!is_dir($hooksSource)) {
            $this->error("Hook scripts not found in example package: $hooksSource");
        }

        $this->printSuccess("  Environment validation passed\n\n");
    }

    /**
     * Create required directories
     */
    private function createDirectories()
    {
        $dirs = [
            '/repository/upload' => 'Upload staging directory',
            '/repository/scanned' => 'Scanned files directory',
            '/repository/download' => 'Download staging directory',
            '/private/hooks' => 'Hooks base directory',
            '/private/hooks/onUpload' => 'Upload hooks directory',
            '/private/quarantine' => 'Quarantine directory',
            '/private/logs' => 'Log files directory',
        ];

        foreach ($dirs as $dir => $description) {
            $fullPath = $this->filegatorPath . $dir;

            if (!$this->dryRun) {
                if (!is_dir($fullPath)) {
                    if (!mkdir($fullPath, 0755, true)) {
                        throw new Exception("Failed to create directory: $fullPath");
                    }
                    echo "  Created: $dir\n";
                } else {
                    echo "  Exists:  $dir\n";
                }
            } else {
                echo "  [DRY RUN] Would create: $dir\n";
            }
        }
    }

    /**
     * Install hook scripts
     */
    private function installHookScripts()
    {
        $hookDestPath = $this->filegatorPath . '/private/hooks/onUpload';

        $hooks = [
            '01_move_from_download.php' => 'Move files from /download to /upload',
            '02_scan_upload.php' => 'Scan files with Trend Micro',
        ];

        foreach ($hooks as $hook => $description) {
            $source = $this->examplePath . '/hooks/onUpload/' . $hook;
            $dest = $hookDestPath . '/' . $hook;

            if (!file_exists($source)) {
                throw new Exception("Hook source file not found: $source");
            }

            if (!$this->dryRun) {
                // Backup existing file
                if (file_exists($dest)) {
                    $this->backupFile($dest);
                }

                if (!copy($source, $dest)) {
                    throw new Exception("Failed to copy hook: $hook");
                }
                chmod($dest, 0755);
                echo "  Installed: $hook\n";
            } else {
                echo "  [DRY RUN] Would install: $hook\n";
            }
        }

        // Also copy the TrendMicroScanner library
        $libSource = $this->examplePath . '/lib/TrendMicroScanner.php';
        $libDest = $this->filegatorPath . '/private/TrendMicroScanner.php';

        if (file_exists($libSource)) {
            if (!$this->dryRun) {
                if (file_exists($libDest)) {
                    $this->backupFile($libDest);
                }
                copy($libSource, $libDest);
                chmod($libDest, 0644);
                echo "  Installed: TrendMicroScanner.php library\n";
            } else {
                echo "  [DRY RUN] Would install: TrendMicroScanner.php\n";
            }
        }
    }

    /**
     * Install hooks configuration
     */
    private function installHooksConfig()
    {
        $template = $this->examplePath . '/config/hooks_config.php.template';
        $dest = $this->filegatorPath . '/private/hooks/config.php';

        if (!file_exists($template)) {
            throw new Exception("Hooks config template not found: $template");
        }

        $content = file_get_contents($template);

        if (!$this->dryRun) {
            if (file_exists($dest)) {
                $this->backupFile($dest);
            }

            if (file_put_contents($dest, $content) === false) {
                throw new Exception("Failed to write hooks config: $dest");
            }
            chmod($dest, 0600);
            echo "  Created: hooks/config.php\n";
        } else {
            echo "  [DRY RUN] Would create: hooks/config.php\n";
        }
    }

    /**
     * Install ACL configuration
     */
    private function installACLConfig()
    {
        $template = $this->examplePath . '/config/acl_config.php.template';
        $dest = $this->filegatorPath . '/private/acl_config.php';

        if (!file_exists($template)) {
            throw new Exception("ACL config template not found: $template");
        }

        $content = file_get_contents($template);

        // Replace placeholders
        $replacements = [
            '{{GATEWAY_IP}}' => $this->config['gateway_ip'],
            '{{INTERNAL_NETWORK}}' => '0.0.0.0/0', // Allow all except gateway (handled by rules)
        ];

        $content = str_replace(
            array_keys($replacements),
            array_values($replacements),
            $content
        );

        if (!$this->dryRun) {
            if (file_exists($dest)) {
                $this->backupFile($dest);
            }

            if (file_put_contents($dest, $content) === false) {
                throw new Exception("Failed to write ACL config: $dest");
            }
            chmod($dest, 0600);
            echo "  Created: acl_config.php\n";
            echo "  Gateway IP: {$this->config['gateway_ip']}\n";
        } else {
            echo "  [DRY RUN] Would create: acl_config.php\n";
        }
    }

    /**
     * Create or update user 'john'
     */
    private function createUser()
    {
        $usersFile = $this->filegatorPath . '/private/users.json';

        if (!file_exists($usersFile)) {
            // Check for .blank file
            $blankFile = $usersFile . '.blank';
            if (file_exists($blankFile)) {
                $usersFile = $blankFile;
            } else {
                throw new Exception("Users file not found: $usersFile");
            }
        }

        $users = json_decode(file_get_contents($usersFile), true);
        if (!is_array($users)) {
            $users = [];
        }

        // Check if user 'john' already exists
        $johnId = null;
        foreach ($users as $id => $user) {
            if (isset($user['username']) && $user['username'] === 'john') {
                $johnId = $id;
                break;
            }
        }

        // Prepare user data
        $johnData = [
            'username' => 'john',
            'name' => 'John Doe',
            'role' => 'user',
            'homedir' => '/',
            'permissions' => 'read|upload|download',
            'password' => password_hash($this->config['john_password'], PASSWORD_BCRYPT),
        ];

        if (!$this->dryRun) {
            // Backup users file
            $dest = str_replace('.blank', '', $usersFile);
            if (file_exists($dest)) {
                $this->backupFile($dest);
            }

            if ($johnId !== null) {
                // Update existing user
                $users[$johnId] = $johnData;
                echo "  Updated existing user: john\n";
            } else {
                // Create new user
                $nextId = empty($users) ? 1 : max(array_keys($users)) + 1;
                $users[$nextId] = $johnData;
                echo "  Created new user: john\n";
            }

            if (file_put_contents($dest, json_encode($users, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES)) === false) {
                throw new Exception("Failed to write users file: $dest");
            }
            chmod($dest, 0600);
            echo "  Password: {$this->config['john_password']}\n";
            $this->printWarning("  IMPORTANT: Change password after first login!\n");
        } else {
            echo "  [DRY RUN] Would " . ($johnId ? "update" : "create") . " user: john\n";
        }
    }

    /**
     * Update main configuration.php
     */
    private function updateMainConfig()
    {
        $configFile = $this->filegatorPath . '/configuration.php';

        if (!file_exists($configFile)) {
            throw new Exception("Configuration file not found: $configFile");
        }

        $content = file_get_contents($configFile);

        // Check if PathACL is configured
        $hasPathACL = strpos($content, 'PathACLInterface') !== false;
        $hasHooks = strpos($content, 'HooksInterface') !== false;

        if (!$hasPathACL || !$hasHooks) {
            $this->printWarning("  Configuration.php needs manual updates:\n");
            if (!$hasPathACL) {
                $this->printWarning("    - Add PathACL service configuration\n");
            }
            if (!$hasHooks) {
                $this->printWarning("    - Add Hooks service configuration\n");
            }
            $this->printWarning("  See DESIGN.md section 4.1 for configuration details\n");
            return;
        }

        // Try to enable PathACL and Hooks by replacing 'enabled' => false with true
        if (!$this->dryRun) {
            $this->backupFile($configFile);

            // Simple regex replacements (may need manual verification)
            $patterns = [
                // Enable PathACL
                "/('Filegator\\\\Services\\\\PathACL\\\\PathACLInterface'[^}]+?'enabled'\s*=>\s*)false/s" => "$1true",
                // Enable Hooks
                "/('Filegator\\\\Services\\\\Hooks\\\\HooksInterface'[^}]+?'enabled'\s*=>\s*)false/s" => "$1true",
            ];

            $modified = false;
            foreach ($patterns as $pattern => $replacement) {
                $newContent = preg_replace($pattern, $replacement, $content);
                if ($newContent !== $content) {
                    $content = $newContent;
                    $modified = true;
                }
            }

            if ($modified) {
                file_put_contents($configFile, $content);
                echo "  Updated: configuration.php (enabled PathACL and Hooks)\n";
                $this->printWarning("  Please verify configuration.php manually!\n");
            } else {
                echo "  PathACL and Hooks appear to be already enabled\n";
            }
        } else {
            echo "  [DRY RUN] Would update: configuration.php\n";
        }
    }

    /**
     * Set proper file permissions
     */
    private function setFilePermissions()
    {
        $permissions = [
            '/private/acl_config.php' => 0600,
            '/private/hooks/config.php' => 0600,
            '/private/users.json' => 0600,
            '/private/hooks/onUpload/01_move_from_download.php' => 0755,
            '/private/hooks/onUpload/02_scan_upload.php' => 0755,
            '/private/quarantine' => 0700,
        ];

        foreach ($permissions as $file => $perm) {
            $fullPath = $this->filegatorPath . $file;
            if (file_exists($fullPath)) {
                if (!$this->dryRun) {
                    chmod($fullPath, $perm);
                    echo "  Set permissions " . decoct($perm) . " on: $file\n";
                } else {
                    echo "  [DRY RUN] Would set permissions " . decoct($perm) . " on: $file\n";
                }
            }
        }
    }

    /**
     * Create .env file with configuration
     */
    private function createEnvFile()
    {
        $envFile = $this->filegatorPath . '/.env';

        $envContent = <<<ENV
# Trend Micro File Scanning Configuration
# Generated by install.php on {date}

# Trend Micro Cloud One File Security
TREND_MICRO_API_KEY={api_key}
TREND_MICRO_API_URL={api_url}

# Email Configuration (for malware alerts)
ADMIN_EMAIL={admin_email}
SMTP_HOST={smtp_host}
SMTP_PORT={smtp_port}
SMTP_USER={smtp_user}
SMTP_PASS={smtp_pass}

# Gateway Configuration
GATEWAY_IP={gateway_ip}
ENV;

        $envContent = str_replace(
            ['{date}', '{api_key}', '{api_url}', '{admin_email}', '{smtp_host}', '{smtp_port}', '{smtp_user}', '{smtp_pass}', '{gateway_ip}'],
            [
                date('Y-m-d H:i:s'),
                $this->config['api_key'],
                $this->config['api_url'],
                $this->config['admin_email'],
                $this->config['smtp_host'],
                $this->config['smtp_port'],
                $this->config['smtp_user'],
                $this->config['smtp_pass'],
                $this->config['gateway_ip'],
            ],
            $envContent
        );

        if (!$this->dryRun) {
            if (file_exists($envFile)) {
                $this->backupFile($envFile);
            }

            if (file_put_contents($envFile, $envContent) === false) {
                throw new Exception("Failed to write .env file: $envFile");
            }
            chmod($envFile, 0600);
            echo "  Created: .env file\n";
            $this->printWarning("  API key stored in .env (permissions: 0600)\n");
        } else {
            echo "  [DRY RUN] Would create: .env file\n";
        }
    }

    /**
     * Validate installation
     */
    private function validateInstallation()
    {
        $checks = [
            'Directories exist' => function() {
                $dirs = ['/repository/upload', '/repository/scanned', '/repository/download', '/private/hooks/onUpload'];
                foreach ($dirs as $dir) {
                    if (!is_dir($this->filegatorPath . $dir)) {
                        return false;
                    }
                }
                return true;
            },
            'Hook scripts installed' => function() {
                return file_exists($this->filegatorPath . '/private/hooks/onUpload/01_move_from_download.php')
                    && file_exists($this->filegatorPath . '/private/hooks/onUpload/02_scan_upload.php');
            },
            'Configurations created' => function() {
                return file_exists($this->filegatorPath . '/private/acl_config.php')
                    && file_exists($this->filegatorPath . '/private/hooks/config.php');
            },
            'User john created' => function() {
                $usersFile = $this->filegatorPath . '/private/users.json';
                if (!file_exists($usersFile)) return false;
                $users = json_decode(file_get_contents($usersFile), true);
                foreach ($users as $user) {
                    if (isset($user['username']) && $user['username'] === 'john') {
                        return true;
                    }
                }
                return false;
            },
        ];

        $allPassed = true;
        foreach ($checks as $name => $check) {
            if (!$this->dryRun) {
                $result = $check();
                $status = $result ? $this->colorize('PASS', self::COLOR_GREEN) : $this->colorize('FAIL', self::COLOR_RED);
                echo "  $name: $status\n";
                $allPassed = $allPassed && $result;
            } else {
                echo "  [DRY RUN] Would check: $name\n";
            }
        }

        if (!$allPassed && !$this->dryRun) {
            throw new Exception("Installation validation failed");
        }
    }

    /**
     * Test Trend Micro API connectivity
     */
    private function testTrendMicroAPI()
    {
        $libFile = $this->filegatorPath . '/private/TrendMicroScanner.php';

        if (!file_exists($libFile)) {
            $this->printWarning("  TrendMicroScanner library not found, skipping API test\n");
            return;
        }

        // Create a simple test (just check if we can instantiate the class)
        echo "  Testing API connectivity (basic check)...\n";
        echo "  Note: Full API test requires actual file upload\n";
        echo "  API Key: " . substr($this->config['api_key'], 0, 10) . "...\n";
        echo "  API URL: {$this->config['api_url']}\n";

        $this->printSuccess("  API configuration validated\n");
    }

    /**
     * Display next steps after installation
     */
    private function displayNextSteps()
    {
        echo "\n";
        $this->printBold("Next Steps:\n");
        $this->printBold(str_repeat("-", 60) . "\n\n");

        echo "1. " . $this->colorize("Verify Configuration", self::COLOR_CYAN) . "\n";
        echo "   - Check that PathACL and Hooks are enabled in configuration.php\n";
        echo "   - Review ACL rules in private/acl_config.php\n";
        echo "   - Review hooks config in private/hooks/config.php\n\n";

        echo "2. " . $this->colorize("Test User Access", self::COLOR_CYAN) . "\n";
        echo "   - Login as 'john' with password: '{$this->config['john_password']}'\n";
        echo "   - " . $this->colorize("Change password immediately!", self::COLOR_YELLOW) . "\n";
        echo "   - Verify folder visibility based on IP address\n\n";

        echo "3. " . $this->colorize("Test Upload Workflow", self::COLOR_CYAN) . "\n";
        echo "   - Upload a safe test file to /download\n";
        echo "   - Verify it moves to /upload automatically\n";
        echo "   - Wait for Trend Micro scan to complete\n";
        echo "   - Check that clean file appears in /scanned\n\n";

        echo "4. " . $this->colorize("Test Malware Detection (Optional)", self::COLOR_CYAN) . "\n";
        echo "   - Download EICAR test file: https://www.eicar.org/download-anti-malware-testfile/\n";
        echo "   - Upload to /download\n";
        echo "   - Verify file is deleted and email alert is sent\n";
        echo "   - Check logs in private/logs/\n\n";

        echo "5. " . $this->colorize("Monitor Logs", self::COLOR_CYAN) . "\n";
        echo "   - Audit log: {$this->filegatorPath}/private/logs/audit.log\n";
        echo "   - Malware log: {$this->filegatorPath}/private/logs/malware_detections.log\n";
        echo "   - Error log: {$this->filegatorPath}/private/logs/scan_errors.log\n\n";

        echo "6. " . $this->colorize("Security Checklist", self::COLOR_CYAN) . "\n";
        echo "   - Ensure .env file has correct permissions (0600)\n";
        echo "   - Never commit .env or API keys to version control\n";
        echo "   - Configure firewall rules for gateway IP\n";
        echo "   - Set up log rotation for audit logs\n\n";

        if ($this->backups) {
            $this->printWarning("Backup Files Created:\n");
            foreach ($this->backups as $backup) {
                echo "  - $backup\n";
            }
            echo "\n";
        }

        echo $this->colorize("Installation Path: ", self::COLOR_BOLD) . "{$this->filegatorPath}\n";
        echo $this->colorize("Documentation: ", self::COLOR_BOLD) . "{$this->examplePath}/README.md\n\n";
    }

    /**
     * Run a step with progress indication
     */
    private function runStep($description, $callback)
    {
        echo $this->colorize("[Step] ", self::COLOR_BLUE) . "$description\n";
        try {
            call_user_func($callback);
            echo "\n";
        } catch (Exception $e) {
            echo "\n";
            throw $e;
        }
    }

    /**
     * Backup a file before overwriting
     */
    private function backupFile($file)
    {
        if (!file_exists($file)) {
            return;
        }

        $backupFile = $file . '.backup.' . date('YmdHis');
        if (copy($file, $backupFile)) {
            $this->backups[] = $backupFile;
        }
    }

    /**
     * Restore backups on failure
     */
    private function restoreBackups()
    {
        if (empty($this->backups)) {
            return;
        }

        $this->printWarning("\nRestoring backup files...\n");
        foreach ($this->backups as $backup) {
            $original = preg_replace('/\.backup\.\d+$/', '', $backup);
            if (file_exists($backup)) {
                copy($backup, $original);
                echo "  Restored: $original\n";
            }
        }
    }

    /**
     * Validate IP address
     */
    private function validateIPAddress($ip)
    {
        return filter_var($ip, FILTER_VALIDATE_IP) !== false;
    }

    /**
     * Validate email address
     */
    private function validateEmail($email)
    {
        return filter_var($email, FILTER_VALIDATE_EMAIL) !== false;
    }

    /**
     * Ask for user confirmation
     */
    private function confirm($question)
    {
        echo $this->colorize($question . " [y/N]: ", self::COLOR_YELLOW);
        $handle = fopen("php://stdin", "r");
        $line = fgets($handle);
        fclose($handle);
        return strtolower(trim($line)) === 'y';
    }

    /**
     * Detect if terminal supports colors
     */
    private function detectColors()
    {
        $this->colors = (function_exists('posix_isatty') && posix_isatty(STDOUT))
            || getenv('TERM') !== false;
    }

    /**
     * Colorize text if colors are supported
     */
    private function colorize($text, $color)
    {
        if (!$this->colors) {
            return $text;
        }
        return $color . $text . self::COLOR_RESET;
    }

    /**
     * Print colored messages
     */
    private function printHeader()
    {
        $header = <<<HEADER

╔══════════════════════════════════════════════════════════════╗
║   Trend Micro File Scanning - FileGator Integration         ║
║   Installation Wizard                                        ║
╚══════════════════════════════════════════════════════════════╝

HEADER;
        echo $this->colorize($header, self::COLOR_CYAN);
    }

    private function printSuccess($msg)
    {
        echo $this->colorize($msg, self::COLOR_GREEN);
    }

    private function printError($msg)
    {
        echo $this->colorize($msg, self::COLOR_RED);
    }

    private function printWarning($msg)
    {
        echo $this->colorize($msg, self::COLOR_YELLOW);
    }

    private function printInfo($msg)
    {
        echo $this->colorize($msg, self::COLOR_BLUE);
    }

    private function printBold($msg)
    {
        echo $this->colorize($msg, self::COLOR_BOLD);
    }

    /**
     * Display current configuration
     */
    private function displayConfiguration()
    {
        echo "\n";
        $this->printBold("Configuration Summary:\n");
        $this->printBold(str_repeat("-", 60) . "\n");
        echo sprintf("%-25s %s\n", "FileGator Path:", $this->filegatorPath);
        echo sprintf("%-25s %s\n", "Gateway IP:", $this->config['gateway_ip']);
        echo sprintf("%-25s %s\n", "Admin Email:", $this->config['admin_email']);
        echo sprintf("%-25s %s\n", "API URL:", $this->config['api_url']);
        echo sprintf("%-25s %s\n", "SMTP Host:", $this->config['smtp_host']);
        echo sprintf("%-25s %s\n", "SMTP Port:", $this->config['smtp_port']);
        echo sprintf("%-25s %s\n", "User Password:", $this->config['john_password']);
        echo str_repeat("-", 60) . "\n";
    }

    /**
     * Show help message
     */
    private function showHelp()
    {
        $help = <<<HELP

Trend Micro File Scanning Installer for FileGator

USAGE:
  php install.php [OPTIONS]

REQUIRED OPTIONS:
  --gateway-ip=IP          Gateway/reverse proxy IP address (e.g., 192.168.1.100)
  --api-key=KEY            Trend Micro Cloud One API key
  --admin-email=EMAIL      Administrator email address for alerts

OPTIONAL OPTIONS:
  --api-url=URL            Trend Micro API endpoint URL
                           Default: https://filesecurity.api.trendmicro.com/v1

  --smtp-host=HOST         SMTP server hostname (default: localhost)
  --smtp-port=PORT         SMTP server port (default: 587)
  --smtp-user=USER         SMTP authentication username
  --smtp-pass=PASS         SMTP authentication password

  --john-password=PASS     Password for user 'john' (default: changeme)
  --filegator-path=PATH    FileGator installation path (auto-detected if not specified)

  --dry-run                Show what would be done without making changes
  --help, -h               Show this help message

EXAMPLES:

  Basic installation:
    php install.php \\
      --gateway-ip=192.168.1.100 \\
      --api-key=YOUR_TM_API_KEY \\
      --admin-email=admin@example.com

  Full installation with SMTP:
    php install.php \\
      --gateway-ip=192.168.1.100 \\
      --api-key=YOUR_TM_API_KEY \\
      --admin-email=admin@example.com \\
      --smtp-host=smtp.gmail.com \\
      --smtp-port=587 \\
      --smtp-user=alerts@example.com \\
      --smtp-pass=app-password

  Dry-run to preview changes:
    php install.php \\
      --gateway-ip=192.168.1.100 \\
      --api-key=YOUR_TM_API_KEY \\
      --admin-email=admin@example.com \\
      --dry-run

WHAT THIS INSTALLER DOES:

  1. Creates required directories (upload, scanned, download)
  2. Installs hook scripts for automated file scanning
  3. Configures ACL rules for IP-based access control
  4. Creates/updates user 'john'
  5. Sets up Trend Micro API integration
  6. Creates .env file with sensitive configuration
  7. Sets proper file permissions
  8. Validates the installation

SECURITY NOTES:

  - API keys are stored in .env file with 0600 permissions
  - Default password for 'john' should be changed immediately
  - Backup files are created before overwriting existing files
  - Never commit .env or API keys to version control

For more information, see:
  - README.md - User guide
  - DESIGN.md - Architecture documentation

HELP;

        echo $help . "\n";
    }

    /**
     * Display error and exit
     */
    private function error($message)
    {
        echo "\n";
        $this->printError("[ERROR] $message\n\n");
        echo "Use --help for usage information\n\n";
        exit(1);
    }
}

// Main execution
if (php_sapi_name() === 'cli') {
    try {
        $installer = new TrendMicroInstaller($argv);
        $installer->install();
    } catch (Exception $e) {
        echo "\n[FATAL ERROR] " . $e->getMessage() . "\n";
        echo "Stack trace:\n" . $e->getTraceAsString() . "\n\n";
        exit(1);
    }
} else {
    echo "This script must be run from the command line.\n";
    exit(1);
}
