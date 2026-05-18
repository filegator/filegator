# Trend Micro File Scanning Integration for FileGator

A production-ready example that automatically scans uploaded files for malware using Trend Micro Cloud One File Security, with intelligent IP-based access control for secure file distribution.

## Overview

This example demonstrates a complete file security pipeline where:

1. **External partners** upload files through a gateway (reverse proxy)
2. **Files are automatically scanned** by Trend Micro Cloud One File Security
3. **Clean files** are made available to internal staff
4. **Malware is quarantined** and administrators are notified via email

The system uses FileGator's Path-based ACL and Hooks services to create a secure, automated workflow that requires zero manual intervention.

### Use Case Summary

A company receives files from external partners via a gateway server. All uploads must be scanned for malware before internal staff can access them. The solution ensures:

- External partners cannot see the company's file structure
- All uploads are automatically scanned with enterprise-grade malware detection
- Clean files are immediately available to internal users
- Security team is alerted to any threats

---

## Prerequisites

### 1. FileGator Installation

- FileGator must be installed and operational
- Minimum version: 7.8.0 (for PathACL and Hooks support)
- Web server: Apache or Nginx with PHP-FPM
- Repository directory must be writable

### 2. PHP Requirements

Ensure these PHP extensions are installed:

```bash
# Check installed extensions
php -m | grep -E "curl|json|mbstring|fileinfo"

# Ubuntu/Debian installation
sudo apt-get install php-curl php-json php-mbstring php-fileinfo

# CentOS/RHEL installation
sudo yum install php-curl php-json php-mbstring php-fileinfo
```

**Required PHP Extensions**:
- `curl` - For Trend Micro API communication
- `json` - For configuration and API responses
- `mbstring` - For string handling
- `fileinfo` - For file type detection

### 3. Trend Micro Vision One Account

> **Official Documentation**: https://automation.trendmicro.com/xdr/api-v3#tag/File-Security

