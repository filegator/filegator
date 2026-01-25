# PHP-DI Dependency Resolution Flow Analysis

## Code Quality Analysis Report

### Summary
- **Overall Quality Score**: 7/10
- **Files Analyzed**: 6
- **Critical Issues Found**: 1 major bug
- **Technical Debt Estimate**: 2-4 hours

---

## Executive Summary

**THE ROOT CAUSE**: The custom `Container::call()` override is NOT being invoked during controller instantiation. The parent class's `getInvoker()` method creates a DIFFERENT invoker that is used for constructor parameter resolution.

---

## Critical Issue: Two Separate Invoker Instances

### The Problem

There are **TWO DIFFERENT INVOKER INSTANCES** with **DIFFERENT RESOLVER ORDERS**:

#### 1. Custom Invoker (in Filegator\Container\Container::call())
```php
// Lines 35-46 in backend/Container/Container.php
$parameterResolver = new ResolverChain([
    new NumericArrayResolver,
    new AssociativeArrayResolver,
    new TypeHintContainerResolver($this),  // ✅ BEFORE DefaultValueResolver
    new DefaultValueResolver,              // ✅ After TypeHint
]);
$this->customInvoker = new Invoker($parameterResolver, $this);
```

#### 2. Parent's Invoker (in DI\Container::getInvoker())
```php
// Lines 412-418 in vendor/php-di/php-di/src/Container.php
$parameterResolver = new ResolverChain([
    new DefinitionParameterResolver($this->definitionResolver),
    new NumericArrayResolver,
    new AssociativeArrayResolver,
    new DefaultValueResolver,                              // ❌ BEFORE TypeHint
    new TypeHintContainerResolver($this->delegateContainer), // ❌ After Default
]);
$this->invoker = new Invoker($parameterResolver, $this);
```

---

## Detailed Resolution Flow Analysis

### Step-by-Step Execution Path

```
1. Router::init() (line 78)
   └─> $this->container->call([$controller, $action], $params);
       ↓
2. Filegator\Container\Container::call() (line 33-48) ✅ CORRECT METHOD CALLED
   └─> return $this->customInvoker->call($callable, $parameters);
       ↓
3. Invoker::call() resolves parameters via ResolverChain
   └─> getParameters() calls each resolver in order
       ↓
4. PROBLEM: Constructor resolution happens INSIDE make/get
   └─> When FileController class needs to be instantiated:
       ├─> Container::get('Filegator\Controllers\FileController')
       ├─> getDefinition() returns ObjectDefinition
       ├─> resolveDefinition() calls DefinitionResolver
       └─> ObjectCreator uses PARENT'S $this->invoker ❌ NOT custom invoker!
```

### Where Constructor Resolution Happens

**Constructor parameters are NOT resolved by `Container::call()`!**

They are resolved by:
```
Container::get/make()
  └─> resolveDefinition()
      └─> DefinitionResolver::resolve()
          └─> ObjectCreator::create()
              └─> $this->resolver->getParameters()  // Uses PARENT's invoker
                  └─> DefaultValueResolver runs BEFORE TypeHintContainerResolver
```

---

## Why the Custom call() Override Doesn't Work

### The call() Method is Used AFTER Construction

```php
// Router.php line 78
$this->container->call([$controller, $action], $params);
```

This breaks down to:
```php
$this->container->call(['\Filegator\Controllers\FileController', 'changeDirectory'], $params);
```

**Order of operations:**
1. **Instantiate** `FileController` class → Uses `Container::get()` → Uses PARENT's invoker
2. **Call** `changeDirectory` method → Uses `Container::call()` → Uses CUSTOM invoker

**The custom invoker only affects METHOD calls, not CONSTRUCTOR calls!**

---

## TypeHintContainerResolver Analysis

### How It Checks for Services

```php
// Lines 38-60 in TypeHintContainerResolver.php
foreach ($parameters as $index => $parameter) {
    $parameterType = $parameter->getType();
    if (! $parameterType instanceof ReflectionNamedType) continue;
    if ($parameterType->isBuiltin()) continue;

    $parameterClass = $parameterType->getName();  // Returns: "Filegator\Services\PathACL\PathACLInterface"

    if ($this->container->has($parameterClass)) {  // ✅ This SHOULD return true
        $resolvedParameters[$index] = $this->container->get($parameterClass);
    }
}
```

### Container::has() Implementation

```php
// Lines 210-229 in vendor/php-di/php-di/src/Container.php
public function has($name)
{
    // Check resolved entries first
    if (array_key_exists($name, $this->resolvedEntries)) {
        return true;  // ✅ PathACL IS in resolvedEntries after App.php registration
    }

    // Otherwise check if definition exists and is resolvable
    $definition = $this->getDefinition($name);
    if ($definition === null) {
        return false;
    }

    return $this->definitionResolver->isResolvable($definition);
}
```

