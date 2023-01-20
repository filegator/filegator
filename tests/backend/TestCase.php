<?php

/*
 * This file is part of the FileGator package.
 *
 * (c) Milos Stojanovic <alcalbg@gmail.com>
 *
 * For the full copyright and license information, please view the LICENSE file
 */

namespace Tests;

use Filegator\App;
use Filegator\Config\Config;
use Filegator\Container\Container;
use Filegator\Kernel\Request;
use Filegator\Kernel\Response;
use Filegator\Kernel\StreamedResponse;
use Filegator\Services\Session\Session;
use PHPUnit\Framework\TestCase as BaseTestCase;
use Symfony\Component\HttpFoundation\Session\Storage\MockFileSessionStorage;

define('APP_ENV', 'test');

define('TEST_DIR', __DIR__.'/tmp');
define('TEST_REPOSITORY', TEST_DIR.'/repository');
define('TEST_ARCHIVE', TEST_DIR.'/testarchive.zip');
define('TEST_FILE', TEST_DIR.'/sample.txt');
define('TEST_TMP_PATH', TEST_DIR.'/temp/');

/**
 * @internal
 * @coversNothing
 */
class TestCase extends BaseTestCase
{
    use TestResponse;

    public $response;

    public $streamedResponse;

    public $previous_session = false;

    protected $auth = false;

    public function bootFreshApp($config = null, $request = null, $response = null, $mock_users = false)
    {
        $config = $config ?: $this->getMockConfig();
        $request = $request ?: new Request();

        return new App($config, $request, new FakeResponse(), new FakeStreamedResponse(), new Container());
    }

    public function sendRequest($method, $uri, $data = null, $files = [], $server = [])
    {
        $fakeRequest = Request::create(
            '?r='.$uri,
            $method,
            [],
            [],
            $files,
            array_replace([
                'CONTENT_TYPE' => 'application/json',
                'HTTP_ACCEPT' => 'application/json',
            ], $server),
            json_encode($data)
        );

        if ($this->previous_session) {
            $fakeRequest->setSession($this->previous_session);
        } else {
            $sessionStorage = new MockFileSessionStorage();
            $fakeRequest->setSession(new Session($sessionStorage));
        }

        $app = $this->bootFreshApp(null, $fakeRequest, null, true);

        $this->response = $app->resolve(Response::class);
        $this->streamedResponse = $app->resolve(StreamedResponse::class);

        return $app;
    }

    public function signIn($username, $password)
    {
        $this->signOut();

        $app = $this->sendRequest('POST', '/login', [
            'username' => $username,
            'password' => $password,
        ]);

        $request = $app->resolve(Request::class);
        $this->previous_session = $request->getSession();
    }

    public function signOut()
    {
        $this->previous_session = false;
    }

    public function getMockConfig()
    {
        $config = require __DIR__.'/configuration.php';

        return new Config($config);
    }

    public function delTree($dir)
    {
        if (! is_dir($dir)) {
            return;
        }
        $files = array_diff(scandir($dir), ['.', '..']);
        foreach ($files as $file) {
            (is_dir("{$dir}/{$file}")) ? $this->delTree("{$dir}/{$file}") : unlink("{$dir}/{$file}");
        }

        return rmdir($dir);
    }

    public function resetTempDir()
    {
        $this->delTree(TEST_TMP_PATH);
        $this->delTree(TEST_REPOSITORY);

        mkdir(TEST_TMP_PATH);
        mkdir(TEST_REPOSITORY);
    }

    public function invokeMethod(&$object, $methodName, array $parameters = [])
    {
        $reflection = new \ReflectionClass(get_class($object));
        $method = $reflection->getMethod($methodName);
        $method->setAccessible(true);

        return $method->invokeArgs($object, $parameters);
    }
}
