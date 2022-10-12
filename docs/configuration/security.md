---
currentMenu: security
---

## Configuring Security service

Simple security service is included in the script by default. This service provides:

- Basic session-based [CSRF](https://en.wikipedia.org/wiki/Cross-site_request_forgery) protection
- IP allow list
- IP deny list

```
        'Filegator\Services\Security\Security' => [
            'handler' => '\Filegator\Services\Security\Security',
            'config' => [
                'csrf_protection' => true,
                'csrf_key' => "123456", // randomize this
                'ip_allowlist' => [],
                'ip_denylist' => [
                    '172.16.1.2',
                    '172.16.3.4',
                ],
            ],
        ],
```

If you set `ip_allowlist` then only users coming from listed IP addresses will be able to use the script.