### Service Registration in App.php

```php
// Lines 31-35 in backend/App.php
foreach ($config->get('services', []) as $key => $service) {
    $instance = $container->get($service['handler']);
    $container->set($key, $instance);  // Sets in $resolvedEntries
    $instance->init(isset($service['config']) ? $service['config'] : []);
}
```

**Registration:**
- Key: `'Filegator\Services\PathACL\PathACLInterface'` (from configuration.php line 112)
- Value: PathACL instance
- Storage: `$this->resolvedEntries[$name]` (Container.php line 302)

---

## The Resolution Order Problem

### Default PHP-DI Order (Parent's getInvoker)
```
1. DefinitionParameterResolver  ← Checks explicit definitions
2. NumericArrayResolver          ← Checks numeric array params
3. AssociativeArrayResolver      ← Checks associative array params
4. DefaultValueResolver          ← ❌ RETURNS NULL for optional params
5. TypeHintContainerResolver     ← ✗ NEVER REACHED for optional params
```

### Custom Order (Our call() override)
```
1. NumericArrayResolver
2. AssociativeArrayResolver
3. TypeHintContainerResolver     ← ✅ CHECKS CONTAINER FIRST
4. DefaultValueResolver          ← ✅ Falls back if not in container
```

### Why DefaultValueResolver "Wins"

From `DefaultValueResolver.php` (php-di/invoker package):
```php
public function getParameters(/* ... */) : array
{
    $parameters = $reflection->getParameters();

    foreach ($parameters as $index => $parameter) {
        if (array_key_exists($index, $resolvedParameters)) {
            continue;  // Already resolved, skip
        }

        if ($parameter->isOptional()) {
            $resolvedParameters[$index] = $parameter->getDefaultValue();  // Returns NULL
        }
    }

    return $resolvedParameters;
}
```

**Once DefaultValueResolver sets `$resolvedParameters[$index] = null`, the ResolverChain stops trying for that parameter!**

---

## Proof Points

### 1. Service IS Registered Correctly
```php
// configuration.php lines 112-118
'Filegator\Services\PathACL\PathACLInterface' => [
    'handler' => '\Filegator\Services\PathACL\PathACL',
    'config' => [
        'enabled' => true,
        'acl_config_file' => __DIR__.'/private/acl_config.php',
    ],
],
```

### 2. Container::has() WOULD Return True
```php
// The key format matches exactly
$parameterClass = "Filegator\\Services\\PathACL\\PathACLInterface";
array_key_exists($parameterClass, $this->resolvedEntries);  // TRUE after App.php runs
```

### 3. Custom call() IS Called for Method Invocation
```php
// Router.php line 78
$this->container->call([$controller, $action], $params);
// This DOES use custom invoker, but AFTER construction is complete
```

### 4. Constructor Uses Different Resolver
```php
// ObjectCreator uses parent's invoker, not custom invoker
// vendor/php-di/php-di/src/Definition/Resolver/ObjectCreator.php
$args = $this->parameterResolver->resolveParameters(/* ... */);
// This is parent's invoker with wrong resolver order
```

---

## Code Smell Detection

### 1. **Inappropriate Architecture** (High Severity)
- **Location**: `backend/Container/Container.php` lines 23-49
- **Issue**: Overriding `call()` doesn't affect constructor resolution
- **Impact**: Optional interface parameters cannot be injected

### 2. **Feature Envy** (Medium Severity)
- **Location**: Multiple controllers accessing `$this->pathacl`
- **Issue**: Controllers check if service exists before using it
- **Pattern**: `if (!$this->pathacl || !$this->pathacl->isEnabled())`
- **Better**: Service should always be available, even if disabled internally

### 3. **Dead Code** (Low Severity)
- **Location**: `backend/Container/Container.php` custom invoker
- **Issue**: Custom invoker works for method calls but doesn't solve constructor injection
- **Impact**: Misleading comments suggest it solves the problem

---

## Root Cause Summary

### The Actual Problem

**PHP-DI has TWO SEPARATE CODE PATHS for parameter resolution:**

