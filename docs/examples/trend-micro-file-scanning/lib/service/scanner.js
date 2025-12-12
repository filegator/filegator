#!/usr/bin/env node
/**
 * Trend Micro File Security Scanner Service
 *
 * Standalone Node.js service that wraps the official Trend Micro gRPC SDK.
 * Communicates via stdin/stdout JSON for portable IPC with PHP.
 *
 * Usage:
 *   echo '{"action":"scan","file":"/path/to/file","apiKey":"...","region":"ap-southeast-2"}' | node scanner.js
 *
 * Input (JSON via stdin):
 *   {
 *     "action": "scan",
 *     "file": "/absolute/path/to/file",
 *     "apiKey": "your-vision-one-api-key",
 *     "region": "ap-southeast-2",
 *     "timeout": 300,
 *     "pml": true,
 *     "tags": ["filegator", "upload"]
 *   }
 *
 * Output (JSON via stdout):
 *   {
 *     "success": true,
 *     "status": "clean|malware|error",
 *     "scanId": "...",
 *     "malwareFound": false,
 *     "threats": [],
 *     "fileSha256": "...",
 *     "message": "..."
 *   }
 *
 * Exit codes:
 *   0 - Success (scan completed, check status for result)
 *   1 - Error (invalid input, connection failure, etc.)
 */

const { AmaasGrpcClient } = require('file-security-sdk');
const fs = require('fs');
const path = require('path');

// Region mapping: short codes to SDK region identifiers
const REGION_MAP = {
    'us': 'us-east-1',
    'us-1': 'us-east-1',
    'us-east-1': 'us-east-1',
    'eu': 'eu-central-1',
    'eu-1': 'eu-central-1',
    'eu-central-1': 'eu-central-1',
    'jp': 'ap-northeast-1',
    'jp-1': 'ap-northeast-1',
    'ap-northeast-1': 'ap-northeast-1',
    'sg': 'ap-southeast-1',
    'sg-1': 'ap-southeast-1',
    'ap-southeast-1': 'ap-southeast-1',
    'au': 'ap-southeast-2',
    'au-1': 'ap-southeast-2',
    'ap-southeast-2': 'ap-southeast-2',
    'in': 'ap-south-1',
    'in-1': 'ap-south-1',
    'ap-south-1': 'ap-south-1',
    'me': 'me-central-1',
    'me-1': 'me-central-1',
    'me-central-1': 'me-central-1',
};

/**
 * Output result as JSON to stdout
 */
function output(result) {
    console.log(JSON.stringify(result));
}

/**
 * Output error and exit
 */
function error(message, details = {}) {
    output({
        success: false,
        status: 'error',
        message: message,
        ...details
    });
    process.exit(1);
}

/**
 * Scan a file using the Trend Micro SDK
 */
async function scanFile(options) {
    const { file, apiKey, region, timeout = 300, pml = true, tags = [] } = options;

    // Validate inputs
    if (!file) {
        error('Missing required parameter: file');
    }
    if (!apiKey) {
        error('Missing required parameter: apiKey');
    }
    if (!region) {
        error('Missing required parameter: region');
    }

    // Check file exists
    if (!fs.existsSync(file)) {
        error(`File not found: ${file}`);
    }

    // Map region to SDK format
    const sdkRegion = REGION_MAP[region.toLowerCase()] || region;

    let client = null;

    try {
        // Initialize the gRPC client
        client = new AmaasGrpcClient(sdkRegion, apiKey, timeout);

        // Scan the file
        const scanOptions = {
            pml: pml,
            tags: tags.length > 0 ? tags : ['filegator'],
        };

        const result = await client.scanFile(file, scanOptions);

        // Parse the result
        const scanResult = typeof result === 'string' ? JSON.parse(result) : result;

        // Determine scan status
        const malwareFound = scanResult.scanResult > 0 ||
                            (scanResult.foundMalwares && scanResult.foundMalwares.length > 0);

        output({
            success: true,
            status: malwareFound ? 'malware' : 'clean',
            scanId: scanResult.scanId || null,
            malwareFound: malwareFound,
            threats: scanResult.foundMalwares || [],
            fileSha256: scanResult.fileSHA256 || null,
            scanTimestamp: scanResult.scanTimestamp || new Date().toISOString(),
            message: malwareFound
                ? `Malware detected: ${(scanResult.foundMalwares || []).map(m => m.malwareName).join(', ')}`
                : 'File is clean'
        });

    } catch (err) {
        // Handle specific error types
        let errorMessage = err.message || 'Unknown error';
        let errorCode = 'UNKNOWN_ERROR';

        if (errorMessage.includes('authentication') || errorMessage.includes('UNAUTHENTICATED')) {
            errorCode = 'AUTH_ERROR';
            errorMessage = 'Authentication failed - check API key and region';
        } else if (errorMessage.includes('timeout') || errorMessage.includes('DEADLINE_EXCEEDED')) {
            errorCode = 'TIMEOUT';
            errorMessage = 'Scan timed out';
        } else if (errorMessage.includes('connect') || errorMessage.includes('UNAVAILABLE')) {
            errorCode = 'CONNECTION_ERROR';
            errorMessage = 'Failed to connect to Trend Micro service';
        }

        error(errorMessage, { errorCode: errorCode });

    } finally {
        // Clean up client
        if (client) {
            try {
                client.close();
            } catch (e) {
                // Ignore close errors
            }
        }
    }
}

/**
 * Handle test mode
 */
function runTest() {
    output({
        success: true,
        status: 'test',
        message: 'Scanner service is working',
        version: '1.0.0',
        nodeVersion: process.version,
        regions: Object.keys(REGION_MAP).filter(k => !k.includes('-'))
    });
    process.exit(0);
}

/**
 * Main entry point - read JSON from stdin
 */
async function main() {
    // Handle --test flag
    if (process.argv.includes('--test')) {
        runTest();
        return;
    }

    // Read input from stdin
    let inputData = '';

    process.stdin.setEncoding('utf8');

    process.stdin.on('data', (chunk) => {
        inputData += chunk;
    });

    process.stdin.on('end', async () => {
        // Parse input
        let request;
        try {
            request = JSON.parse(inputData.trim());
        } catch (e) {
            error('Invalid JSON input', { received: inputData.substring(0, 100) });
            return;
        }

        // Handle actions
        const action = request.action || 'scan';

        switch (action) {
            case 'scan':
                await scanFile(request);
                break;
            case 'test':
                runTest();
                break;
            default:
                error(`Unknown action: ${action}`);
        }
    });

    // Handle stdin not being piped (interactive mode)
    if (process.stdin.isTTY) {
        error('No input provided. Pipe JSON to stdin or use --test flag.');
    }
}

// Run
main().catch(err => {
    error('Unexpected error: ' + err.message);
});
