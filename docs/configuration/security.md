---
currentMenu: security
---

## Configuring Security service

Simple security service is included in the script by default. This service provides:

- Basic session-based [CSRF](https://en.wikipedia.org/wiki/Cross-site_request_forgery) protection
- IP whitelisting
- IP blacklisting

```
        'Filegator\Services\Security\Security' => [
            'handler' => '\Filegator\Services\Security\Security',
            'config' => [
                'csrf_protection' => true,
                'ip_whitelist' => [],
                'ip_blacklist' => [
                    '172.16.1.2',
                    '172.16.3.4',
                ],
            ],
        ],
```

If you set `ip_whitelist` then only users coming from listed IP addresses will be able to use the script.
