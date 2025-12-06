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
use Filegator\Services\Auth\AuthInterface;
use Filegator\Services\Auth\User;
use Filegator\Services\Storage\Filesystem;
use Rakit\Validation\Validator;
use Filegator\Config\Config;

class AdminController
{
    protected $auth;

    protected $storage;

    public function __construct(AuthInterface $auth, Filesystem $storage)
    {
        $this->auth = $auth;
        $this->storage = $storage;
    }

    public function listUsers(Request $request, Response $response)
    {
        return $response->json($this->auth->allUsers());
    }

    public function storeUser(User $user, Request $request, Response $response, Validator $validator)
    {
        $validator->setMessage('required', 'This field is required');
        $validation = $validator->validate($request->all(), [
            'name' => 'required',
            'username' => 'required',
            'homedir' => 'required',
            'password' => 'required',
        ]);

        if ($validation->fails()) {
            $errors = $validation->errors();

            return $response->json($errors->firstOfAll(), 422);
        }

        if ($this->auth->find($request->input('username'))) {
            return $response->json(['username' => 'Username already taken'], 422);
        }

        try {
            $user->setName($request->input('name'));
            $user->setUsername($request->input('username'));
            $user->setHomedir(
                rtrim($this->auth->user()->getHomeDir(), $this->storage->getSeparator())
                .$this->storage->getSeparator()
                .ltrim($request->input('homedir'), $this->storage->getSeparator())
            );
            $user->setRole($request->input('role', 'user'));
            $user->setPermissions($request->input('permissions'));
            $ret = $this->auth->add($user, $request->input('password'));
        } catch (\Exception $e) {
            return $response->json($e->getMessage(), 422);
        }

        return $response->json($ret);
    }

    public function updateUser($username, Request $request, Response $response, Validator $validator)
    {
        $user = $this->auth->find($username);

        if (! $user) {
            return $response->json('User not found', 422);
        }

        $validator->setMessage('required', 'This field is required');
        $validation = $validator->validate($request->all(), [
            'name' => 'required',
            'username' => 'required',
            'homedir' => 'required',
        ]);

        if ($validation->fails()) {
            $errors = $validation->errors();

            return $response->json($errors->firstOfAll(), 422);
        }

        if ($username != $request->input('username') && $this->auth->find($request->input('username'))) {
            return $response->json(['username' => 'Username already taken'], 422);
        }

        try {
            $user->setName($request->input('name'));
            $user->setUsername($request->input('username'));
            $user->setHomedir($request->input('homedir'));
            $user->setRole($request->input('role', 'user'));
            $user->setPermissions($request->input('permissions'));

            return $response->json($this->auth->update($username, $user, $request->input('password', '')));
        } catch (\Exception $e) {
            return $response->json($e->getMessage(), 422);
        }
    }

    public function deleteUser($username, Request $request, Response $response)
    {
        $user = $this->auth->find($username);

        if (! $user) {
            return $response->json('User not found', 422);
        }

        return $response->json($this->auth->delete($user));
    }

    public function getFrontendSettings(Response $response, Config $config)
    {
        $frontend = $config->get('frontend_config');
        $root = [
            'overwrite_on_upload' => $config->get('overwrite_on_upload'),
            'timezone' => $config->get('timezone'),
            'download_inline' => $config->get('download_inline'),
            'lockout_attempts' => $config->get('lockout_attempts'),
            'lockout_timeout' => $config->get('lockout_timeout'),
        ];

        $publicDir = $config->get('public_dir');
        $cssFile = $publicDir.'/css/custom.css';
        $jsFile = $publicDir.'/js/custom.js';
        $css = '';
        $js = '';
        if (is_file($cssFile) && is_readable($cssFile)) {
            $c = @file_get_contents($cssFile);
            if ($c !== false) { $css = $c; }
        }
        if (is_file($jsFile) && is_readable($jsFile)) {
            $j = @file_get_contents($jsFile);
            if ($j !== false) { $js = $j; }
        }
        $frontend['custom_css'] = $css;
        $frontend['custom_js'] = $js;

        return $response->json([
            'frontend_config' => $frontend,
            'root_config' => $root,
        ]);
    }

