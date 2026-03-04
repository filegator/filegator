# IP Address Matching and Specification: Research Report

**Date**: 2025-12-09
**Context**: FileGator PHP Web Application
**Current Implementation**: `/mnt/ai/filegator/backend/Services/Security/Security.php` (lines 59-91)

## Executive Summary

This research analyzes IP address matching and specification syntax across major production systems to recommend the best approach for FileGator's PHP-based configuration. The current implementation only supports exact IP matching without CIDR, ranges, or IPv6 support.

**Key Findings**:
- **Most Familiar**: Apache-style CIDR notation (`192.168.1.0/24`)
- **Most Expressive**: Nginx's geo module with variable-based logic
- **Most Robust**: First-match evaluation with explicit precedence
- **Best Practice**: Combine allowlist-first evaluation with CIDR support using Symfony's IpUtils

---

## 1. Apache mod_authz_host

### Syntax Overview

**Modern Apache 2.4+ (Recommended)**:
```apache
<RequireAll>
    Require all granted
    Require not ip 1.2.3.4
    Require not ip 1.2.3.0/24
    Require not ip 2001:db8::/32
</RequireAll>
```

**Legacy Apache 2.2 (Deprecated)**:
```apache
Order Deny,Allow
Deny from all
Allow from 192.168.1.0/24
Allow from 2001:db8::/32
```

### Supported Formats

| Format | Example | Description |
|--------|---------|-------------|
| Single IP | `192.168.1.1` | Exact IP match |
| Partial IP | `192.168.1` | Matches 192.168.1.0-255 |
| CIDR Block | `192.168.1.0/24` | Network mask notation |
| Network/Netmask | `10.0.0.0/255.255.0.0` | Traditional subnet mask |
| IPv6 | `2001:db8::/32` | IPv6 CIDR notation |

### Evaluation Order

**Apache 2.4**:
- Uses `RequireAll`, `RequireAny`, `RequireNone` containers
- Logical AND/OR/NOT operations
- More explicit and predictable than 2.2

**Apache 2.2** (Legacy):
- `Order Deny,Allow`: Evaluates Deny rules first, then Allow (default Allow)
- `Order Allow,Deny`: Evaluates Allow rules first, then Deny (default Deny)
- Last match wins within the same directive type

### Negation/Exceptions

Apache 2.4 uses `Require not ip` for exclusions:
```apache
<RequireAll>
    Require ip 192.168.1.0/24
    Require not ip 192.168.1.50
</RequireAll>
```

### IPv6 Support

Full IPv6 support in both 2.2 and 2.4 versions with standard CIDR notation.

