# Optional Service Injection in FileGator

## Overview

This document explains how optional services (like `PathACLInterface` and `HooksInterface`) are injected into controllers, and provides an alternative architectural solution for future consideration.

## Current Implementation (Router-based Injection)

### The Problem

PHP-DI's default parameter resolver chain processes resolvers in this order:

1. `DefinitionParameterResolver`
2. `NumericArrayResolver`
3. `AssociativeArrayResolver`
4. `DefaultValueResolver` ← **Runs before container check**
5. `TypeHintContainerResolver` ← **Never reached for optional params**

For optional constructor parameters like `PathACLInterface $pathacl = null`:
- `DefaultValueResolver` sees the `= null` default and immediately returns `null`
- `TypeHintContainerResolver` never gets a chance to check if the service exists in the container
- Result: Optional services are always `null` even when properly registered

### The Solution

In `Router.php`, we explicitly inject optional services before calling the controller:

```php
// Explicitly inject optional services that PHP-DI doesn't autowire for optional parameters
$optionalServices = [
    'pathacl' => 'Filegator\Services\PathACL\PathACLInterface',
    'hooks' => 'Filegator\Services\Hooks\HooksInterface',
];

foreach ($optionalServices as $paramName => $serviceKey) {
    if ($this->container->has($serviceKey)) {
        $params[$paramName] = $this->container->get($serviceKey);
    }
}

$this->container->call([$controller, $action], $params);
```

### Pros
- 100% backwards compatible
- Guaranteed to work
- Simple to understand
- Easy to add new optional services

### Cons
- Manual maintenance required when adding new optional services
- Breaks pure DI pattern (explicit service names in Router)

---

## Alternative Solution: Override `getInvoker()` (Future Option)

A more elegant solution is to override PHP-DI's `getInvoker()` method to reorder the resolver chain. This fixes the root cause at the container level.

### Implementation

Replace the contents of `/backend/Container/Container.php` with:

```php
<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Container;

use DI\Container as PHPDIContainer;
use DI\Definition\Resolver\DefinitionResolver;
use Invoker\Invoker;
use Invoker\InvokerInterface;
use Invoker\ParameterResolver\AssociativeArrayResolver;
use Invoker\ParameterResolver\Container\TypeHintContainerResolver;
use Invoker\ParameterResolver\DefaultValueResolver;
use Invoker\ParameterResolver\NumericArrayResolver;
use Invoker\ParameterResolver\ResolverChain;
use Invoker\ParameterResolver\TypeHintResolver;

class Container extends PHPDIContainer implements ContainerInterface
{
    /**
     * @var InvokerInterface|null
     */
    private $customInvoker = null;

    /**
     * Override getInvoker() to reorder the parameter resolver chain.
     *
     * PHP-DI's default order puts DefaultValueResolver BEFORE TypeHintContainerResolver,
     * which means optional parameters (e.g., PathACLInterface $pathacl = null) always
     * receive their default value instead of being resolved from the container.
     *
     * Our order: TypeHintContainerResolver BEFORE DefaultValueResolver
     * This ensures registered services are injected even for optional parameters.
     *
     * @return InvokerInterface
     */
    protected function getInvoker(): InvokerInterface
    {
        if ($this->customInvoker === null) {
            // Get the definition resolver from parent (needed for DefinitionParameterResolver)
            // Note: This requires access to protected property, may need reflection
            $definitionResolver = $this->getDefinitionResolver();

            $parameterResolver = new ResolverChain([
                new \DI\Invoker\DefinitionParameterResolver($definitionResolver),
                new NumericArrayResolver(),
                new AssociativeArrayResolver(),
                new TypeHintContainerResolver($this->delegateContainer ?? $this), // BEFORE defaults
                new DefaultValueResolver(), // AFTER container check
            ]);

            $this->customInvoker = new Invoker($parameterResolver, $this);
        }

        return $this->customInvoker;
    }

    /**
     * Get the definition resolver instance.
     *
     * Note: This may require using reflection to access the protected property
     * from the parent class if not directly accessible.
     *
     * @return DefinitionResolver
     */
    private function getDefinitionResolver(): DefinitionResolver
    {
        // Option 1: If definitionResolver is accessible
        // return $this->definitionResolver;

        // Option 2: Use reflection to access protected property
        $reflection = new \ReflectionClass(PHPDIContainer::class);
        $property = $reflection->getProperty('definitionResolver');
        $property->setAccessible(true);
        return $property->getValue($this);
    }
}
```

### Important Notes

1. **Reflection Required**: The parent's `definitionResolver` property is protected, so reflection is needed to access it.

2. **Testing Required**: This approach modifies core container behavior. Thorough testing is recommended:
   - Test all controllers with optional parameters
   - Test controllers with required parameters still work
   - Test that services not in container still get `null`
   - Test error handling when container lookup fails

3. **PHP-DI Version Compatibility**: This relies on PHP-DI's internal structure. May need adjustment if PHP-DI is upgraded.

4. **If Implementing This Solution**:
   - Remove the explicit injection code from `Router.php`
   - Add comprehensive tests
   - Document the change in CHANGELOG

### Pros
- Fixes root cause at container level
- No manual service list maintenance
- Works for ALL optional interface parameters automatically
- Cleaner DI pattern

### Cons
- More complex implementation
- Relies on PHP-DI internals (reflection to access protected property)
- May break with PHP-DI upgrades
- Requires thorough testing

---

## Adding New Optional Services

### With Current Implementation (Router-based)

1. Add the service to `configuration.php`
2. Add the parameter name and interface to the `$optionalServices` array in `Router.php`:

```php
$optionalServices = [
    'pathacl' => 'Filegator\Services\PathACL\PathACLInterface',
    'hooks' => 'Filegator\Services\Hooks\HooksInterface',
    'newservice' => 'Filegator\Services\NewService\NewServiceInterface', // Add here
];
```

3. Add the optional parameter to controller constructors as needed

### With Alternative Implementation (getInvoker override)

1. Add the service to `configuration.php`
2. Add the optional parameter to controller constructors
3. Done - no Router changes needed

---

## Related Files

- `/backend/Services/Router/Router.php` - Contains explicit injection code
- `/backend/Container/Container.php` - Custom container class
- `/backend/Controllers/FileController.php` - Uses PathACLInterface
- `/backend/Controllers/DownloadController.php` - Uses PathACLInterface
- `/backend/Controllers/UploadController.php` - Uses PathACLInterface
- `/configuration.php` - Service registration
- `/vendor/php-di/php-di/src/Container.php` - Parent container (reference)
- `/vendor/php-di/invoker/src/ParameterResolver/` - Resolver implementations (reference)

---

## References

- [PHP-DI Documentation](https://php-di.org/doc/)
- [PHP-DI Invoker](https://github.com/PHP-DI/Invoker)
- Swarm Analysis: Agent ID `48199f13` - Detailed PHP-DI resolution flow analysis