    private function replaceConfigValueSegment(string $text, string $key, string $literal): string
    {
        $pattern = "/('".preg_quote($key, '/')."'\s*=>\s*)/";
        if (preg_match($pattern, $text, $m, PREG_OFFSET_CAPTURE)) {
            $start = $m[0][1] + strlen($m[0][0]);
            $len = strlen($text);
            $i = $start;
            $depthParen = 0;
            $depthBracket = 0;
            $inSingle = false;
            $inDouble = false;
            $prev = '';
            while ($i < $len) {
                $ch = $text[$i];
                if (!$inSingle && !$inDouble) {
                    if ($ch === '(') { $depthParen++; }
                    elseif ($ch === ')') { if ($depthParen > 0) { $depthParen--; } }
                    elseif ($ch === '[') { $depthBracket++; }
                    elseif ($ch === ']') { if ($depthBracket > 0) { $depthBracket--; } }
                    elseif ($ch === ',' && $depthParen === 0 && $depthBracket === 0) {
                        $before = substr($text, 0, $start);
                        $after = substr($text, $i);
                        return $before.$literal.$after;
                    }
                }
                if ($ch === "'" && !$inDouble) {
                    $inSingle = ($prev === '\\') ? $inSingle : !$inSingle;
                } elseif ($ch === '"' && !$inSingle) {
                    $inDouble = ($prev === '\\') ? $inDouble : !$inDouble;
                }
                $prev = $ch;
                $i++;
            }
        }
        return $text;
    }

