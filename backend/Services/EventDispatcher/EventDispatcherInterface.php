<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\EventDispatcher;

/**
 * Interface for the EventDispatcher service
 */
interface EventDispatcherInterface
{
    /**
     * Dispatch an event to all registered listeners
     *
     * @param string|Event $event Event name or Event object
     * @param array $data Event data (only used if $event is a string)
     * @return Event The dispatched event
     */
    public function dispatch($event, array $data = []): Event;

    /**
     * Add an event listener
     *
     * @param string $eventName The event name to listen for
     * @param callable $listener The listener callback
     * @param int $priority Higher values = earlier execution (default: 0)
     */
    public function addListener(string $eventName, callable $listener, int $priority = 0): void;

    /**
     * Remove an event listener
     *
     * @param string $eventName The event name
     * @param callable $listener The listener to remove
     */
    public function removeListener(string $eventName, callable $listener): void;

    /**
     * Add an event subscriber
     *
     * @param EventSubscriberInterface $subscriber The subscriber to add
     */
    public function addSubscriber(EventSubscriberInterface $subscriber): void;

    /**
     * Remove an event subscriber
     *
     * @param EventSubscriberInterface $subscriber The subscriber to remove
     */
    public function removeSubscriber(EventSubscriberInterface $subscriber): void;

    /**
     * Get all listeners for a specific event
     *
     * @param string $eventName The event name
     * @return array<callable> Array of listeners sorted by priority
     */
    public function getListeners(string $eventName): array;

    /**
     * Check if an event has any listeners
     *
     * @param string $eventName The event name
     */
    public function hasListeners(string $eventName): bool;
}
