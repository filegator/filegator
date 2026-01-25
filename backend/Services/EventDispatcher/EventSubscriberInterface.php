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
 * Interface for event subscribers (plugins)
 *
 * Plugins can implement this interface to subscribe to multiple events
 * at once. The getSubscribedEvents method returns an array mapping
 * event names to method names or arrays of [method, priority].
 */
interface EventSubscriberInterface
{
    /**
     * Returns an array of events this subscriber wants to listen to.
     *
     * The array keys are event names and the value can be:
     *
     *  - The method name to call (priority defaults to 0)
     *  - An array composed of the method name and priority
     *  - An array of arrays composed of method names and priorities
     *
     * Example:
     * [
     *     'file.upload.before' => 'onBeforeUpload',
     *     'file.upload.after' => ['onAfterUpload', 10],
     *     'file.delete' => [
     *         ['onDeleteFirst', 10],
     *         ['onDeleteLast', -10],
     *     ],
     * ]
     *
     * @return array<string, string|array>
     */
    public static function getSubscribedEvents(): array;
}
