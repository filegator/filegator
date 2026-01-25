<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\EventDispatcher;

use Filegator\Services\Service;

/**
 * EventDispatcher - Central hub for event management in FileGator
 *
 * This service allows plugins to hook into various FileGator operations
 * like file uploads, downloads, deletions, and authentication events.
 */
class EventDispatcher implements EventDispatcherInterface, Service
{
    /**
     * @var array<string, array<array{listener: callable, priority: int}>>
     */
    private $listeners = [];

    /**
     * @var array<string, array<callable>> Sorted listeners cache
     */
    private $sorted = [];

    /**
     * @var array<EventSubscriberInterface> Registered subscribers
     */
    private $subscribers = [];

    /**
     * Initialize the EventDispatcher service
     */
    public function init(array $config = [])
    {
        // Register any subscribers defined in configuration
        if (isset($config['subscribers']) && is_array($config['subscribers'])) {
            foreach ($config['subscribers'] as $subscriberClass) {
                if (class_exists($subscriberClass)) {
                    $subscriber = new $subscriberClass();
                    if ($subscriber instanceof EventSubscriberInterface) {
                        $this->addSubscriber($subscriber);
                    }
                }
            }
        }
    }

    /**
     * {@inheritdoc}
     */
    public function dispatch($event, array $data = []): Event
    {
        if (is_string($event)) {
            $eventName = $event;
            $event = new Event($eventName, $data);
        } else {
            $eventName = $event->getName();
        }

        if (!$eventName) {
            return $event;
        }

        foreach ($this->getListeners($eventName) as $listener) {
            if ($event->isPropagationStopped()) {
                break;
            }
            $listener($event);
        }

        return $event;
    }

    /**
     * {@inheritdoc}
     */
    public function addListener(string $eventName, callable $listener, int $priority = 0): void
    {
        $this->listeners[$eventName][] = [
            'listener' => $listener,
            'priority' => $priority,
        ];
        unset($this->sorted[$eventName]);
    }

    /**
     * {@inheritdoc}
     */
    public function removeListener(string $eventName, callable $listener): void
    {
        if (!isset($this->listeners[$eventName])) {
            return;
        }

        foreach ($this->listeners[$eventName] as $key => $registered) {
            if ($registered['listener'] === $listener) {
                unset($this->listeners[$eventName][$key]);
                unset($this->sorted[$eventName]);
                break;
            }
        }
    }

    /**
     * {@inheritdoc}
     */
    public function addSubscriber(EventSubscriberInterface $subscriber): void
    {
        $this->subscribers[] = $subscriber;

        foreach ($subscriber::getSubscribedEvents() as $eventName => $params) {
            if (is_string($params)) {
                $this->addListener($eventName, [$subscriber, $params]);
            } elseif (is_array($params)) {
                // Check if it's a single listener with priority
                if (isset($params[0]) && is_string($params[0])) {
                    $this->addListener(
                        $eventName,
                        [$subscriber, $params[0]],
                        $params[1] ?? 0
                    );
                } else {
                    // Multiple listeners for same event
                    foreach ($params as $listener) {
                        $this->addListener(
                            $eventName,
                            [$subscriber, $listener[0]],
                            $listener[1] ?? 0
                        );
                    }
                }
            }
        }
    }

    /**
     * {@inheritdoc}
     */
    public function removeSubscriber(EventSubscriberInterface $subscriber): void
    {
        foreach ($subscriber::getSubscribedEvents() as $eventName => $params) {
            if (is_string($params)) {
                $this->removeListener($eventName, [$subscriber, $params]);
            } elseif (is_array($params)) {
                if (isset($params[0]) && is_string($params[0])) {
                    $this->removeListener($eventName, [$subscriber, $params[0]]);
                } else {
                    foreach ($params as $listener) {
                        $this->removeListener($eventName, [$subscriber, $listener[0]]);
                    }
                }
            }
        }

        $this->subscribers = array_filter(
            $this->subscribers,
            fn($s) => $s !== $subscriber
        );
    }

    /**
     * {@inheritdoc}
     */
    public function getListeners(string $eventName): array
    {
        if (!isset($this->listeners[$eventName])) {
            return [];
        }

        if (!isset($this->sorted[$eventName])) {
            $this->sortListeners($eventName);
        }

        return $this->sorted[$eventName];
    }

    /**
     * {@inheritdoc}
     */
    public function hasListeners(string $eventName): bool
    {
        return !empty($this->listeners[$eventName]);
    }

    /**
     * Get all registered subscribers
     *
     * @return array<EventSubscriberInterface>
     */
    public function getSubscribers(): array
    {
        return $this->subscribers;
    }

    /**
     * Sort listeners by priority (higher priority = earlier execution)
     */
    private function sortListeners(string $eventName): void
    {
        $listeners = $this->listeners[$eventName];

        usort($listeners, function ($a, $b) {
            return $b['priority'] <=> $a['priority'];
        });

        $this->sorted[$eventName] = array_column($listeners, 'listener');
    }
}
