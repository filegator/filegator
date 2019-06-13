<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Filegator\Controllers;

use Filegator\Kernel\Request;
use Filegator\Kernel\Response;

class ErrorController
{
    protected $request_type;

    public function __construct(Request $request)
    {
        $this->request_type = $request->getContentType();
    }

    public function notFound(Response $response)
    {
        return $this->request_type == 'json' ? $response->json('Not Found', 404) : $response->html('Not Found', 404);
    }

    public function methodNotAllowed(Response $response)
    {
        return $this->request_type == 'json' ? $response->json('Not Allowed', 401) : $response->html('Not Found', 401);
    }
}
