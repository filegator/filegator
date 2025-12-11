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
use Invoker\Invoker;
use Invoker\ParameterResolver\AssociativeArrayResolver;
use Invoker\ParameterResolver\Container\TypeHintContainerResolver;
use Invoker\ParameterResolver\DefaultValueResolver;
use Invoker\ParameterResolver\NumericArrayResolver;
use Invoker\ParameterResolver\ResolverChain;

class Container extends PHPDIContainer implements ContainerInterface
{
    private $customInvoker = null;

    /**
     * Override call() to use a custom invoker that resolves container type hints
     * BEFORE default values. This allows optional interface parameters (e.g., PathACLInterface $pathacl = null)
     * to be properly injected from the container when available.
     *
     * PHP-DI's default order: DefaultValueResolver before TypeHintContainerResolver
     * Our order: TypeHintContainerResolver before DefaultValueResolver
     */
    public function call($callable, array $parameters = [])
    {
        if ($this->customInvoker === null) {
            // Create resolver chain with TypeHintContainerResolver BEFORE DefaultValueResolver
            // This ensures registered interfaces are injected even for optional parameters
            $parameterResolver = new ResolverChain([
                new NumericArrayResolver,
                new AssociativeArrayResolver,
                new TypeHintContainerResolver($this),  // Check container BEFORE using defaults
                new DefaultValueResolver,              // Fall back to defaults if not in container
            ]);

            $this->customInvoker = new Invoker($parameterResolver, $this);
        }

        return $this->customInvoker->call($callable, $parameters);
    }
}
