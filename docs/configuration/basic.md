
## Basic
Edit ```configuration.php``` file to change basic things like logo, title, language and upload restrictions.


NOTE: if you've made a mistake in configuration file, forgot to close a quote, the script will throw an error. Please use provided default ```configuration_sample.php``` to verify this.

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
To change default color scheme and other options, edit ```/frontend/App.vue```. When you're done, recompile with ```npm run build```.

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

