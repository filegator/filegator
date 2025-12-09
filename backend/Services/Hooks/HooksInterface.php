<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\Hooks;

/**
 * Interface for the Hooks service
 *
 * Hooks allow plugins/scripts to react to FileGator events.
 * When an event occurs (e.g., file upload), all matching hook scripts
 * in the hooks directory are executed with event data passed as arguments.
 */
interface HooksInterface
{
    /**
     * Trigger a hook event, executing all registered hook scripts
     *
     * @param string $hookName The hook name (e.g., 'onUpload', 'onDelete')
     * @param array $data Data to pass to hook scripts
     * @return array Results from all executed hooks
     */
    public function trigger(string $hookName, array $data = []): array;

    /**
     * Register a callback hook (in-memory, for plugins)
     *
     * @param string $hookName The hook name
     * @param callable $callback The callback to execute
     * @param int $priority Higher priority = earlier execution
     */
    public function register(string $hookName, callable $callback, int $priority = 0): void;

    /**
     * Get all registered hooks for an event
     *
     * @param string $hookName The hook name
     * @return array List of hook scripts/callbacks
     */
    public function getHooks(string $hookName): array;

    /**
     * Check if any hooks are registered for an event
     *
     * @param string $hookName The hook name
     */
    public function hasHooks(string $hookName): bool;
}