1. **Constructor parameters** → Resolved by `ObjectCreator` → Uses `Container::$invoker` (parent's instance)
2. **Method parameters** → Resolved by `Container::call()` → Uses custom invoker (if overridden)

**Our custom `call()` override only affects #2, not #1!**

---

## Solutions

### Solution 1: Override getInvoker() (RECOMMENDED)
```php
class Container extends PHPDIContainer implements ContainerInterface
{
    protected function getInvoker() : InvokerInterface
    {
        if (! $this->invoker) {
            $parameterResolver = new ResolverChain([
                new DefinitionParameterResolver($this->definitionResolver),
                new NumericArrayResolver,
                new AssociativeArrayResolver,
                new TypeHintContainerResolver($this->delegateContainer), // BEFORE default
                new DefaultValueResolver,                                // AFTER typehint
            ]);

            $this->invoker = new Invoker($parameterResolver, $this);
        }

        return $this->invoker;
    }
}
```

**Pros:**
- Fixes constructor injection
- Maintains method injection
- Minimal changes

**Cons:**
- Relies on protected method (could break in future PHP-DI versions)

### Solution 2: Explicit Constructor Definition
```php
// In App.php or container configuration
$container->set('Filegator\Controllers\FileController', function($c) {
    return new FileController(
        $c->get(Config::class),
        $c->get(Session::class),
        $c->get(AuthInterface::class),
        $c->get(Filesystem::class),
        $c->get(PathACLInterface::class),  // Explicit injection
        $c->get(HooksInterface::class)     // Explicit injection
    );
});
```

**Pros:**
- Guaranteed to work
- Explicit control

**Cons:**
- Verbose
- Needs updating when constructor changes
- Loses autowiring benefits

### Solution 3: Custom DefinitionResolver
```php
class CustomObjectCreator extends ObjectCreator
{
    // Override parameter resolution to use correct resolver order
}
```

**Pros:**
- Most architecturally sound

**Cons:**
- Complex implementation
- High maintenance burden

### Solution 4: Make Parameters Required
```php
// FileController.php line 41
public function __construct(
    Config $config,
    Session $session,
    AuthInterface $auth,
    Filesystem $storage,
    PathACLInterface $pathacl,  // Remove = null
    HooksInterface $hooks       // Remove = null
)
```

**Pros:**
- Forces dependency injection
- Clearer contract

**Cons:**
- Breaks backward compatibility
- Requires services to always be registered

---

## Refactoring Opportunities

### 1. Service Locator Pattern → Dependency Injection
**Current**: Controllers check if services exist
```php
if (!$this->pathacl || !$this->pathacl->isEnabled()) { ... }
```

**Better**: Always inject services, use null object pattern
```php
// Create NullPathACL that always returns true
class NullPathACL implements PathACLInterface {
    public function isEnabled() { return false; }
    public function checkPermission(...) { return true; }
}
```

### 2. Extract ACL Logic to Middleware
**Benefit**: Centralized permission checking before controller execution
**Impact**: Removes repetitive `checkPathACL()` calls from every controller method

### 3. Use PHP 8.0+ Constructor Property Promotion
```php
public function __construct(
    protected Config $config,
    protected Session $session,
    protected AuthInterface $auth,
    protected Filesystem $storage,
    protected ?PathACLInterface $pathacl = null,
    protected ?HooksInterface $hooks = null
) {}
```

---

## Positive Findings

✅ **Clean architecture**: Services properly abstracted behind interfaces
✅ **Consistent patterns**: All services follow same initialization pattern
✅ **Good documentation**: Comments explain intent of custom container
✅ **Proper separation**: Business logic separated from framework concerns
✅ **Type safety**: Full type hints on all parameters

---

## Recommendations

### Immediate Action (Priority: CRITICAL)
1. Implement **Solution 1** (Override getInvoker) to fix dependency injection
2. Add integration test to verify PathACL injection works
3. Update documentation to reflect actual resolution behavior

### Short-term (Priority: HIGH)
1. Consider implementing null object pattern for optional services
2. Add logging to verify service injection during development
3. Create diagnostic command to show registered services

### Long-term (Priority: MEDIUM)
1. Migrate to middleware-based ACL checking
2. Consider upgrading to PHP-DI 7+ with improved autowiring
3. Evaluate service provider pattern for complex initialization

---

## Technical Debt Assessment

- **Complexity**: Medium (resolver chain understanding required)
- **Risk**: High (affects all optional interface injections)
- **Effort**: 2-4 hours to implement and test Solution 1
- **Impact**: High (enables proper dependency injection throughout application)

---

## Conclusion

The custom `Container::call()` override is architecturally sound but targets the wrong extension point. The parent class's `getInvoker()` method creates a separate invoker instance used for constructor parameter resolution, which maintains the default resolver order (DefaultValueResolver before TypeHintContainerResolver).

The PathACL service IS correctly registered and WOULD be injected if the resolver order were correct during constructor resolution. The fix requires overriding `getInvoker()` instead of (or in addition to) `call()`.

**Estimated Fix Time**: 30 minutes implementation + 1.5 hours testing = 2 hours total
**Risk Level**: Low (well-understood problem with proven solution)
**Priority**: Critical (blocks PathACL feature functionality)
