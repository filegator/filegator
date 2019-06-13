<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Services\View\Adapters;

use Filegator\Config\Config;
use Filegator\Services\Service;
use Filegator\Services\View\ViewInterface;

class Vuejs implements Service, ViewInterface
{
    private $config;

    public function __construct(Config $config)
    {
        $this->config = $config;
    }

    public function init(array $config = [])
    {
    }

    public function getIndexPage()
    {
        $title = $this->config->get('frontend_config.app_name');
        $public_path = $this->config->get('public_path');
        $public_dir = $this->config->get('public_dir');

        return '<!DOCTYPE html>
<html lang=en>
  <head>
    <meta charset=utf-8>
    <meta http-equiv=X-UA-Compatible content="IE=edge">
    <meta name=viewport content="width=device-width,initial-scale=1">
    <meta name="robots" content="noindex,nofollow">
    <title>'.$title.'</title>
    <link rel=stylesheet href=https://use.fontawesome.com/releases/v5.2.0/css/all.css>
    <link rel=stylesheet href=//cdn.materialdesignicons.com/2.5.94/css/materialdesignicons.min.css>
    <link href="'.$public_path.'css/app.css?'.@filemtime($public_dir.'/css/app.css').'" rel=stylesheet>
  </head>
  <body>
    <noscript><strong>Please enable JavaScript to continue.</strong></noscript>
    <div id=app></div>
    <script src="'.$public_path.'js/app.js?'.@filemtime($public_dir.'/js/app.js').'"></script>
    <script src="'.$public_path.'js/chunk-vendors.js?'.@filemtime($public_dir.'/js/chunk-vendors.js').'"></script>
  </body>
</html>
';
    }
}
