<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests\Unit;

use Filegator\Config\Config;
use Filegator\Services\View\Adapters\Vuejs;
use Tests\TestCase;

/**
 * @internal
 */
class ViewTest extends TestCase
{
    public function testViewService()
    {
        $config_mock = new Config(['frontend_config' => ['app_name' => 'testapp']]);

        $service = new Vuejs($config_mock);
        $service->init();

        $this->assertRegexp('/testapp/', $service->getIndexPage());
    }
}
