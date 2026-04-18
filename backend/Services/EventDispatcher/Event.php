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
 * Base event class for all FileGator events
 */
class Event
{
    /**
     * @var bool Whether event propagation has been stopped
     */
    private $propagationStopped = false;

    /**
     * @var string The event name
     */
    private $name;

    /**
     * @var array Event data/payload
     */
    protected $data = [];

    /**
     * @var mixed The result that can be modified by listeners
     */
    protected $result = null;

    public function __construct(string $name = '', array $data = [])
    {
        $this->name = $name;
        $this->data = $data;
    }

    /**
     * Get the event name
     */
    public function getName(): string
    {
        return $this->name;
    }

    /**
     * Set the event name
     */
    public function setName(string $name): self
    {
        $this->name = $name;
        return $this;
    }

    /**
     * Get all event data
     */
    public function getData(): array
    {
        return $this->data;
    }

    /**
     * Get a specific data value
     *
     * @param mixed $default
     * @return mixed
     */
    public function get(string $key, $default = null)
    {
        return $this->data[$key] ?? $default;
    }

    /**
     * Set a data value
     *
     * @param mixed $value
     */
    public function set(string $key, $value): self
    {
        $this->data[$key] = $value;
        return $this;
    }

    /**
     * Get the result
     *
     * @return mixed
     */
    public function getResult()
    {
        return $this->result;
    }

    /**
     * Set the result
     *
     * @param mixed $result
     */
    public function setResult($result): self
    {
        $this->result = $result;
        return $this;
    }

    /**
     * Stop event propagation to subsequent listeners
     */
    public function stopPropagation(): void
    {
        $this->propagationStopped = true;
    }

    /**
     * Check if propagation was stopped
     */
    public function isPropagationStopped(): bool
    {
        return $this->propagationStopped;
    }
}
