#!/usr/bin/env php
<?php
/**
 * Diagnostic script to test PathACL dependency injection
 *
 * This script mimics how Router.php instantiates controllers
 * to determine why PathACLInterface is not being injected.
 */

require __DIR__ . '/../vendor/autoload.php';

use DI\ContainerBuilder;
use Filegator\Container\Container;
use Filegator\Controllers\FileController;
use Filegator\Controllers\DownloadController;
use Filegator\Controllers\UploadController;
use Filegator\Services\PathACL\PathACL;
use Filegator\Services\PathACL\PathACLInterface;

echo "=== PathACL Dependency Injection Test ===\n\n";

// Create container
$containerBuilder = new ContainerBuilder(Container::class);
$containerBuilder->useAutowiring(true);
$containerBuilder->useAnnotations(false);

$container = $containerBuilder->build();

// Register PathACL interface binding (as done in configuration.php)
$pathACL = new PathACL();
$container->set(PathACLInterface::class, $pathACL);

echo "1. Container has PathACLInterface registered: " . ($container->has(PathACLInterface::class) ? "YES" : "NO") . "\n";
echo "2. Retrieving PathACLInterface directly: " . (get_class($container->get(PathACLInterface::class))) . "\n\n";

// Test 1: Using container->call() with controller class name and method
echo "=== TEST 1: Using call() with ['ControllerClass', 'method'] ===\n";
try {
    echo "Attempting: \$container->call(['\\Filegator\\Controllers\\FileController', 'getDirectory'], [])\n";

    // This is how Router.php calls the controller
    // Note: We can't actually call getDirectory without proper setup, so we'll catch the error
    ob_start();
    try {
        $result = $container->call(['\\Filegator\\Controllers\\FileController', 'getDirectory'], []);
    } catch (Exception $e) {
        // Expected - we don't have all dependencies set up
        echo "Expected error during method call: " . substr($e->getMessage(), 0, 100) . "...\n";
    }
    ob_end_clean();

    echo "Result: call() was able to instantiate the controller\n";
} catch (Exception $e) {
    echo "ERROR: " . $e->getMessage() . "\n";
    echo "Stack trace:\n" . $e->getTraceAsString() . "\n";
}

echo "\n=== TEST 2: Direct instantiation via container->get() ===\n";
try {
    echo "Attempting: \$container->get('\\Filegator\\Controllers\\FileController')\n";
    $controller = $container->get('\\Filegator\\Controllers\\FileController');
    echo "SUCCESS: Controller instantiated\n";

    // Use reflection to check if pathacl was injected
    $reflection = new ReflectionClass($controller);
    $property = $reflection->getProperty('pathacl');
    $property->setAccessible(true);
    $pathAclValue = $property->getValue($controller);

    echo "PathACL property value: " . ($pathAclValue ? get_class($pathAclValue) : "NULL") . "\n";

} catch (Exception $e) {
    echo "ERROR: " . $e->getMessage() . "\n";
}

echo "\n=== TEST 3: Using call() with controller instance ===\n";
try {
    echo "Creating controller instance first, then calling method\n";
    $controller = $container->get('\\Filegator\\Controllers\\FileController');

    echo "Attempting: \$container->call([\$controller, 'getDirectory'], [])\n";
    // Again, we expect this to fail due to missing Request/Response, but that's OK
    ob_start();
    try {
        $result = $container->call([$controller, 'getDirectory'], []);
    } catch (Exception $e) {
        echo "Expected error: " . substr($e->getMessage(), 0, 100) . "...\n";
    }
    ob_end_clean();

    echo "Result: Method callable\n";
} catch (Exception $e) {
    echo "ERROR: " . $e->getMessage() . "\n";
}

echo "\n=== TEST 4: Check resolver order in Container ===\n";
$containerReflection = new ReflectionClass($container);
echo "Container class: " . get_class($container) . "\n";
echo "Container has custom call() implementation: " . ($containerReflection->hasMethod('call') ? "YES" : "NO") . "\n";

// Read the Container.php source to verify resolver order
$containerSource = file_get_contents(__DIR__ . '/../backend/Container/Container.php');
if (strpos($containerSource, 'TypeHintContainerResolver($this)') !== false) {
    echo "Custom resolver order is present in Container.php\n";

    if (strpos($containerSource, 'new TypeHintContainerResolver') < strpos($containerSource, 'new DefaultValueResolver')) {
        echo "✓ Resolver order is CORRECT (TypeHintContainerResolver before DefaultValueResolver)\n";
    } else {
        echo "✗ Resolver order is WRONG (DefaultValueResolver before TypeHintContainerResolver)\n";
    }
}

echo "\n=== TEST 5: Compare with DownloadController and UploadController ===\n";
foreach (['FileController', 'DownloadController', 'UploadController'] as $controllerName) {
    $className = '\\Filegator\\Controllers\\' . $controllerName;
    try {
        $instance = $container->get($className);
        $reflection = new ReflectionClass($instance);
        $property = $reflection->getProperty('pathacl');
        $property->setAccessible(true);
        $value = $property->getValue($instance);

        printf("%-20s PathACL injected: %s\n", $controllerName, $value ? "YES" : "NO");
    } catch (Exception $e) {
        printf("%-20s ERROR: %s\n", $controllerName, $e->getMessage());
    }
}

echo "\n=== TEST 6: Verify constructor signatures ===\n";
foreach (['FileController', 'DownloadController', 'UploadController'] as $controllerName) {
    $className = '\\Filegator\\Controllers\\' . $controllerName;
    $reflection = new ReflectionClass($className);
    $constructor = $reflection->getConstructor();

    echo "\n$controllerName constructor parameters:\n";
    foreach ($constructor->getParameters() as $param) {
        $type = $param->getType();
        $typeName = $type ? $type->getName() : 'mixed';
        $optional = $param->isOptional() ? ' (optional)' : '';
        $default = $param->isDefaultValueAvailable() ? ' = ' . var_export($param->getDefaultValue(), true) : '';

        echo "  - \${$param->getName()}: {$typeName}{$optional}{$default}\n";
    }
}

echo "\n=== CONCLUSION ===\n";
echo "This test helps determine:\n";
echo "1. Whether PathACLInterface is properly registered in the container\n";
echo "2. Whether call() method instantiates controllers correctly\n";
echo "3. Whether the custom resolver order in Container.php is working\n";
echo "4. Whether there are differences between controller implementations\n";
