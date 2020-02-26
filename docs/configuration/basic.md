---
currentMenu: basic
---

## Basic
All services are set with reasonable defaults. For regular users there is no need to change anything. The script should work out of the box.


You can edit `configuration.php` file to change the basic things like logo image, title, language and upload restrictions.


Note: if you've made a mistake in configuration file (forgot to close a quote?) the script will fail to load or throw an error. Please use provided default `configuration_sample.php` to put everything back to normal.

```
    'frontend_config' => [
        'app_name' => 'FileGator',
        'app_version' => APP_VERSION,
        'language' => 'english',
        'logo' => 'https://raw.githubusercontent.com/filegator/filegator/master/dist/img/logo.png',
        'upload_max_size' => 100 * 1024 * 1024, // 100MB
        'upload_chunk_size' => 1 * 1024 * 1024, // 1MB
        'upload_simultaneous' => 3,
        'default_archive_name' => 'archive.zip',
        'editable' => ['.txt', '.css', '.js', '.ts', '.html', '.php'],
        'date_format' => 'YY/MM/DD hh:mm:ss', // see: https://momentjs.com/docs/#/displaying/format/
    ],
```

## Additional HTML
You can add additional html to the head and body like this:
```
        'Filegator\Services\View\ViewInterface' => [
            'handler' => '\Filegator\Services\View\Adapters\Vuejs',
            'config' => [
                'add_to_head' => '<meta name="author" content="something">',
                'add_to_body' => '<script src="http://example.com/analytics.js"></script>',
            ],
        ],
```

## Frontend tweaks
To change default color scheme and other options, edit `frontend/App.vue` When you're done, recompile with `npm run build`

```
// Primary color
$primary: #34B891;
$primary-invert: findColorInvert($primary);

$colors: (
    "primary": ($primary, $primary-invert),
    "info": ($info, $info-invert),
    "success": ($success, $success-invert),
    "warning": ($warning, $warning-invert),
    "danger": ($danger, $danger-invert),
);

// Links
$link: $primary;
$link-invert: $primary-invert;
$link-focus-border: $primary;

// Disable the widescreen breakpoint
$widescreen-enabled: false;

// Disable the fullhd breakpoint
$fullhd-enabled: false;
```