1. **Create Account**: Sign up at [Trend Micro Vision One](https://portal.xdr.trendmicro.com/)
2. **Enable File Security**: Navigate to File Security service
3. **Generate API Key**:
   - Go to Administration > API Keys
   - Click "New" to create a new API key
   - Select role with "Run file scan via SDK" permission
   - Copy the API key (you won't see it again)
4. **Note Your Region**: Your API key is tied to a specific region. Check your portal URL to determine your region:

   | Region | Portal URL | API Endpoint |
   |--------|------------|--------------|
   | `us` | `portal.xdr.trendmicro.com` | `api.xdr.trendmicro.com` |
   | `eu` | `portal.eu.xdr.trendmicro.com` | `api.eu.xdr.trendmicro.com` |
   | `jp` | `portal.jp.xdr.trendmicro.com` | `api.xdr.trendmicro.co.jp` |
   | `sg` | `portal.sg.xdr.trendmicro.com` | `api.sg.xdr.trendmicro.com` |
   | `au` | `portal.au.xdr.trendmicro.com` | `api.au.xdr.trendmicro.com` |
   | `in` | `portal.in.xdr.trendmicro.com` | `api.in.xdr.trendmicro.com` |

   > **Reference**: [Regional Domains Documentation](https://docs.trendmicro.com/en-us/documentation/article/trend-micro-vision-one-automation-center-regional-domains)

### 4. Network Configuration

You need to know:

- **Gateway IP Address**: The IP address of your reverse proxy/gateway server
  - Example: `192.168.1.100`
  - This is the IP that external users connect through

- **Internal Network Range**: Your corporate network CIDR
  - Example: `192.168.1.0/24`
  - This defines which IPs are considered "internal users"

**Finding Your Gateway IP**:

```bash
# If using nginx/apache reverse proxy
grep -r X-Forwarded-For /etc/nginx/sites-enabled/

# Or check your server's IP
ip addr show
```

---

## Quick Start

### One-Command Installation

```bash
cd /path/to/filegator/docs/examples/trend-micro-file-scanning

# Basic installation (US region - default)
php install.php \
  --gateway-ip=192.168.1.100 \
  --api-key=YOUR_TREND_MICRO_API_KEY \
  --admin-email=admin@example.com

# Installation with specific region (e.g., Europe)
php install.php \
  --gateway-ip=192.168.1.100 \
  --api-key=YOUR_TREND_MICRO_API_KEY \
  --admin-email=admin@example.com \
  --region=eu

# Full installation with all options (Singapore region)
php install.php \
  --gateway-ip=192.168.1.100 \
  --api-key=YOUR_TREND_MICRO_API_KEY \
  --admin-email=admin@example.com \
  --region=sg \
  --filegator-path=/var/www/filegator \
  --smtp-host=smtp.gmail.com \
  --smtp-port=587 \
  --smtp-user=alerts@example.com \
  --smtp-pass=app-password
```

**Available Regions**: `us` (default), `eu`, `jp`, `sg`, `au`, `in`

**What This Does**:
1. Creates required directories (`/upload`, `/scanned`, `/download`)
2. Installs hook scripts for automatic file processing
3. Configures Path-based ACL rules
4. Creates user "john" for testing
5. Tests Trend Micro API connectivity
6. Displays next steps

**Installation Output Example**:

```
╔══════════════════════════════════════════════════════════════╗
║   Trend Micro File Scanning - FileGator Integration         ║
║   Installation Wizard                                        ║
╚══════════════════════════════════════════════════════════════╝

Configuration Summary:
------------------------------------------------------------
FileGator Path:           /var/www/filegator
Gateway IP:               192.168.1.100
Admin Email:              admin@example.com
Vision One Region:        eu
API URL:                  https://api.eu.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file
SMTP Host:                localhost
------------------------------------------------------------

[Step] Creating directories
  Created: /repository/upload
  Created: /repository/scanned
  Created: /repository/download

[Step] Installing hook scripts
  Installed: 01_move_from_download.php
  Installed: 02_scan_upload.php

[Step] Installing hooks configuration
  Created: hooks/config.php

[Step] Installing ACL configuration
  Created: acl_config.php
  Gateway IP: 192.168.1.100

============================================================
Installation completed successfully!
============================================================
```

---

## Manual Installation

If you prefer to install components manually or want to understand the process:

### Step 1: Create Directory Structure

```bash
cd /var/www/filegator

# Create folders in repository
mkdir -p repository/upload
mkdir -p repository/scanned
mkdir -p repository/download

# Create hooks directory
mkdir -p private/hooks/onUpload

# Create logs and quarantine directories
mkdir -p private/logs
mkdir -p private/quarantine

# Set permissions
chown -R www-data:www-data repository/
chown -R www-data:www-data private/
chmod 755 repository/{upload,scanned,download}
chmod 755 private/hooks/onUpload
chmod 755 private/quarantine
```

### Step 2: Copy Hook Scripts

```bash
cd /path/to/filegator/docs/examples/trend-micro-file-scanning

# Copy hook scripts
cp hooks/onUpload/01_move_from_download.php /var/www/filegator/private/hooks/onUpload/
cp hooks/onUpload/02_scan_upload.php /var/www/filegator/private/hooks/onUpload/

# Install the Trend Micro PHP SDK via Composer
cd /var/www/filegator/private
composer require trendandrew/file-security-sdk

# Install Node.js dependencies for the scanner service
cd vendor/trendandrew/file-security-sdk/service
npm install
```

### Step 3: Configure Hooks

```bash
# Copy hooks configuration template
cp config/hooks_config.php.template /var/www/filegator/private/hooks/config.php

# Edit configuration
nano /var/www/filegator/private/hooks/config.php
```

### Step 4: Configure ACL

```bash
# Copy ACL configuration template
cp config/acl_config.php.template /var/www/filegator/private/acl_config.php

# Replace placeholders
sed -i 's/{{GATEWAY_IP}}/192.168.1.100/g' /var/www/filegator/private/acl_config.php
sed -i 's/{{INTERNAL_NETWORK}}/192.168.1.0\/24/g' /var/www/filegator/private/acl_config.php
```

### Step 5: Enable Services

Edit `/var/www/filegator/configuration.php`:

```php
'services' => [
    // ... existing services ...

    'Filegator\Services\PathACL\PathACLInterface' => [
        'handler' => '\Filegator\Services\PathACL\PathACL',
        'config' => [
            'enabled' => true,  // REQUIRED: Must be true to enable PathACL
            'acl_config_file' => __DIR__.'/private/acl_config.php',
        ],
    ],

    'Filegator\Services\Hooks\HooksInterface' => [
        'handler' => '\Filegator\Services\Hooks\Hooks',
        'config' => [
            'enabled' => true,  // Change to true
            'hooks_path' => __DIR__.'/private/hooks',
            'timeout' => 60,
            'async' => false,
        ],
    ],
],
```

### Step 6: Create User

Add user "john" to `/var/www/filegator/private/users.json`:

```json
{
  "1": {
    "username": "admin",
    "name": "Admin",
    "role": "admin",
    "homedir": "/",
    "permissions": "read|write|upload|download|batchdownload|zip",
    "password": "..."
  },
  "2": {
    "username": "john",
    "name": "John Doe",
    "role": "user",
    "homedir": "/",
    "permissions": "read|upload|download",
    "password": "$2y$10$..."
  }
}
```

Generate password hash:

```bash
php -r "echo password_hash('changeme', PASSWORD_BCRYPT);"
```

---

## Configuration

### Getting Your Trend Micro API Key

1. **Login to Trend Micro Cloud One**: Visit [https://cloudone.trendmicro.com/](https://cloudone.trendmicro.com/)

2. **Navigate to File Security**:
   - Click on "File Security" in the left sidebar
   - If not enabled, click "Enable" and follow the setup wizard

3. **Create API Key**:
   - Go to "Administration" (top right)
   - Click "API Keys" in the dropdown
   - Click "New" button
   - Name: "FileGator Integration"
   - Role: Select "Full Access" or "Scanner"
   - Click "Next"
   - **Copy the API key immediately** (it won't be shown again)

4. **Identify Your Region**:
   - Look at your Trend Micro console URL
   - `cloudone.trendmicro.com` = us-1
   - `eu-1.cloudone.trendmicro.com` = eu-1
   - `sg-1.cloudone.trendmicro.com` = sg-1
   - `au-1.cloudone.trendmicro.com` = au-1

### Setting the Gateway IP

The gateway IP is the address from which external users access FileGator. This is typically:

- Your reverse proxy server IP (nginx, Apache)
- Your load balancer IP
- Your VPN gateway IP

**Finding Gateway IP in Nginx**:

```nginx
# Check your nginx configuration
cat /etc/nginx/sites-enabled/filegator

# Look for proxy_set_header X-Forwarded-For
# This is the IP FileGator will see
```

**Testing Gateway IP**:

```bash
# From FileGator server
tail -f /var/log/nginx/access.log

# Then access FileGator from gateway
# Note the IP address in the log
```

### Email Notification Setup

Create `.env` file in FileGator root:

```bash
# Trend Micro Vision One File Security
# API Documentation: https://automation.trendmicro.com/xdr/api-v3#tag/File-Security
# Regional Domains: https://docs.trendmicro.com/en-us/documentation/article/trend-micro-vision-one-automation-center-regional-domains
TREND_MICRO_API_KEY=your-api-key-here
TREND_MICRO_REGION=us
TREND_MICRO_API_URL=https://api.xdr.trendmicro.com/v3.0/sandbox/fileSecurity/file

# Email Configuration for Malware Alerts
ADMIN_EMAIL=security@yourcompany.com
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=alerts@yourcompany.com
SMTP_PASS=your-app-password

# Gateway Configuration
GATEWAY_IP=192.168.1.100
```

**Gmail Configuration**:

1. Enable 2-factor authentication on your Gmail account
2. Generate App Password:
   - Go to Google Account > Security
   - Under "2-Step Verification", click "App passwords"
   - Select "Mail" and "Other (Custom name)"
   - Copy the 16-character password
3. Use this as `SMTP_PASS` in `.env`

**Securing .env File**:

```bash
chmod 600 /var/www/filegator/.env
chown www-data:www-data /var/www/filegator/.env

# Add to .gitignore
echo ".env" >> /var/www/filegator/.gitignore
```

### Environment Variables Supported

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `TREND_MICRO_API_KEY` | Your Vision One API key | Yes | - |
| `TREND_MICRO_REGION` | API region: us, eu, jp, sg, au, in | No | us |
| `TREND_MICRO_API_URL` | Override API URL (advanced) | No | Auto-generated from region |
| `ADMIN_EMAIL` | Security alert recipient | Yes | - |
| `SMTP_HOST` | SMTP server address | No | localhost |
| `SMTP_PORT` | SMTP port | No | 587 |
| `SMTP_USER` | SMTP username | No | - |
| `SMTP_PASS` | SMTP password | No | - |
| `GATEWAY_IP` | Gateway server IP | Yes | - |

---

## How It Works

### Permission Matrix Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                    User: john (Standard User)                     │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  GATEWAY ACCESS (192.168.1.100)    INTERNAL ACCESS (LAN)         │
│  ────────────────────────────────  ──────────────────────────   │
│                                                                   │
│  Folder: /                         Folder: /                     │
│  Permission: Read                  Permission: Read              │
│  Visibility: Yes                   Visibility: Yes               │
│                                                                   │
│  Folder: /download                 Folder: /download             │
│  Permission: NONE                  Permission: Read, Upload      │
│  Visibility: Hidden                Visibility: Yes               │
│                                                                   │
│  Folder: /upload                   Folder: /upload               │
│  Permission: Read, Upload, Download Permission: NONE             │
│  Visibility: Yes                   Visibility: Hidden            │
│                                                                   │
│  Folder: /scanned                  Folder: /scanned              │
│  Permission: NONE                  Permission: Read, Download    │
│  Visibility: Hidden                Visibility: Yes               │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

### Workflow Explanation with Diagrams

#### Scenario 1: External Partner Uploads File

```
┌─────────────────┐
│ External Partner│
│ (via Gateway)   │
└────────┬────────┘
         │
         │ 1. Uploads file directly to /upload
         │    (Gateway has upload access to /upload)
         │
         ▼
┌────────────────────────────────────────────────┐
│         FileGator (/upload folder)             │
│  File: report.pdf                              │
└────────┬───────────────────────────────────────┘
         │
         │ 2. Hook: 02_scan_upload.php
         │    Detects file in /upload
         │    Calls Trend Micro API
         │
         ▼
┌────────────────────────────────────────────────┐
│         Trend Micro Cloud One                  │
│  Scanning: report.pdf                          │
│  Status: In Progress...                        │
└────────┬───────────────────────────────────────┘
         │
         │ 4. Scan Result: CLEAN
         │
         ▼
┌────────────────────────────────────────────────┐
│         FileGator (/scanned folder)            │
│  File: report.pdf                              │
│  Available to: Internal Users                  │
└────────────────────────────────────────────────┘
```

#### Scenario 2: Malware Detected

```
┌─────────────────┐
│  Any User       │
└────────┬────────┘
         │
         │ 1. Uploads malicious.exe
         │
         ▼
┌────────────────────────────────────────────────┐
│  Auto-moved to /upload                         │
└────────┬───────────────────────────────────────┘
         │
         │ 2. Trend Micro Scan
         │
         ▼
┌────────────────────────────────────────────────┐
│  Scan Result: MALWARE DETECTED                 │
│  Threat: Trojan.GenericKD.12345                │
└────────┬───────────────────────────────────────┘
         │
         ├─── 3a. File Deleted
         │
         ├─── 3b. Logged to /private/logs/malware_detections.log
         │
         └─── 3c. Email Sent to Admin
              ┌────────────────────────────────┐
              │ To: admin@example.com          │
              │ Subject: Malware Detected      │
              │ File: malicious.exe            │
              │ Threat: Trojan.GenericKD.12345 │
              │ Action: Deleted                │
              └────────────────────────────────┘
```

### Hook Execution Flow

```
USER UPLOADS FILE TO /download
       │
       ▼
┌──────────────────────────────────────┐
│ HOOK 1: 01_move_from_download.php   │
├──────────────────────────────────────┤
│ 1. Check: Is file in /download?      │
│    Yes → Continue                    │
│    No  → Skip (exit)                 │
│                                      │
│ 2. Calculate paths:                  │
│    Source: /download/filename.ext    │
│    Dest:   /upload/filename.ext      │
│                                      │
│ 3. Validate paths (security check)   │
│                                      │
│ 4. Move file:                        │
│    rename(source, dest)              │
│                                      │
│ 5. Log action to audit log           │
│                                      │
│ 6. Return success                    │
└──────────────────┬───────────────────┘
                   │
                   │ File is now in /upload
                   │ This triggers onUpload event again
                   ▼
┌──────────────────────────────────────┐
│ HOOK 2: 02_scan_upload.php          │
├──────────────────────────────────────┤
│ 1. Check: Is file in /upload?        │
│    Yes → Continue                    │
│    No  → Skip (exit)                 │
│                                      │
│ 2. Check file size (max 100MB)       │
│                                      │
│ 3. Initialize Trend Micro scanner:   │
│    - Load API key from env           │
│    - Set region endpoint             │
│    - Configure timeout               │
│                                      │
│ 4. Upload file to TM API:            │
│    POST /v1/scan                     │
│    Returns: scan_id                  │
│                                      │
│ 5. Poll for result (max 2 min):      │
│    GET /v1/scan/{scan_id}            │
│    Wait 2 seconds between attempts   │
│                                      │
│ 6. Process result:                   │
│    ├─ CLEAN:                         │
│    │  - Move to /scanned             │
│    │  - Log success                  │
│    │  - Return success               │
│    │                                 │
│    ├─ MALWARE:                       │
│    │  - Delete file                  │
│    │  - Log threat details           │
│    │  - Send email alert             │
│    │  - Return malware result        │
│    │                                 │
│    └─ ERROR:                         │
│       - Log error                    │
│       - Quarantine file (optional)   │
│       - Return error                 │
└──────────────────────────────────────┘
```

---

## Usage Guide

### For External Users (via Gateway)

**What You Can Do**:
- Upload files to `/download` folder
- View files in `/upload` folder (after they're moved)
- Download files from `/upload` folder

**What You Cannot Do**:
- View `/scanned` folder (hidden)
- Access other users' files
- Delete files

**Upload Process**:

1. **Login** as user "john" through gateway URL:
   ```
   https://gateway.yourcompany.com/filegator
   ```

2. **Navigate** to `/download` folder

3. **Upload** your file using the web interface

4. **Wait** a few seconds - file will disappear from `/download`

5. **Check** `/upload` folder to see your file being scanned

6. File will be removed from `/upload` once scan completes

### For Internal Users

**What You Can Do**:
- Upload files to `/download` folder (for scanning)
- Download files from `/scanned` folder (already scanned)
- View files in `/scanned` folder

**What You Cannot Do**:
- View `/upload` folder (hidden)
- Access quarantined files

**Download Process**:

1. **Login** as user "john" from internal network:
   ```
   https://filegator.internal.yourcompany.com
   ```

2. **Navigate** to `/scanned` folder

3. **Download** any clean files

4. Files are guaranteed malware-free by Trend Micro

### What Happens to Uploaded Files

```
TIMELINE OF UPLOADED FILE
═════════════════════════

T+0 seconds:  File uploaded to /download
              Status: Waiting for processing
              Visible to: Uploader

T+1 second:   Hook moves file to /upload
              Status: Moved to scanning queue
              Visible to: Gateway users

T+2 seconds:  File uploaded to Trend Micro
              Status: Scan in progress
              Visible to: Gateway users

T+5-30 sec:   Scan completes
              Status: Result received
              Visible to: Depends on result

If CLEAN:
  T+30 sec:   Moved to /scanned
              Status: Available for download
              Visible to: Internal users only

If MALWARE:
  T+30 sec:   File deleted
              Status: Threat neutralized
              Visible to: No one
              Action: Email sent to admin
```

---

## Troubleshooting

### Common Issues and Solutions

#### Issue 1: Files Not Moving from /download to /upload

**Symptoms**:
- Files stay in `/download` folder
- No files appear in `/upload`

**Diagnosis**:

```bash
# Check hook execution logs
tail -f /var/www/filegator/private/logs/hooks.log

# Check hook is present
ls -la /var/www/filegator/private/hooks/onUpload/01_move_from_download.php

# Verify hooks are enabled
grep -A5 "HooksInterface" /var/www/filegator/configuration.php
```

**Solutions**:

1. **Enable Hooks Service**:
   ```php
   // In configuration.php
   'Filegator\Services\Hooks\HooksInterface' => [
       'config' => [
           'enabled' => true,  // Must be true
       ],
   ],
   ```

2. **Check File Permissions**:
   ```bash
   chmod 755 /var/www/filegator/private/hooks/onUpload
   chmod 644 /var/www/filegator/private/hooks/onUpload/*.php
   ```

3. **Verify Directory Exists**:
   ```bash
   mkdir -p /var/www/filegator/repository/upload
   chown www-data:www-data /var/www/filegator/repository/upload
   ```

#### Issue 2: Scan Fails with API Error

**Symptoms**:
- Files stuck in `/upload` folder
- Error logs show API failures

**Diagnosis**:

```bash
# Check scan logs
tail -f /var/www/filegator/private/logs/scan_errors.log

# Test API connectivity
php /var/www/filegator/docs/examples/trend-micro-file-scanning/scripts/check_tm_api.php
```

**Solutions**:

1. **Verify API Key**:
   ```bash
   # Check .env file
   cat /var/www/filegator/.env | grep TREND_MICRO_API_KEY

   # Test API key manually
   curl -H "Authorization: Bearer YOUR_API_KEY" \
        https://filesecurity.api.trendmicro.com/v1/health
   ```

2. **Check Region Setting**:
   ```bash
   # Ensure region matches your account
   # us-1, eu-1, sg-1, or au-1
   cat /var/www/filegator/.env | grep TREND_MICRO_REGION
   ```

3. **Verify Network Access**:
   ```bash
   # Test outbound HTTPS
   curl -v https://filesecurity.api.trendmicro.com

   # Check firewall rules
   sudo iptables -L OUTPUT -n -v
   ```

#### Issue 3: Gateway IP Not Recognized

**Symptoms**:
- Gateway users see wrong folders
- Internal users from gateway IP see wrong folders

**Diagnosis**:

```bash
# Check what IP FileGator sees
# Login from gateway and check:
tail -f /var/www/filegator/private/logs/audit.log | grep "user: john"
```

**Solutions**:

1. **Configure Reverse Proxy**:

   For Nginx:
   ```nginx
   location / {
       proxy_pass http://filegator_backend;
       proxy_set_header X-Forwarded-For 192.168.1.100;  # Set gateway IP
       proxy_set_header X-Real-IP $remote_addr;
   }
   ```

   For Apache:
   ```apache
   <VirtualHost *:443>
       ProxyPass / http://filegator_backend/
       RequestHeader set X-Forwarded-For "192.168.1.100"
   </VirtualHost>
   ```

2. **Update ACL Configuration**:
   ```bash
   # Edit ACL config
   nano /var/www/filegator/private/acl_config.php

   # Update gateway IP
   'ip_allowlist' => ['192.168.1.100'],  # Your actual gateway IP
   ```

#### Issue 4: Email Notifications Not Sending

**Symptoms**:
- Malware detected but no email received
- Logs show email errors

**Diagnosis**:

```bash
# Check email logs
tail -f /var/www/filegator/private/logs/hooks.log | grep "email"

# Check SMTP settings
cat /var/www/filegator/.env | grep SMTP
```

**Solutions**:

1. **Test SMTP Connection**:
   ```bash
   # Install swaks (SMTP test tool)
   sudo apt-get install swaks

   # Test SMTP
   swaks --to admin@example.com \
         --from alerts@yourcompany.com \
         --server smtp.gmail.com:587 \
         --auth LOGIN \
         --auth-user alerts@yourcompany.com \
         --auth-password "your-app-password" \
         --tls
   ```

2. **Verify Gmail Settings** (if using Gmail):
   - Enable 2FA on account
   - Generate App Password
   - Use App Password in SMTP_PASS

3. **Check PHP Mail Configuration**:
   ```bash
   # Check if PHP can send mail
   php -r "mail('test@example.com', 'Test', 'Test message');"
   ```

### Log File Locations

```
/var/www/filegator/private/logs/
├── hooks.log                 # General hook execution
├── audit.log                 # Audit trail (uploads, scans, moves)
├── malware_detections.log    # Malware alerts
└── scan_errors.log           # Scan failures
```

**Reading Logs**:

```bash
# Watch hooks in real-time
tail -f /var/www/filegator/private/logs/hooks.log

# Search for specific file
grep "filename.pdf" /var/www/filegator/private/logs/audit.log

# View malware detections
cat /var/www/filegator/private/logs/malware_detections.log

# Check recent errors
tail -20 /var/www/filegator/private/logs/scan_errors.log
```

### Testing the Setup

#### Test 1: Upload and Scan Clean File

```bash
# Create test file
echo "This is a clean test file" > /tmp/test_clean.txt

# Upload via command line (requires credentials)
# Or upload via web interface

# Monitor processing
tail -f /var/www/filegator/private/logs/audit.log

# Expected output:
# [timestamp] FILE_MOVED: /download/test_clean.txt -> /upload/test_clean.txt
# [timestamp] SCAN_RESULT: test_clean.txt - Status: clean, Malware: NO
# [timestamp] File moved to /scanned/test_clean.txt
```

#### Test 2: Upload EICAR Test File

**WARNING**: EICAR is a safe test file recognized as malware by all antivirus software.

```bash
# Download EICAR test file
curl -o /tmp/eicar.com https://secure.eicar.org/eicar.com

# Upload to FileGator (via web interface)

# Monitor processing
tail -f /var/www/filegator/private/logs/malware_detections.log

# Expected output:
# [timestamp] MALWARE_DETECTED: eicar.com - Threat: EICAR-Test-File

# Check email inbox for alert
```

#### Test 3: Verify ACL Rules

```bash
# Test script
php /var/www/filegator/docs/examples/trend-micro-file-scanning/scripts/test_installation.php

# Expected output:
# Running: check_directories... PASS
# Running: check_hooks... PASS
# Running: check_acl_config... PASS
# Running: check_hooks_config... PASS
# Running: check_env_variables... PASS
# Running: check_permissions... PASS
```

---

## Security Considerations

### API Key Storage

**Never hardcode API keys in PHP files**. Always use environment variables:

```bash
# .env file (NEVER commit to git)
TREND_MICRO_API_KEY=your-actual-api-key-here

# Secure the .env file
chmod 600 /var/www/filegator/.env
chown www-data:www-data /var/www/filegator/.env

# Add to .gitignore
echo ".env" >> .gitignore
echo "private/hooks/config.php" >> .gitignore
```

**Rotating API Keys**:

1. Generate new key in Trend Micro console
2. Update `.env` file with new key
3. Test with a clean file upload
4. Revoke old key in Trend Micro console

### Network Security

#### Firewall Rules

```bash
# Allow outbound HTTPS to Trend Micro API
sudo iptables -A OUTPUT -p tcp --dport 443 -d filesecurity.api.trendmicro.com -j ACCEPT

# Block all other outbound connections (optional)
sudo iptables -A OUTPUT -p tcp --dport 443 -j DROP
```

#### Reverse Proxy Security

```nginx
# Nginx configuration for gateway
server {
    listen 443 ssl;
    server_name gateway.yourcompany.com;

    # SSL Configuration
    ssl_certificate /etc/ssl/certs/gateway.crt;
    ssl_certificate_key /etc/ssl/private/gateway.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=upload:10m rate=10r/s;

    location / {
        # Apply rate limiting
        limit_req zone=upload burst=20;

        # Forward to FileGator
        proxy_pass http://127.0.0.1:8080;

        # Set gateway IP
        proxy_set_header X-Forwarded-For 192.168.1.100;
        proxy_set_header X-Real-IP $remote_addr;

        # File upload size limit
        client_max_body_size 100M;
    }
}
```

### File Permissions

**Recommended Permissions**:

```bash
# Repository (file storage)
chown -R www-data:www-data /var/www/filegator/repository
chmod 755 /var/www/filegator/repository
chmod 755 /var/www/filegator/repository/{upload,scanned,download}

# Hooks
chown -R www-data:www-data /var/www/filegator/private/hooks
chmod 755 /var/www/filegator/private/hooks
chmod 644 /var/www/filegator/private/hooks/**/*.php

# Configuration
chmod 644 /var/www/filegator/private/acl_config.php
chmod 644 /var/www/filegator/private/hooks/config.php
chmod 600 /var/www/filegator/.env

# Quarantine (restricted access)
chmod 700 /var/www/filegator/private/quarantine
```

---

## Customization

### Changing Directories

To use different folder names:

1. **Update ACL Configuration** (`private/acl_config.php`):
   ```php
   'path_rules' => [
       '/my_upload_folder' => [  // Changed from /upload
           // ... rules ...
       ],
   ],
   ```

2. **Update Hook Scripts**:
   ```php
   // In 01_move_from_download.php
   $destDir = $repoPath . $homeDir . '/my_upload_folder';  // Changed

   // In 02_scan_upload.php
   if (strpos($uploadPath, '/my_upload_folder/') !== 0) {  // Changed
   ```

3. **Create New Directories**:
   ```bash
   mkdir -p /var/www/filegator/repository/my_upload_folder
   ```

### Modifying Scan Behavior

Edit `private/hooks/config.php`:

```php
'trend_micro' => [
    // Skip scanning certain file types
    'skip_extensions' => ['txt', 'jpg', 'png'],  // Add extensions to skip

    // Change file size limit
    'max_file_size' => 50 * 1024 * 1024,  // 50MB instead of 100MB

    // Quarantine instead of delete
    'on_malware' => [
        'action' => 'quarantine',  // Changed from 'delete'
        'quarantine_dir' => __DIR__ . '/../quarantine',
    ],

    // Allow files on scan error
    'on_error' => [
        'action' => 'allow',  // Changed from 'quarantine'
        // File will be moved to /scanned even on error
    ],
],
```

### Adding More Users

Edit `private/users.json`:

```json
{
  "2": {
    "username": "john",
    "name": "John Doe",
    "role": "user",
    "homedir": "/",
    "permissions": "read|upload|download",
    "password": "$2y$10$..."
  },
  "3": {
    "username": "jane",
    "name": "Jane Smith",
    "role": "user",
    "homedir": "/",
    "permissions": "read|upload|download",
    "password": "$2y$10$..."
  }
}
```

Update ACL rules to include new users:

```php
'path_rules' => [
    '/upload' => [
        'rules' => [
            [
                'users' => ['john', 'jane'],  // Add new user
                // ... rest of rule ...
            ],
        ],
    ],
],
```

---

## Additional Resources

### Trend Micro Vision One Documentation

- [File Security API Reference](https://automation.trendmicro.com/xdr/api-v3#tag/File-Security) - Complete API documentation
- [Vision One File Security Overview](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-file-security-intro-origin) - Product overview
- [API Key Setup Guide](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-api-keys) - How to create API keys
- [Node.js SDK](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk) - Official SDK for reference

### FileGator Documentation

- [FileGator Documentation](https://docs.filegator.io/)
- [FileGator Hooks Guide](https://docs.filegator.io/hooks.html)
- [FileGator ACL Guide](https://docs.filegator.io/acl.html)

### Example Files

All example files are in: `/docs/examples/trend-micro-file-scanning/`

- `DESIGN.md` - Complete architecture documentation
- `config/acl_config.php.template` - ACL configuration template
- `config/hooks_config.php.template` - Hooks configuration template
- `hooks/onUpload/01_move_from_download.php` - File movement hook
- `hooks/onUpload/02_scan_upload.php` - Scanning hook
- `scripts/test_installation.php` - Installation verification
- `scripts/check_tm_api.php` - API connectivity test

### PHP SDK

The Trend Micro File Security PHP SDK is available as a separate Composer package:

- Package: `trendandrew/file-security-sdk`
- Repository: [https://github.com/trendandrew/tm-v1-fs-php-sdk](https://github.com/trendandrew/tm-v1-fs-php-sdk)
- Install: `composer require trendandrew/file-security-sdk`

### Support

- FileGator Issues: [https://github.com/filegator/filegator/issues](https://github.com/filegator/filegator/issues)
- Trend Micro Support: [https://cloudone.trendmicro.com/support](https://cloudone.trendmicro.com/support)

---

## License

This example is provided as-is for demonstration purposes. See FileGator's main LICENSE file for licensing terms.

---

**Document Version**: 1.0
**Last Updated**: 2025-12-09
**Tested With**: FileGator 7.8.0, Trend Micro Cloud One File Security API v1