    public function updateFrontendSettings(Request $request, Response $response)
    {
        $path = dirname(__DIR__, 2).'/configuration.php';

        if (! is_file($path) || ! is_readable($path)) {
            return $response->json('Configuration file not found', 422);
        }

        $incoming = $request->input('frontend_config', []);
        $incomingRoot = $request->input('root_config', []);

        if (is_object($incoming)) { $incoming = (array) $incoming; }
        if (is_object($incomingRoot)) { $incomingRoot = (array) $incomingRoot; }

        if (! is_array($incoming) || ! is_array($incomingRoot)) {
            return $response->json('Invalid payload', 422);
        }

        $contents = file_get_contents($path);
        if ($contents === false) {
            return $response->json('Failed to read configuration', 422);
        }

        $updated = $contents;

        if (isset($incoming['app_name'])) {
            $val = str_replace(['`', "'"], ['', "\\'"], $incoming['app_name']);
            $updated = $this->replaceConfigValueSegment($updated, 'app_name', "'".$val."'");
        }

        if (isset($incoming['language'])) {
            $val = str_replace(['`', "'"], ['', "\\'"], $incoming['language']);
            $updated = $this->replaceConfigValueSegment($updated, 'language', "'".$val."'");
        }

        if (isset($incoming['logo'])) {
            $val = str_replace(['`', "'"], ['', "\\'"], $incoming['logo']);
            $updated = $this->replaceConfigValueSegment($updated, 'logo', "'".$val."'");
        }

        foreach ([
            'upload_max_size', 'upload_chunk_size', 'upload_simultaneous',
            'search_simultaneous'
        ] as $intKey) {
            if (isset($incoming[$intKey])) {
                $val = (int) $incoming[$intKey];
                $updated = $this->replaceConfigValueSegment($updated, $intKey, (string)$val);
            }
        }

        // write custom CSS/JS to public files
        $publicDir = dirname(__DIR__, 2).'/dist';
        if (isset($incoming['custom_css'])) {
            @file_put_contents($publicDir.'/css/custom.css', (string)$incoming['custom_css']);
        }
        if (isset($incoming['custom_js'])) {
            @file_put_contents($publicDir.'/js/custom.js', (string)$incoming['custom_js']);
        }

        foreach ([
            'default_archive_name', 'date_format', 'guest_redirection'
        ] as $strKey) {
            if (isset($incoming[$strKey])) {
                $val = str_replace(['`', "'"], ['', "\\'"], $incoming[$strKey]);
                $updated = $this->replaceConfigValueSegment($updated, $strKey, "'".$val."'");
            }
        }

        foreach ([
            'search_direct_download'
        ] as $boolKey) {
            if (isset($incoming[$boolKey])) {
                $val = $incoming[$boolKey] ? 'true' : 'false';
                $updated = $this->replaceConfigValueSegment($updated, $boolKey, $val);
            }
        }

        // array-type keys: editable, filter_entries, pagination
        foreach ([
            'editable', 'filter_entries', 'pagination'
        ] as $arrKey) {
            if (isset($incoming[$arrKey])) {
                $arrVal = $incoming[$arrKey];
                if (is_string($arrVal)) {
                    $items = array_map('trim', explode(',', $arrVal));
                } elseif (is_array($arrVal)) {
                    $items = $arrVal;
                } else {
                    $items = [];
                }
                $phpArray = '[';
                $first = true;
                foreach ($items as $it) {
                    if (! $first) { $phpArray .= ', '; }
                    $first = false;
                    if (is_numeric($it)) {
                        $phpArray .= (int) $it;
                    } else {
                        $phpArray .= "'".str_replace("'", "\\'", $it)."'";
                    }
                }
                $phpArray .= ']';

                $updated = $this->replaceConfigValueSegment($updated, $arrKey, $phpArray);
            }
        }

        // root_config replacements
        if (isset($incomingRoot['overwrite_on_upload'])) {
            $val = $incomingRoot['overwrite_on_upload'] ? 'true' : 'false';
            $updated = $this->replaceConfigValueSegment($updated, 'overwrite_on_upload', $val);
        }

        if (isset($incomingRoot['timezone'])) {
            $val = str_replace(['`', "'"], ['', "\\'"], $incomingRoot['timezone']);
            $updated = $this->replaceConfigValueSegment($updated, 'timezone', "'".$val."'");
        }

        // if (isset($incomingRoot['lockout_attempts'])) {
        //     $updated = preg_replace(
        //         "/('lockout_attempts'\s*=>\s*)\d+/",
        //         "$1".$incomingRoot['lockout_attempts'],
        //         $updated,
        //         1
        //     );
        // }

        // if (isset($incomingRoot['lockout_timeout'])) {
        //     $updated = preg_replace(
        //         "/('lockout_timeout'\s*=>\s*)\d+/",
        //         "$1".$incomingRoot['lockout_timeout'],
        //         $updated,
        //         1
        //     );
        // }

        if (isset($incomingRoot['download_inline'])) {
            $arrVal = $incomingRoot['download_inline'];
            if (is_string($arrVal)) {
                $items = array_map('trim', explode(',', $arrVal));
            } elseif (is_array($arrVal)) {
                $items = $arrVal;
            } else {
                $items = [];
            }
            $phpArray = '[';
            $first = true;
            foreach ($items as $it) {
                if (! $first) { $phpArray .= ', '; }
                $first = false;
                $phpArray .= "'".str_replace("'", "\\'", $it)."'";
            }
            $phpArray .= ']';
            $updated = $this->replaceConfigValueSegment($updated, 'download_inline', $phpArray);
        }

        if ($updated === null) {
            return $response->json('Failed to update configuration', 422);
        }

                if (@file_put_contents($path, $updated) === false) {
            return $response->json('Failed to write configuration', 422);
        }

        $config = include $path;
        $publicDir = dirname(__DIR__, 2).'/dist';
        $css = '';
        $js = '';
        if (is_file($publicDir.'/css/custom.css') && is_readable($publicDir.'/css/custom.css')) {
            $c = @file_get_contents($publicDir.'/css/custom.css');
            if ($c !== false) { $css = $c; }
        }
        if (is_file($publicDir.'/js/custom.js') && is_readable($publicDir.'/js/custom.js')) {
            $j = @file_get_contents($publicDir.'/js/custom.js');
            if ($j !== false) { $js = $j; }
        }
        $config['frontend_config']['custom_css'] = $css;
        $config['frontend_config']['custom_js'] = $js;
        return $response->json($config['frontend_config']);
    }
}
