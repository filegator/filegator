---
currentMenu: tmpfs
---

## Temporary file system service
This service is responsible for managing temporary files. TMP files are created:

- When uploading files, chunks are stored in the TMP folder before merging and moving to the final storage destination
- When creating and extracting archives (zip files)
- When downloading multiple files, they are copied into TMP folder before zipping
- To count and lock excessive login attempts


Tmp files are usually removed immediately after the use. For expired files, configurable garbage collection is used:

```
        'Filegator\Services\Tmpfs\TmpfsInterface' => [
            'handler' => '\Filegator\Services\Tmpfs\Adapters\Tmpfs',
            'config' => [
                'path' => __DIR__.'/private/tmp/',
                'gc_probability_perc' => 10,
                'gc_older_than' => 60 * 60 * 24 * 2, // 2 days
            ],
        ],
```

Note: if you want to use this script as a stateless app or in any kind of multi-node environment, you must mount a single shared TMP folder for all the instances. You can solve this problem with [Amazon Elastic File System](https://aws.amazon.com/efs/) or similar approach.
