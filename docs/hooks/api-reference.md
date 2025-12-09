# Hooks API Reference

This document provides the complete API reference for the FileGator hooks system.

## HooksInterface

The main interface for interacting with the hooks system.

```php
namespace Filegator\Services\Hooks;

interface HooksInterface
{
    public function trigger(string $hookName, array $data = []): array;
    public function register(string $hookName, callable $callback, int $priority = 0): void;
    public function getHooks(string $hookName): array;
    public function hasHooks(string $hookName): bool;
}
```

### Methods

#### trigger()

Execute all hooks for a given event.

```php
public function trigger(string $hookName, array $data = []): array
```

**Parameters:**
- `$hookName` (string) - The hook event name (e.g., 'onUpload')
- `$data` (array) - Data to pass to hook scripts

**Returns:** Array of results from all executed hooks

**Example:**
```php
$hooks = $container->get(HooksInterface::class);

$results = $hooks->trigger('onUpload', [
    'file_path' => '/uploads/document.pdf',
    'file_name' => 'document.pdf',
    'file_size' => 1048576,
    'user' => 'john',
]);

foreach ($results as $result) {
    if (!$result['success']) {
        // Handle hook failure
        error_log("Hook failed: " . $result['error']);
    }
}
```

---

#### register()

Register an in-memory callback hook.

```php
public function register(string $hookName, callable $callback, int $priority = 0): void
```

**Parameters:**
- `$hookName` (string) - The hook event name
- `$callback` (callable) - Function to execute when hook triggers
- `$priority` (int) - Execution priority (higher = earlier, default: 0)

**Example:**
```php
// Simple callback
$hooks->register('onUpload', function($data) {
    // Process upload
    return ['processed' => true];
});

// With priority (executes first)
$hooks->register('onUpload', function($data) {
    // Validate first
    if (empty($data['file_path'])) {
        return false; // Stop execution
    }
    return true;
}, 100);

// Class method
$hooks->register('onUpload', [$myPlugin, 'handleUpload']);
```

---

#### getHooks()

Get all registered hooks for an event.

```php
public function getHooks(string $hookName): array
```

**Returns:** Array of hook definitions with 'type' ('callback' or 'script') and relevant data

**Example:**
```php
$hooks = $container->get(HooksInterface::class);
$uploadHooks = $hooks->getHooks('onUpload');

foreach ($uploadHooks as $hook) {
    echo "Type: " . $hook['type'] . "\n";
    if ($hook['type'] === 'script') {
        echo "Path: " . $hook['path'] . "\n";
    }
}
```

---

#### hasHooks()

Check if any hooks are registered for an event.

```php
public function hasHooks(string $hookName): bool
```

**Example:**
```php
if ($hooks->hasHooks('onUpload')) {
    // Execute upload hooks
    $hooks->trigger('onUpload', $uploadData);
}
```

---

## Hooks Class

The default implementation of HooksInterface.

### Additional Methods

#### setEnabled()

Enable or disable hooks globally.

```php
public function setEnabled(bool $enabled): void
```

---

#### isEnabled()

Check if hooks are currently enabled.

```php
public function isEnabled(): bool
```

---

#### getHooksPath()

Get the configured hooks directory path.

```php
public function getHooksPath(): string
```

---

#### setLogger()

Set a logger for error reporting.

```php
public function setLogger(?LoggerInterface $logger): void
```

---

## Hook Script API

### Available Variables

Every hook script has access to:

| Variable | Type | Description |
|----------|------|-------------|
| `$hookData` | array | Event data passed to the hook |

### Return Values

| Return Type | Behavior |
|-------------|----------|
| `true` | Success, continue chain |
| `false` | Stop hook chain |
| `null` | Same as `true` |
| `array` | Custom result, continues chain |
| `['action' => 'stop']` | Stop hook chain |
| `['action' => 'continue']` | Continue chain |

### Script Template

```php
<?php
/**
 * @hook [hookName]
 * @description [What this hook does]
 */

// Access hook data
$filePath = $hookData['file_path'] ?? null;

// Validation
if (!$filePath) {
    return ['status' => 'error', 'message' => 'No file path'];
}

// Processing
try {
    // Your logic here
    $result = processFile($filePath);
} catch (\Throwable $e) {
    return [
        'action' => 'continue',
        'status' => 'error',
        'error' => $e->getMessage(),
    ];
}

// Return result
return [
    'action' => 'continue',
    'status' => 'success',
    'result' => $result,
];
```

---

## Result Structure

Each hook execution returns a result array:

```php
[
    'type' => 'callback',    // or 'script'
    'success' => true,       // bool
    'result' => mixed,       // Return value from hook
    'error' => null,         // Error message if failed
    'path' => '/path/...',   // Script path (for scripts only)
]
```

---

## Configuration Reference

```php
'Filegator\Services\Hooks\HooksInterface' => [
    'handler' => '\Filegator\Services\Hooks\Hooks',
    'config' => [
        // Enable/disable hooks globally
        'enabled' => true,

        // Path to hooks directory
        'hooks_path' => __DIR__.'/private/hooks',

        // Maximum execution time per script (seconds)
        'timeout' => 30,

        // Reserved for future async support
        'async' => false,
    ],
],
```

---

## Dependency Injection

Access hooks via the container:

```php
use Filegator\Services\Hooks\HooksInterface;

class MyController
{
    protected $hooks;

    public function __construct(HooksInterface $hooks = null)
    {
        $this->hooks = $hooks;
    }

    public function upload()
    {
        // ... upload logic ...

        if ($this->hooks) {
            $this->hooks->trigger('onUpload', [
                'file_path' => $path,
                'file_name' => $name,
            ]);
        }
    }
}
```

---

## Error Handling

Hooks handle errors gracefully:

1. **Script errors** are caught and logged
2. **Timeouts** are enforced per script
3. **Invalid hooks** are silently ignored
4. **Missing directories** are created automatically

```php
// Hooks never throw to calling code
$results = $hooks->trigger('onUpload', $data);

// Check individual results for errors
foreach ($results as $result) {
    if (!$result['success']) {
        // Log or handle error
        $this->logger->log("Hook error: " . $result['error']);
    }
}
```