### Sources
- [Access Control - Apache HTTP Server Version 2.4](https://httpd.apache.org/docs/2.4/howto/access.html)
- [mod_authz_host - Apache HTTP Server Version 2.4](https://httpd.apache.org/docs/2.4/mod/mod_authz_host.html)
- [Apache: How to accept requests from only a range of IPs - Sling Academy](https://www.slingacademy.com/article/apache-how-to-accept-requests-from-only-a-range-of-ips/)

---

## 2. Nginx

### Syntax Overview

**Basic Allow/Deny (ngx_http_access_module)**:
```nginx
location / {
    deny  192.168.1.1;
    allow 192.168.1.0/24;
    allow 10.1.1.0/16;
    allow 2001:0db8::/32;
    deny  all;
}
```

**Geo Module (ngx_http_geo_module)**:
```nginx
geo $limited_access {
    default 0;
    192.168.1.0/24 1;
    10.0.0.0/16 1;
}

server {
    location /restricted {
        if ($limited_access = 0) {
            return 403;
        }
    }
}
```

### Supported Formats

| Format | Example | Description |
|--------|---------|-------------|
| Single IP | `192.168.1.1` | Exact IP match |
| CIDR Block | `192.168.1.0/24` | Network mask notation |
| IP Range | `192.168.1.10-192.168.1.50` | Range notation (geo module) |
| IPv6 CIDR | `2001:0db8::/32` | IPv6 support (v1.3.10+) |

### Evaluation Order

**First-match wins**: Rules are checked sequentially until the first match is found.

Example:
```nginx
deny  192.168.1.1;        # Rule 1: Specific deny
allow 192.168.1.0/24;     # Rule 2: Broader allow
deny  all;                # Rule 3: Default deny
```

If IP is `192.168.1.1`: Matches Rule 1 → DENIED
If IP is `192.168.1.50`: Matches Rule 2 → ALLOWED
If IP is `10.0.0.1`: Matches Rule 3 → DENIED

### Negation/Exceptions

Nginx uses explicit rule ordering rather than negation operators. Place specific exceptions before broader rules:
```nginx
allow 192.168.1.100;      # Exception: allow this specific IP
deny  192.168.1.0/24;     # Deny the rest of the subnet
```

### IPv6 Support

Native IPv6 support since version 1.3.10/1.2.7 using standard CIDR notation.

### External IP Lists

```nginx
geo $limited_access {
    include /etc/nginx/conf.d/ip_allowlist.conf;
    default 0;
}
```

### Sources
- [Module ngx_http_geo_module](https://nginx.org/en/docs/http/ngx_http_geo_module.html)
- [Module ngx_http_access_module](https://nginx.org/en/docs/http/ngx_http_access_module.html)
- [Dynamic Denylisting of IP Addresses | NGINX Documentation](https://docs.nginx.com/nginx/admin-guide/security-controls/denylisting-ip-addresses/)
- [Nginx Block And Deny IP Address OR Network Subnets - nixCraft](https://www.cyberciti.biz/faq/linux-unix-nginx-access-control-howto/)

---

## 3. iptables / nftables

### Syntax Overview

**iptables (IPv4)**:
```bash
# Single IP
iptables -A INPUT -s 192.168.1.1 -j ACCEPT

# CIDR block
iptables -A INPUT -s 192.168.1.0/24 -j ACCEPT

# IP Range (using iprange module)
iptables -A INPUT -m iprange --src-range 192.168.1.10-192.168.1.50 -j ACCEPT
```

**nftables**:
```bash
# Single IP
nft add rule filter output ip daddr 8.8.8.8 counter

# Set-based matching (efficient for large lists)
nft add rule filter input ip saddr { 192.168.1.0/24, 10.0.0.0/16 } accept
```

### Supported Formats

| Format | Example | Description |
|--------|---------|-------------|
| Single IP | `192.168.1.1` | Exact IP match |
| CIDR Block | `192.168.1.0/24` | Standard CIDR notation |
| IP Range | `192.168.1.10-192.168.1.50` | Range with iprange module |
| IPv6 | `2001:db8::/32` | IPv6 CIDR |
| IP Sets | `{ 10.0.0.0/8, 192.168.0.0/16 }` | Multiple values (nftables) |

### Evaluation Order

**Chain Priority System**:
- Rules are evaluated sequentially within a chain
- First matching rule with a verdict (accept/drop) terminates evaluation
- Chains can have different priorities at the same hook point
- Lower priority numbers execute first

**nftables Verdict Statements**:
- `accept`: Accept and stop evaluation
- `drop`: Drop and stop evaluation
- `continue`: Continue to next rule
- `return`: Return to calling chain
- `jump <chain>`: Jump to another chain

### Negation/Exceptions

**iptables**:
```bash
# NOT operator
iptables -A INPUT ! -s 192.168.1.50 -j ACCEPT
```

**nftables**:
```bash
# NOT with sets
nft add rule filter input ip saddr != { 192.168.1.50, 192.168.1.51 } accept
```

### IPv6 Support

**ip6tables** (separate tool for IPv6):
```bash
ip6tables -A INPUT -s 2001:db8::/32 -j ACCEPT
```

**nftables** (unified IPv4/IPv6):
```bash
# inet family handles both IPv4 and IPv6
nft add rule inet filter input ip6 saddr 2001:db8::/32 accept
```

### Sources
- [Rules - nftables wiki](https://wiki.nftables.org/wiki-nftables/index.php/Simple_rule_management)
- [nftables - ArchWiki](https://wiki.archlinux.org/title/Nftables)
- [Quick reference-nftables in 10 minutes](https://wiki.nftables.org/wiki-nftables/index.php/Quick_reference-nftables_in_10_minutes)
- [Linux Iptables: How to specify a range of IP addresses or ports - nixCraft](https://www.cyberciti.biz/tips/linux-iptables-how-to-specify-a-range-of-ip-addresses-or-ports.html)

---

## 4. AWS Security Groups / S3 Bucket Policies

### Syntax Overview

**S3 Bucket Policy with IP Conditions**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowIPmix",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::my-bucket/*",
      "Condition": {
        "IpAddress": {
          "aws:SourceIp": [
            "192.0.2.0/24",
            "2001:DB8:1234:5678::/64"
          ]
        },
        "NotIpAddress": {
          "aws:SourceIp": [
            "203.0.113.0/24"
          ]
        }
      }
    }
  ]
}
```

### Supported Formats

| Format | Example | Description |
|--------|---------|-------------|
| CIDR IPv4 | `"192.0.2.0/24"` | Standard CIDR notation |
| CIDR IPv6 | `"2001:DB8::/64"` | IPv6 CIDR with :: notation |
| Array | `["10.0.0.0/16", "192.168.0.0/24"]` | Multiple ranges |

### Evaluation Order

**Policy Logic**:
- Multiple condition operators within a statement are ANDed together
- Multiple values within a condition are ORed together
- `IpAddress` operator: Matches if source IP is in ANY of the listed ranges
- `NotIpAddress` operator: Matches if source IP is NOT in ANY of the listed ranges

**Example**:
```json
"Condition": {
  "IpAddress": {
    "aws:SourceIp": ["10.0.0.0/8", "192.168.0.0/16"]
  },
  "NotIpAddress": {
    "aws:SourceIp": ["192.168.1.0/24"]
  }
}
```
This allows access from `10.0.0.0/8` OR `192.168.0.0/16` EXCEPT `192.168.1.0/24`.

### Negation/Exceptions

AWS uses explicit condition operators:
- `IpAddress`: Positive matching
- `NotIpAddress`: Negative matching (exclusion)

Both can be combined in the same statement for allow-with-exceptions logic.

### IPv6 Support

Full IPv6 support with standard CIDR notation including `::` compression:
```json
"aws:SourceIp": "2001:DB8:1234:5678::/64"
```

**Important**: `aws:SourceIp` only works for public IP addresses, not internal VPC addresses.

### Sources
- [Examples of Amazon S3 bucket policies - Amazon Simple Storage Service](https://docs.aws.amazon.com/AmazonS3/latest/userguide/example-bucket-policies.html)
- [IAM JSON policy elements: Condition operators - AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_elements_condition_operators.html)
- [S3 Bucket Policies: Restrict Access to Specific IP Addresses](https://asecure.cloud/a/s3_restrict_ip/)
- [Bucket Policy | Products - IONOS](https://docs.ionos.com/cloud/storage-and-backup/ionos-object-storage/settings/bucket-policy)

---

## 5. Cloudflare WAF / IP Rules

### Syntax Overview

**IP Access Rules (UI-based)**:
- Allowlist, block, or challenge traffic by IP, ASN, or country
- CIDR notation supported for IP ranges
- Limitations: Only /16 and /24 CIDR blocks allowed

**Firewall Rules / WAF Custom Rules**:
```javascript
// Expression syntax
(ip.src in {127.0.0.1 0.0.0.0})

// With hostname and URI
(http.host eq "example.com" and ip.src in {192.168.1.0/24})

// Using IP lists
(ip.src in $ip_list_name)
```

### Supported Formats

| Format | Example | Description |
|--------|---------|-------------|
| Single IP | `192.168.1.1` | Exact IP match |
| CIDR /16 | `192.168.0.0/16` | Class B network |
| CIDR /24 | `192.168.1.0/24` | Class C network |
| IP Lists | `$trusted_ips` | Named IP list reference |
| ASN | `AS64496` | Autonomous System Number |

### Evaluation Order

**Rule Evaluation**:
- Rules are evaluated in priority order (lower number = higher priority)
- First matching rule determines the action
- Actions: Allow, Block, Challenge (CAPTCHA), JS Challenge, Log

**Best Practice from Cloudflare**:
- Allow known good bots first (`cf.client.bot` field)
- Then apply security rules
- Default deny/challenge at the end

### Negation/Exceptions

Use boolean operators in expressions:
```javascript
// Allow all except specific IPs
(not ip.src in {192.168.1.50 192.168.1.51})

// Complex logic
(ip.src in {10.0.0.0/8}) and (not ip.src in {10.0.1.0/24})
```

### IPv6 Support

IPv6 is fully supported in Cloudflare's IP matching:
```javascript
(ip.src in {2001:db8::/32})
```

### Origin Protection Best Practice

Cloudflare strongly recommends:
- Configure origin server firewall to only accept traffic from Cloudflare IP ranges
- This ensures all traffic is filtered through WAF
- Prevents direct-to-origin attacks

### Sources
- [About Cloudflare Firewall Rules · Cloudflare Firewall Rules (deprecated) docs](https://developers.cloudflare.com/firewall/cf-firewall-rules/)
- [IP Access rules · Cloudflare Web Application Firewall (WAF) docs](https://developers.cloudflare.com/waf/tools/ip-access-rules/)
- [Guide to Using Cloudflare WAF - Setup, Rules, Example and Documentation](https://www.appsecure.security/blog/guide-to-cloudfare-firewall-setup)
- [How to Use Cloudflare Firewall Rules to Protect Your Website](https://blog.runcloud.io/cloudflare-firewall-rules/)

---

## 6. PHP Libraries

### Symfony HttpFoundation IpUtils

**Primary Recommendation**: `Symfony\Component\HttpFoundation\IpUtils`

**Features**:
- Check if IP is in list of IPs or subnets
- IPv4 and IPv6 support
- CIDR notation support
- IP anonymization
- Well-tested and maintained

**API**:
```php
use Symfony\Component\HttpFoundation\IpUtils;

// Check single IP against list
$trusted = ['192.168.1.0/24', '10.0.0.0/8', '2001:db8::/32'];
$clientIp = '192.168.1.50';

if (IpUtils::checkIp($clientIp, $trusted)) {
    // IP is in trusted list
}

// Check IPv6
if (IpUtils::checkIp('2001:db8::1', ['2001:db8::/32'])) {
    // IPv6 match
}

// Anonymize IP (GDPR compliance)
$anonymized = IpUtils::anonymize('192.168.1.50'); // Returns 192.168.1.0
```

**Internal Implementation**:
- Uses CIDR range expansion for matching
- Handles IPv4-mapped IPv6 addresses
- Supports both CIDR notation and raw netmask format
- Available in Symfony 5.3, 6.1, 7.2, 7.3, 7.4

### Symfony CIDR Validator

```php
use Symfony\Component\Validator\Constraints\Cidr;

#[Cidr(
    version: 'all',  // '4', '6', or 'all'
    netmaskMin: 8,
    netmaskMax: 32,
    message: 'Invalid CIDR notation'
)]
private string $ipRange;
```

### Alternative: tholu/php-cidr-match

**GitHub**: [tholu/php-cidr-match](https://github.com/tholu/php-cidr-match)

**Note**: Author recommends using Symfony's IpUtils instead, as it provides the same functionality with better testing and maintenance.

### Comparison

| Feature | Symfony IpUtils | php-cidr-match |
|---------|----------------|----------------|
| IPv4 Support | ✅ | ✅ |
| IPv6 Support | ✅ | ✅ |
| CIDR Notation | ✅ | ✅ |
| Maintenance | Active (Symfony) | Less active |
| Testing | Extensive | Basic |
| Dependencies | Symfony ecosystem | Standalone |
| Recommendation | **Primary choice** | Legacy/standalone |

### Sources
- [Symfony HttpFoundation IpUtils Class Reference](https://pkp.sfu.ca/ojs/doxygen/master/html/classSymfony_1_1Component_1_1HttpFoundation_1_1IpUtils.html)
- [Cidr (Symfony Docs)](https://symfony.com/doc/current/reference/constraints/Cidr.html)
- [GitHub - symfony/http-foundation IpUtils.php](https://github.com/symfony/http-foundation/blob/7.3/IpUtils.php)
- [GitHub - tholu/php-cidr-match](https://github.com/tholu/php-cidr-match)

---

## Comparative Analysis

### Syntax Comparison

| System | Single IP | CIDR Range | IPv6 | Negation | Evaluation |
|--------|-----------|------------|------|----------|------------|
| **Apache 2.4** | `Require ip 1.2.3.4` | `Require ip 1.2.3.0/24` | ✅ | `Require not ip` | Logical containers |
| **Nginx** | `allow 1.2.3.4;` | `allow 1.2.3.0/24;` | ✅ | Order-based | First-match |
| **iptables** | `-s 1.2.3.4` | `-s 1.2.3.0/24` | ✅ (ip6tables) | `! -s` | Sequential |
| **nftables** | `ip saddr 1.2.3.4` | `ip saddr 1.2.3.0/24` | ✅ (inet family) | `!=` operator | Priority-based |
| **AWS S3** | `"1.2.3.4"` (JSON) | `"1.2.3.0/24"` | ✅ | `NotIpAddress` | AND/OR logic |
| **Cloudflare** | `{1.2.3.4}` | `{1.2.3.0/24}` | ✅ | `not` operator | Priority order |
| **Symfony** | `checkIp()` | `checkIp()` | ✅ | Array logic | Programmatic |

### Evaluation Order Models

1. **First-Match (Nginx, iptables)**:
   - Rules evaluated sequentially
   - First matching rule wins
   - Simple and predictable
   - Requires careful rule ordering

2. **Logical Containers (Apache 2.4)**:
   - `RequireAll`: All conditions must match (AND)
   - `RequireAny`: At least one condition matches (OR)
   - `RequireNone`: No conditions match (NOT)
   - More expressive but more complex

3. **Priority-Based (nftables)**:
   - Chains have numeric priorities
   - Lower numbers execute first
   - Multiple chains can exist at same hook
   - Most flexible but complex

4. **Boolean Logic (AWS, Cloudflare)**:
   - Condition operators combine with AND/OR
   - Explicit inclusion/exclusion operators
   - Policy-based rather than rule-based
   - Good for complex scenarios

### IPv6 Considerations

**Standard CIDR Notation**:
- All modern systems support standard IPv6 CIDR: `2001:db8::/32`
- Use `::` for zero compression: `2001:db8:1234:5678::/64`
- Subnet sizes: Typical allocations are /48 (organization), /64 (network)

**Dual-Stack Handling**:
- nftables: Use `inet` family for unified IPv4/IPv6 rules
- Nginx: Separate directives for IPv4 and IPv6
- Symfony IpUtils: Transparent handling of both

**IPv4-Mapped IPv6**:
- Format: `::ffff:192.0.2.1`
- Symfony IpUtils handles these correctly
- Important for dual-stack environments

---

## Recommendations for FileGator PHP Application

### Current State Analysis

**File**: `/mnt/ai/filegator/backend/Services/Security/Security.php` (lines 59-91)

**Current Implementation**:
```php
if (! empty($config['ip_allowlist'])) {
    $pass = false;
    foreach ($config['ip_allowlist'] as $ip) {
        if ($this->request->getClientIp() == $ip) {  // ⚠️ Exact match only
            $pass = true;
        }
    }
    // ...
}
```

**Limitations**:
- ✗ No CIDR support (`192.168.1.0/24`)
- ✗ No IP range support (`192.168.1.1-192.168.1.254`)
- ✗ No IPv6 support
- ✗ Inefficient loop-based checking
- ✗ No validation of IP format
- ✓ Basic allowlist/denylist support
- ✓ Allowlist evaluated before denylist (correct precedence)

### Recommended Approach

#### 1. Syntax Choice: **Apache-Style CIDR with Symfony IpUtils**

**Rationale**:
- **Familiar**: Most sysadmins know CIDR notation from Apache/Nginx
- **Simple**: Clean array-based configuration
- **Robust**: Leverages battle-tested Symfony library
- **Future-proof**: Full IPv6 support

**Recommended Configuration Syntax**:
```php
'Filegator\Services\Security\Security' => [
    'handler' => '\Filegator\Services\Security\Security',
    'config' => [
        'csrf_protection' => true,
        'csrf_key' => "123456",

        // New improved syntax
        'ip_allowlist' => [
            '192.168.1.0/24',           // CIDR notation
            '10.0.0.0/8',                // Large network
            '2001:db8::/32',             // IPv6 network
            '172.16.50.100',             // Single IP
        ],

        'ip_denylist' => [
            '192.168.1.50',              // Specific blocked IP
            '10.0.1.0/24',               // Blocked subnet
        ],

        'allow_insecure_overlays' => false,
    ],
],
```

#### 2. Evaluation Model: **Allowlist-First with Explicit Precedence**

**Recommended Logic**:
1. If `ip_allowlist` is defined and non-empty:
   - Check if IP matches any allowlist entry → **ALLOW** (skip denylist)
   - If no allowlist match → **DENY** (implicit default-deny)
2. If `ip_allowlist` is empty:
   - Check if IP matches any denylist entry → **DENY**
   - If no denylist match → **ALLOW** (implicit default-allow)

**Rationale**:
- Allowlist mode is explicit: define allowlist → only those IPs allowed
- Denylist mode is implicit: no allowlist → all IPs allowed except denylist
- No ambiguity when both lists are defined (allowlist takes precedence)
- Follows security best practice: whitelist trumps blacklist

#### 3. Implementation Pattern

**Dependencies**:
```bash
# FileGator already uses Symfony components
composer require symfony/http-foundation
```

**Enhanced Security.php Implementation**:
```php
use Symfony\Component\HttpFoundation\IpUtils;

public function init(array $config = [])
{
    // ... existing CSRF code ...

    // IP Access Control
    $clientIp = $this->request->getClientIp();

    // Handle deprecated names
    if (!empty($config['ip_whitelist'])) {
        $config['ip_allowlist'] = $config['ip_whitelist'];
    }
    if (!empty($config['ip_blacklist'])) {
        $config['ip_denylist'] = $config['ip_blacklist'];
    }

    // Validate and normalize IP lists
    $allowlist = $this->normalizeIpList($config['ip_allowlist'] ?? []);
    $denylist = $this->normalizeIpList($config['ip_denylist'] ?? []);

    // Evaluation: Allowlist takes precedence
    if (!empty($allowlist)) {
        // Allowlist mode: default-deny
        if (!IpUtils::checkIp($clientIp, $allowlist)) {
            $this->denyAccess("IP not in allowlist: {$clientIp}");
        }
    } elseif (!empty($denylist)) {
        // Denylist mode: default-allow
        if (IpUtils::checkIp($clientIp, $denylist)) {
            $this->denyAccess("IP in denylist: {$clientIp}");
        }
    }
    // else: No restrictions, allow all
}

private function normalizeIpList(array $ips): array
{
    $normalized = [];
    foreach ($ips as $ip) {
        $ip = trim($ip);
        if (empty($ip)) continue;

        // Validate format (basic check)
        if (!$this->isValidIpOrCidr($ip)) {
            $this->logger->log("Invalid IP/CIDR format: {$ip}");
            continue;
        }

        $normalized[] = $ip;
    }
    return $normalized;
}

private function isValidIpOrCidr(string $ip): bool
{
    // Check for CIDR notation
    if (strpos($ip, '/') !== false) {
        list($addr, $mask) = explode('/', $ip, 2);
        if (!filter_var($addr, FILTER_VALIDATE_IP)) return false;
        if (!is_numeric($mask)) return false;
        $maskNum = (int)$mask;

        // IPv4: 0-32, IPv6: 0-128
        $maxMask = filter_var($addr, FILTER_VALIDATE_IP, FILTER_FLAG_IPV6) ? 128 : 32;
        return $maskNum >= 0 && $maskNum <= $maxMask;
    }

    // Single IP validation
    return filter_var($ip, FILTER_VALIDATE_IP) !== false;
}

private function denyAccess(string $message): void
{
    $this->response->setStatusCode(403);
    $this->response->send();
    $this->logger->log("Forbidden - {$message}");
    exit;
}
```

#### 4. Configuration Examples

**Example 1: Office Network Access Only**
```php
'ip_allowlist' => [
    '203.0.113.0/24',        // Office network
    '198.51.100.10',         // VPN gateway
    '2001:db8::/48',         // IPv6 office network
],
'ip_denylist' => [],         // Not used in allowlist mode
```
Result: Only IPs from office networks can access. All others denied.

**Example 2: Public Access with Known Bad Actors Blocked**
```php
'ip_allowlist' => [],        // Empty = denylist mode
'ip_denylist' => [
    '192.0.2.50',            // Specific attacker
    '198.51.100.0/24',       // Malicious subnet
    '2001:db8:bad::/48',     // IPv6 malicious network
],
```
Result: All IPs can access except those in denylist.

**Example 3: Admin Section Protection**
```php
// In a hypothetical per-route configuration
'/admin' => [
    'ip_allowlist' => [
        '127.0.0.1',         // Localhost
        '::1',               // IPv6 localhost
        '10.0.0.0/8',        // Internal network
    ],
],
```

#### 5. Migration Path

**Phase 1**: Add new functionality while maintaining backward compatibility
```php
// Old exact-match still works
'ip_allowlist' => ['192.168.1.50']

// New CIDR notation also works
'ip_allowlist' => ['192.168.1.0/24']
```

**Phase 2**: Update documentation with examples

**Phase 3**: Deprecate old behavior (if needed)

### Error-Resistance Features

1. **Validation**:
   - Log invalid IP formats instead of failing silently
   - Skip malformed entries but continue processing valid ones

2. **Safe Defaults**:
   - Empty allowlist + empty denylist = allow all (backward compatible)
   - Non-empty allowlist = default deny (secure by default)

3. **Clear Precedence**:
   - Allowlist always wins (no ambiguity)
   - Document this prominently

4. **IPv4/IPv6 Transparent**:
   - Admin doesn't need to think about IP version
   - Symfony IpUtils handles both uniformly

### Performance Considerations

**Symfony IpUtils Performance**:
- O(n) complexity for checking against list
- Acceptable for typical lists (< 100 entries)
- For large lists (> 1000 entries), consider:
  - Caching expanded ranges
  - Using trie-based data structures
  - External IP filtering (nginx/Cloudflare)

**Optimization for FileGator**:
- Move IP checks early in request lifecycle
- Cache IpUtils results for repeated checks in same request
- Consider nginx/Apache layer filtering for high-traffic scenarios

---

## Industry Best Practices Summary

### 1. Syntax Standards

**Universal Format**: CIDR notation is industry standard
- Apache, Nginx, iptables, AWS, Cloudflare all support it
- Format: `192.168.1.0/24` (IPv4), `2001:db8::/32` (IPv6)
- Most familiar to system administrators

### 2. Evaluation Order

**First-Match**: Most common and predictable
- Used by Nginx, iptables, Cloudflare
- Simple mental model: "first rule that matches wins"
- Requires careful ordering (specific before general)

**Best Practice**: Allowlist evaluation before denylist
- Security principle: explicit allow > explicit deny > default
- Prevents bypass through conflicting rules

### 3. IPv6 Strategy

**Dual-Stack Ready**:
- Always support both IPv4 and IPv6
- Use libraries that handle both transparently
- Don't rely on IPv4-only assumptions

**IPv6 Gotchas**:
- IPv4-mapped IPv6 addresses: `::ffff:192.0.2.1`
- Link-local addresses: `fe80::/10`
- Unique local addresses: `fc00::/7`

### 4. Security Principles

**Defense in Depth**:
- IP filtering alone is insufficient
- Combine with authentication, rate limiting, CAPTCHA
- Modern approach: Zero Trust + MFA + IP filtering

**Regular Maintenance**:
- Review and update IP lists regularly
- Remove stale entries
- Monitor for bypass attempts
- Automate where possible

**Avoid Common Pitfalls**:
- Don't use substring matching for IPs (`contains "192.168"` is dangerous)
- Don't forget to test IPv6
- Don't hardcode IPs in application code
- Don't skip validation of IP list entries

### 5. Modern Alternatives

**Static IP Limitations**:
- IP spoofing (though difficult for TCP)
- NAT and proxies complicate IP-based auth
- Mobile users have changing IPs
- IPv4 exhaustion leads to shared IPs

**Complement with**:
- OAuth2 / OIDC for user identity
- mTLS for service-to-service
- Device fingerprinting
- Behavioral analytics
- Rate limiting and CAPTCHA

---

## Conclusion

### Recommended Solution for FileGator

**Syntax**: Apache-style CIDR notation in PHP arrays
**Library**: Symfony HttpFoundation IpUtils
**Evaluation**: Allowlist-first with explicit precedence
**IPv6**: Full support via IpUtils

**Key Benefits**:
- ✅ Familiar to administrators (Apache/Nginx experience transfers)
- ✅ Simple configuration (clean PHP array syntax)
- ✅ Robust implementation (battle-tested Symfony library)
- ✅ Error-resistant (validation + clear precedence rules)
- ✅ Future-proof (full IPv6 support)
- ✅ Performant (efficient CIDR matching)
- ✅ Backward compatible (existing exact-match configs still work)

**Implementation Effort**: Low (FileGator already uses Symfony components)

**Risk**: Minimal (additive change, maintains backward compatibility)

---

## Additional Research Resources

### IP Address Fundamentals
- [Classless Inter-Domain Routing - Wikipedia](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing)
- [A Comprehensive Guide to IPv4 and IPv6 CIDR | IP2Location.com](https://blog.ip2location.com/knowledge-base/a-comprehensive-guide-to-ipv4-and-ipv6-cidr/)

### Security Best Practices
- [IP Whitelisting in 2023: Everything You Need to Know](https://www.goodaccess.com/ip-whitelisting)
- [IP Whitelisting vs. Blacklisting for APIs](https://blog.dreamfactory.com/ip-whitelisting-vs.-blacklisting-for-apis)
- [Whitelisting vs Blacklisting: Key Differences Explained](https://instasafe.com/blog/whitelisting-vs-blacklisting-whats-the-difference/)

### Implementation Guides
- [How to Block IP Address with .htaccess](https://htaccessbook.com/block-ip-address/)
- [NGINX: How to Allow Only a Range of IP Addresses - Sling Academy](https://www.slingacademy.com/article/nginx-how-to-allow-only-a-range-of-ip-addresses/)
- [Iptables and IPv6: Configure Firewall Rules for IPv6 Networks](https://www.fosslinux.com/100839/iptables-and-ipv6-configuring-firewall-rules-for-ipv6-networks.htm)

---

**Document Version**: 1.0
**Last Updated**: 2025-12-09
**Author**: Research Agent
**Status**: Ready for Implementation Planning
