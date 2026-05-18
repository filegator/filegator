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
            'region::',
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

        // Validate region if provided
        // Vision One regions: https://docs.trendmicro.com/en-us/documentation/article/trend-micro-vision-one-automation-center-regional-domains
        $region = $options['region'] ?? 'us';
        $validRegions = [
            'us'  => 'api.xdr.trendmicro.com',           // United States
            'eu'  => 'api.eu.xdr.trendmicro.com',        // European Union
            'jp'  => 'api.xdr.trendmicro.co.jp',         // Japan
            'sg'  => 'api.sg.xdr.trendmicro.com',        // Singapore
            'au'  => 'api.au.xdr.trendmicro.com',        // Australia
            'in'  => 'api.in.xdr.trendmicro.com',        // India
        ];

        if (!array_key_exists($region, $validRegions)) {
            $this->error("Invalid region: $region\nValid regions: " . implode(', ', array_keys($validRegions)));
        }

        // Build API URL from region (unless explicitly overridden)
        $apiUrl = $options['api-url'] ?? null;
        if (!$apiUrl) {
            $apiUrl = "https://{$validRegions[$region]}/v3.0/sandbox/fileSecurity/file";
        }

        // Build configuration
        $this->config = [
            'gateway_ip' => trim($options['gateway-ip']),
            'api_key' => trim($options['api-key']),
            'admin_email' => trim($options['admin-email']),
            'region' => $region,
            'api_url' => $apiUrl,
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

        // Install TrendMicroScanner SDK via Composer
        $this->installTrendMicroSDK();
    }

    /**
     * Install Trend Micro File Security SDK via Composer
     */
    private function installTrendMicroSDK(): void
    {
        $privateDir = $this->filegatorPath . '/private';
        $vendorDir = $privateDir . '/vendor/trendandrew/file-security-sdk';
        $serviceDir = $vendorDir . '/service';

        echo "\n  Installing Trend Micro File Security SDK...\n";

        // Check if already installed
        if (is_dir($vendorDir) && file_exists($vendorDir . '/src/TrendMicroScanner.php')) {
            echo "  SDK already installed at: $vendorDir\n";
            // Just ensure Node.js dependencies are installed
            if (is_dir($serviceDir) && !is_dir($serviceDir . '/node_modules')) {
                $this->installNodeDependencies($serviceDir);
            }
            return;
        }

        if (!$this->dryRun) {
            // Check if composer is available
            $composerCmd = $this->findComposer();
            if (!$composerCmd) {
                $this->printSDKInstallInstructions($privateDir);
                return;
            }

            // Check if package exists on Packagist (with timeout)
            echo "  Checking Packagist for trendandrew/file-security-sdk...\n";
            $packagistUrl = 'https://packagist.org/packages/trendandrew/file-security-sdk.json';
            $context = stream_context_create([
                'http' => [
                    'timeout' => 10,
                    'header' => 'User-Agent: FileGator-Installer/1.0'
                ]
            ]);
            $packagistResponse = @file_get_contents($packagistUrl, false, $context);

            if ($packagistResponse === false) {
                $this->printWarning("  Could not verify package on Packagist (may be a network issue).\n");
                $this->printWarning("  Attempting install anyway...\n");
            }

            // Create composer.json if it doesn't exist
            $composerJson = $privateDir . '/composer.json';
            if (!file_exists($composerJson)) {
                $composerContent = json_encode([
                    'name' => 'filegator/private',
                    'description' => 'FileGator private directory dependencies',
                    'minimum-stability' => 'beta',
                    'prefer-stable' => true,
                    'require' => new \stdClass(),
                ], JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES);
                file_put_contents($composerJson, $composerContent);
                echo "  Created: composer.json (with minimum-stability: beta)\n";
            } else {
                // Check if we need to update minimum-stability
                $existingComposer = json_decode(file_get_contents($composerJson), true);
                if (!isset($existingComposer['minimum-stability']) || $existingComposer['minimum-stability'] === 'stable') {
                    $existingComposer['minimum-stability'] = 'beta';
                    $existingComposer['prefer-stable'] = true;
                    file_put_contents($composerJson, json_encode($existingComposer, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES));
                    echo "  Updated: composer.json (set minimum-stability: beta)\n";
                }
            }

            // Run composer require with timeout and non-interactive mode
            $cmd = sprintf(
                'cd %s && timeout 120 %s require --no-interaction trendandrew/file-security-sdk 2>&1',
                escapeshellarg($privateDir),
                $composerCmd
            );

            echo "  Running: composer require trendandrew/file-security-sdk (timeout: 120s)\n";
            echo "  This may take a minute...\n";

            $output = [];
            $returnCode = 0;
            exec($cmd, $output, $returnCode);

            // timeout command returns 124 on timeout
            if ($returnCode === 124) {
                $this->printWarning("  Composer timed out after 120 seconds.\n");
                $this->printSDKInstallInstructions($privateDir);
                return;
            }

            if ($returnCode !== 0) {
                $this->printWarning("  Composer install failed (exit code: $returnCode). Output:\n");
                foreach ($output as $line) {
                    $this->printWarning("    $line\n");
                }
                $this->printSDKInstallInstructions($privateDir);
                return;
            }

            echo "  Installed: trendandrew/file-security-sdk via Composer\n";

            // Install Node.js dependencies for the scanner service
            if (is_dir($serviceDir)) {
                $this->installNodeDependencies($serviceDir);
            }
        } else {
            echo "  [DRY RUN] Would run: cd $privateDir && composer require trendandrew/file-security-sdk\n";
            echo "  [DRY RUN] Would run: cd $serviceDir && npm install\n";
        }
    }

    /**
     * Print SDK manual installation instructions
     */
    private function printSDKInstallInstructions(string $privateDir): void
    {
        $this->printWarning("\n  The SDK must be installed manually. Options:\n\n");

        $this->printWarning("  Option 1: Install from Packagist (when available):\n");
        $this->printWarning("    cd $privateDir\n");
        $this->printWarning("    composer require trendandrew/file-security-sdk\n");
        $this->printWarning("    cd vendor/trendandrew/file-security-sdk/service && npm install\n\n");

        $this->printWarning("  Option 2: Install from GitHub:\n");
        $this->printWarning("    cd $privateDir\n");
        $this->printWarning("    mkdir -p vendor/trendandrew\n");
        $this->printWarning("    git clone https://github.com/trendandrew/tm-v1-fs-php-sdk.git vendor/trendandrew/file-security-sdk\n");
        $this->printWarning("    cd vendor/trendandrew/file-security-sdk/service && npm install\n\n");

        $this->printWarning("  Option 3: Add GitHub repo to composer.json:\n");
        $this->printWarning("    Add to $privateDir/composer.json:\n");
        $this->printWarning("    {\n");
        $this->printWarning("      \"repositories\": [\n");
        $this->printWarning("        {\"type\": \"vcs\", \"url\": \"https://github.com/trendandrew/tm-v1-fs-php-sdk\"}\n");
        $this->printWarning("      ],\n");
        $this->printWarning("      \"require\": {\n");
        $this->printWarning("        \"trendandrew/file-security-sdk\": \"dev-main\"\n");
        $this->printWarning("      }\n");
        $this->printWarning("    }\n");
        $this->printWarning("    Then run: composer install && cd vendor/trendandrew/file-security-sdk/service && npm install\n");
    }

    /**
     * Find composer executable
     */
    private function findComposer(): ?string
    {
        // Check for composer in common locations
        $composerPaths = [
            'composer',
            'composer.phar',
            '/usr/local/bin/composer',
            '/usr/bin/composer',
        ];

        foreach ($composerPaths as $path) {
            $output = [];
            $returnCode = 0;
            exec("which $path 2>/dev/null", $output, $returnCode);
            if ($returnCode === 0) {
                return $path;
            }
        }

        // Check if composer.phar exists in current directory
        if (file_exists('composer.phar')) {
            return 'php composer.phar';
        }

        return null;
    }

    /**
     * Recursively copy a directory
     */
    private function copyDirectory(string $source, string $dest): void
    {
        if (!is_dir($dest)) {
            mkdir($dest, 0755, true);
        }

        $dir = opendir($source);
        while (($file = readdir($dir)) !== false) {
            if ($file === '.' || $file === '..') {
                continue;
            }

            $srcPath = $source . '/' . $file;
            $destPath = $dest . '/' . $file;

            if (is_dir($srcPath)) {
                $this->copyDirectory($srcPath, $destPath);
            } else {
                copy($srcPath, $destPath);
                chmod($destPath, 0644);
            }
        }
        closedir($dir);
    }

    /**
     * Install Node.js dependencies for the scanner service
     */
    private function installNodeDependencies(string $servicePath): void
    {
        if (!is_dir($servicePath) || !file_exists($servicePath . '/package.json')) {
            $this->printWarning("  Warning: Node.js service not found at $servicePath\n");
            return;
        }

        // Check if node and npm are available
        $nodeVersion = @shell_exec('node --version 2>/dev/null');
        $npmVersion = @shell_exec('npm --version 2>/dev/null');

        if (empty($nodeVersion)) {
            $this->printWarning("  Warning: Node.js not found. Please install Node.js >= 16.0.0\n");
            $this->printWarning("  Then run: cd $servicePath && npm install\n");
            return;
        }

        // Check Node.js version (need >= 16.0.0)
        $version = trim(str_replace('v', '', $nodeVersion));
        if (version_compare($version, '16.0.0', '<')) {
            $this->printWarning("  Warning: Node.js $version is too old. Please upgrade to >= 16.0.0\n");
            return;
        }

        echo "  Node.js version: " . trim($nodeVersion) . "\n";
        echo "  NPM version: " . trim($npmVersion) . "\n";
        echo "  Installing Node.js dependencies...\n";

        // Run npm install
        $originalDir = getcwd();
        chdir($servicePath);

        $output = [];
        $returnCode = 0;
        exec('npm install 2>&1', $output, $returnCode);

        chdir($originalDir);

        if ($returnCode !== 0) {
            $this->printWarning("  Warning: npm install failed with exit code $returnCode\n");
            $this->printWarning("  Output: " . implode("\n", array_slice($output, -5)) . "\n");
            $this->printWarning("  Please run manually: cd $servicePath && npm install\n");
        } else {
            $this->printSuccess("  Node.js dependencies installed successfully\n");
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
            $this->setWebServerOwnership($dest);
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
            $this->setWebServerOwnership($dest);
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
        $modified = false;

        // Check if PathACL is configured
        $hasPathACL = strpos($content, 'PathACLInterface') !== false;
        $hasHooks = strpos($content, 'HooksInterface') !== false;

        // Check if services are in correct order (before Router)
        $routerPos = strpos($content, "'Filegator\\Services\\Router\\Router'");
        $pathACLPos = $hasPathACL ? strpos($content, "'Filegator\\Services\\PathACL\\PathACLInterface'") : false;
        $hooksPos = $hasHooks ? strpos($content, "'Filegator\\Services\\Hooks\\HooksInterface'") : false;

        // Detect misconfigured order
        $pathACLAfterRouter = $hasPathACL && $routerPos !== false && $pathACLPos > $routerPos;
        $hooksAfterRouter = $hasHooks && $routerPos !== false && $hooksPos > $routerPos;

        if (!$this->dryRun) {
            $this->backupFile($configFile);

            // Fix misconfigured order: remove PathACL from wrong position and re-inject before Router
            if ($pathACLAfterRouter) {
                $this->printWarning("  Fixing: PathACL is configured AFTER Router, moving it before Router...\n");
                $content = $this->removeServiceConfig($content, 'PathACL');
                $content = $this->injectServiceConfig($content, $this->getPathACLConfig());
                $modified = true;
                echo "  Fixed: PathACL moved to before Router\n";
                $hasPathACL = true; // Mark as handled
            }

            // Fix misconfigured order: remove Hooks from wrong position and re-inject before Router
            if ($hooksAfterRouter) {
                $this->printWarning("  Fixing: Hooks is configured AFTER Router, moving it before Router...\n");
                $content = $this->removeServiceConfig($content, 'Hooks');
                $content = $this->injectServiceConfig($content, $this->getHooksConfig());
                $modified = true;
                echo "  Fixed: Hooks moved to before Router\n";
                $hasHooks = true; // Mark as handled
            }

            // Add PathACL configuration if missing
            if (!$hasPathACL) {
                $content = $this->injectServiceConfig($content, $this->getPathACLConfig());
                $modified = true;
                echo "  Added: PathACL service configuration (before Router)\n";
            }

            // Add Hooks configuration if missing
            if (!$hasHooks) {
                $content = $this->injectServiceConfig($content, $this->getHooksConfig());
                $modified = true;
                echo "  Added: Hooks service configuration (before Router)\n";
            }

            // Try to enable PathACL and Hooks by replacing 'enabled' => false with true
            $patterns = [
                // Enable PathACL
                "/('Filegator\\\\Services\\\\PathACL\\\\PathACLInterface'[^}]+?'enabled'\s*=>\s*)false/s" => "$1true",
                // Enable Hooks
                "/('Filegator\\\\Services\\\\Hooks\\\\HooksInterface'[^}]+?'enabled'\s*=>\s*)false/s" => "$1true",
            ];

            foreach ($patterns as $pattern => $replacement) {
                $newContent = preg_replace($pattern, $replacement, $content);
                if ($newContent !== $content) {
                    $content = $newContent;
                    $modified = true;
                }
            }

            if ($modified) {
                file_put_contents($configFile, $content);
                echo "  Updated: configuration.php\n";
            } else {
                echo "  PathACL and Hooks are already configured and enabled\n";
            }
        } else {
            echo "  [DRY RUN] Would update: configuration.php\n";
            if ($pathACLAfterRouter) {
                echo "    - Would move PathACL before Router (currently after)\n";
            } elseif (!$hasPathACL) {
                echo "    - Would add PathACL service configuration\n";
            }
            if ($hooksAfterRouter) {
                echo "    - Would move Hooks before Router (currently after)\n";
            } elseif (!$hasHooks) {
                echo "    - Would add Hooks service configuration\n";
            }
        }
    }

    /**
     * Get PathACL service configuration string
     */
    private function getPathACLConfig()
    {
        return <<<'CONFIG'
        'Filegator\Services\PathACL\PathACLInterface' => [
            'handler' => '\Filegator\Services\PathACL\PathACL',
            'config' => [
                'enabled' => true,
                'acl_config_file' => __DIR__.'/private/acl_config.php',
            ],
        ],
CONFIG;
    }

    /**
     * Get Hooks service configuration string
     */
    private function getHooksConfig()
    {
        return <<<'CONFIG'
        'Filegator\Services\Hooks\HooksInterface' => [
            'handler' => '\Filegator\Services\Hooks\Hooks',
            'config' => [
                'enabled' => true,
                'hooks_path' => __DIR__.'/private/hooks',
                'timeout' => 30,
            ],
        ],
CONFIG;
    }

    /**
     * Inject a service configuration into the services array
     *
     * IMPORTANT: Services are injected BEFORE Router because Router's init()
     * method dispatches the request immediately. Any service needed by controllers
     * must be registered before Router runs.
     */
    private function injectServiceConfig($content, $serviceConfig)
    {
        // Strategy: Find the Router key and insert the new service BEFORE it.
        // Use simple strpos() for reliability across PHP versions.

        // Pattern 1: Look for the comment block that precedes Router
        $commentSearch = "        // IMPORTANT: Router MUST be the last service";
        $pos = strpos($content, $commentSearch);
        if ($pos !== false) {
            return substr($content, 0, $pos) . $serviceConfig . "\n" . substr($content, $pos);
        }

        // Pattern 2: Look for the Router service key directly
        $literalSearch = "        'Filegator\\Services\\Router\\Router' =>";
        $pos = strpos($content, $literalSearch);
        if ($pos !== false) {
            return substr($content, 0, $pos) . $serviceConfig . "\n" . substr($content, $pos);
        }

        // Pattern 3: Try with double quotes
        $doubleQuoteSearch = '        "Filegator\\Services\\Router\\Router" =>';
        $pos = strpos($content, $doubleQuoteSearch);
        if ($pos !== false) {
            return substr($content, 0, $pos) . $serviceConfig . "\n" . substr($content, $pos);
        }

        // Fallback: warn user if we can't find injection point
        $this->printWarning("  Warning: Could not auto-inject service config. Manual edit required.\n");
        $this->printWarning("  PathACL and Hooks must be configured BEFORE Router in configuration.php\n");
        return $content;
    }

    /**
     * Remove a service configuration block from content
     *
     * @param string $content The configuration file content
     * @param string $serviceName Service name (PathACL or Hooks)
     * @return string Modified content with service removed
     */
    private function removeServiceConfig($content, $serviceName)
    {
        // Build the service key pattern based on service name
        if ($serviceName === 'PathACL') {
            $serviceKey = 'Filegator\\\\Services\\\\PathACL\\\\PathACLInterface';
        } elseif ($serviceName === 'Hooks') {
            $serviceKey = 'Filegator\\\\Services\\\\Hooks\\\\HooksInterface';
        } else {
            return $content;
        }

        // Pattern to match the entire service block including trailing comma and whitespace
        // Matches: 'ServiceKey' => [ ... ], (with proper nesting)
        $pattern = "/\s*'" . $serviceKey . "'\s*=>\s*\[[^\]]*\[[^\]]*\][^\]]*\],?\s*/s";

        $newContent = preg_replace($pattern, "\n", $content, 1);

        if ($newContent === null || $newContent === $content) {
            // Try simpler pattern for configs without nested arrays
            $simplePattern = "/\s*'" . $serviceKey . "'\s*=>\s*\[[^\]]+\],?\s*/s";
            $newContent = preg_replace($simplePattern, "\n", $content, 1);
        }

        return $newContent !== null ? $newContent : $content;
    }

    /**
     * Set proper file permissions and ownership
     *
     * Files are owned by www-data:www-data with 0600 permissions for security.
     * This ensures only the web server can read sensitive config files.
     */
    private function setFilePermissions()
    {
        // Files that need www-data ownership with restrictive permissions
        $secureFiles = [
            '/private/acl_config.php' => 0600,
            '/private/hooks/config.php' => 0600,
            '/private/users.json' => 0600,
            '/.env' => 0600,
        ];

        // Executable files (hooks)
        $executableFiles = [
            '/private/hooks/onUpload/01_move_from_download.php' => 0755,
            '/private/hooks/onUpload/02_scan_upload.php' => 0755,
        ];

        // Directories that need www-data write access
        $directories = [
            '/private/quarantine' => 0700,
            '/private/logs' => 0700,
            '/repository/upload' => 0755,
            '/repository/scanned' => 0755,
            '/repository/download' => 0755,
        ];

        // Process secure config files
        foreach ($secureFiles as $file => $perm) {
            $fullPath = $this->filegatorPath . $file;
            if (file_exists($fullPath)) {
                if (!$this->dryRun) {
                    chmod($fullPath, $perm);
                    $this->setWebServerOwnership($fullPath);
                    echo "  Set permissions " . decoct($perm) . " (www-data) on: $file\n";
                } else {
                    echo "  [DRY RUN] Would set permissions " . decoct($perm) . " (www-data) on: $file\n";
                }
            }
        }

        // Process executable files
        foreach ($executableFiles as $file => $perm) {
            $fullPath = $this->filegatorPath . $file;
            if (file_exists($fullPath)) {
                if (!$this->dryRun) {
                    chmod($fullPath, $perm);
                    $this->setWebServerOwnership($fullPath);
                    echo "  Set permissions " . decoct($perm) . " on: $file\n";
                } else {
                    echo "  [DRY RUN] Would set permissions " . decoct($perm) . " on: $file\n";
                }
            }
        }

        // Process directories
        foreach ($directories as $dir => $perm) {
            $fullPath = $this->filegatorPath . $dir;
            if (is_dir($fullPath)) {
                if (!$this->dryRun) {
                    chmod($fullPath, $perm);
                    $this->setWebServerOwnership($fullPath);
                    echo "  Set permissions " . decoct($perm) . " (www-data) on: $dir\n";
                } else {
                    echo "  [DRY RUN] Would set permissions " . decoct($perm) . " (www-data) on: $dir\n";
                }
            }
        }
    }

    /**
     * Set web server ownership on a file or directory
     *
     * Attempts to chown to www-data:www-data. Requires root privileges.
     * Falls back gracefully if chown fails (non-root installation).
     *
     * @param string $path File or directory path
     * @return bool True if successful
     */
    private function setWebServerOwnership(string $path): bool
    {
        // Detect web server user (www-data on Debian/Ubuntu, apache on RHEL/CentOS, nginx)
        $webUser = 'www-data';
        $webGroup = 'www-data';

        // Try to detect the web server user
        if (function_exists('posix_getpwnam')) {
            if (!posix_getpwnam('www-data')) {
                if (posix_getpwnam('apache')) {
                    $webUser = 'apache';
                    $webGroup = 'apache';
                } elseif (posix_getpwnam('nginx')) {
                    $webUser = 'nginx';
                    $webGroup = 'nginx';
                }
            }
        }

        // Attempt to change ownership
        $success = @chown($path, $webUser) && @chgrp($path, $webGroup);

        if (!$success && posix_getuid() !== 0) {
            // Not running as root, warn user
            static $warned = false;
            if (!$warned) {
                $this->printWarning("  Note: Run as root to set www-data ownership, or run:\n");
                $this->printWarning("        sudo chown -R {$webUser}:{$webGroup} {$this->filegatorPath}/private\n");
                $warned = true;
            }
        }

        return $success;
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

# Trend Micro Vision One File Security
# API Documentation: https://automation.trendmicro.com/xdr/api-v3#tag/File-Security
TREND_MICRO_API_KEY={api_key}
TREND_MICRO_REGION={region}
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
            ['{date}', '{api_key}', '{region}', '{api_url}', '{admin_email}', '{smtp_host}', '{smtp_port}', '{smtp_user}', '{smtp_pass}', '{gateway_ip}'],
            [
                date('Y-m-d H:i:s'),
                $this->config['api_key'],
                $this->config['region'],
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
     * Test Trend Micro scanner service
     */
    private function testTrendMicroAPI()
    {
        $vendorDir = $this->filegatorPath . '/private/vendor/trendandrew/file-security-sdk';
        $serviceDir = $vendorDir . '/service';

        if (!file_exists($vendorDir . '/src/TrendMicroScanner.php')) {
            $this->printWarning("  TrendMicroScanner library not found, skipping service test\n");
            $this->printWarning("  Install via: cd " . $this->filegatorPath . "/private && composer require trendandrew/file-security-sdk\n");
            return;
        }

        // Check if Node.js service is installed
        if (!file_exists($serviceDir . '/node_modules')) {
            $this->printWarning("  Node.js dependencies not installed, skipping service test\n");
            $this->printWarning("  Run: cd $serviceDir && npm install\n");
            return;
        }

        // Test Node.js scanner service
        echo "  Testing Node.js scanner service...\n";

        $scannerJs = $serviceDir . '/scanner.js';
        $output = [];
        $returnCode = 0;
        exec("node " . escapeshellarg($scannerJs) . " --test 2>&1", $output, $returnCode);

        if ($returnCode === 0) {
            $result = json_decode(implode('', $output), true);
            if ($result && $result['success']) {
                $this->printSuccess("  Scanner service is working\n");
                echo "  Node.js version: " . ($result['nodeVersion'] ?? 'Unknown') . "\n";
                echo "  Available regions: " . implode(', ', $result['regions'] ?? []) . "\n";
            } else {
                $this->printWarning("  Scanner service test returned unexpected result\n");
            }
        } else {
            $this->printWarning("  Scanner service test failed (exit code: $returnCode)\n");
            $this->printWarning("  Output: " . implode("\n", array_slice($output, -3)) . "\n");
        }

        echo "  API Key: " . substr($this->config['api_key'], 0, 10) . "...\n";
        echo "  Region: " . $this->config['region'] . "\n";

        $this->printSuccess("  Configuration validated\n");
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
        echo sprintf("%-25s %s\n", "Vision One Region:", $this->config['region']);
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
  --api-key=KEY            Trend Micro Vision One API key
  --admin-email=EMAIL      Administrator email address for alerts

OPTIONAL OPTIONS:
  --region=REGION          Trend Micro Vision One region (default: us)
                           Valid regions:
                             us - United States (api.xdr.trendmicro.com)
                             eu - European Union (api.eu.xdr.trendmicro.com)
                             jp - Japan (api.xdr.trendmicro.co.jp)
                             sg - Singapore (api.sg.xdr.trendmicro.com)
                             au - Australia (api.au.xdr.trendmicro.com)
                             in - India (api.in.xdr.trendmicro.com)

  --api-url=URL            Override auto-generated API URL (advanced)
                           Default: https://api.{region}.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file

  --smtp-host=HOST         SMTP server hostname (default: localhost)
  --smtp-port=PORT         SMTP server port (default: 587)
  --smtp-user=USER         SMTP authentication username
  --smtp-pass=PASS         SMTP authentication password

  --john-password=PASS     Password for user 'john' (default: changeme)
  --filegator-path=PATH    FileGator installation path (auto-detected if not specified)

  --dry-run                Show what would be done without making changes
  --help, -h               Show this help message

EXAMPLES:

  Basic installation (US region - default):
    php install.php \\
      --gateway-ip=192.168.1.100 \\
      --api-key=YOUR_TM_API_KEY \\
      --admin-email=admin@example.com

  Installation with EU region:
    php install.php \\
      --gateway-ip=192.168.1.100 \\
      --api-key=YOUR_TM_API_KEY \\
      --admin-email=admin@example.com \\
      --region=eu

  Full installation with SMTP (Singapore):
    php install.php \\
      --gateway-ip=192.168.1.100 \\
      --api-key=YOUR_TM_API_KEY \\
      --admin-email=admin@example.com \\
      --region=sg \\
      --smtp-host=smtp.gmail.com \\
      --smtp-port=587 \\
      --smtp-user=alerts@example.com \\
      --smtp-pass=app-password

  Dry-run to preview changes:
    php install.php \\
      --gateway-ip=192.168.1.100 \\
      --api-key=YOUR_TM_API_KEY \\
      --admin-email=admin@example.com \\
      --region=jp \\
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
