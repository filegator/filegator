# Trend Micro Vision One File Security API Research

## Executive Summary

Trend Micro Vision One File Security provides malware scanning capabilities through both gRPC-based SDKs and potentially REST APIs. The service uses API key authentication and returns structured JSON responses indicating whether files are clean or contain malware.

**Key Finding:** The official SDKs use **gRPC protocol** (not traditional REST HTTP), which differs from the PHP curl code snippet provided. However, REST API endpoints may also be available for direct HTTP-based integration.

---

## 1. API Endpoint URL Patterns

### gRPC Endpoint Format (SDK-based)
```
antimalware.<REGION>.cloudone.trendmicro.com:443
```

**Supported Regions:**
- `us-east-1` (US East)
- `eu-central-1` (Europe Central)
- `eu-west-2` (Europe West)
- `ap-southeast-1` (Asia Pacific Southeast 1)
- `ap-southeast-2` (Asia Pacific Southeast 2)
- `ap-northeast-1` (Asia Pacific Northeast)
- `ap-south-1` (Asia Pacific South)
- `me-central-1` (Middle East Central)

**Example:**
```
antimalware.us-east-1.cloudone.trendmicro.com:443
```

### REST API Endpoint (For Direct HTTP Requests)
Based on the PHP code snippet provided and common Trend Micro patterns, the REST endpoint likely follows this format:
```
https://antimalware.<REGION>.cloudone.trendmicro.com/api/v1/scan
```

**Note:** The official documentation primarily focuses on gRPC SDKs. Direct REST API documentation may be available through Trend Micro support channels.

---

## 2. Authentication

### API Key Generation
1. Log into Trend Vision One console
2. Navigate to **Administration > API Keys**
3. Click "Add API Key"
4. Assign a role with **"Run file scan via SDK"** permission
5. Generate and securely store the API key

**Important:** API keys expire after 1 year by default, but administrators can revoke/regenerate them at any time.

### Authentication Header Format

#### For REST API (HTTP/HTTPS)
```http
Authorization: ApiKey YOUR_API_KEY_HERE
Content-Type: application/octet-stream
```

**PHP Example:**
```php
$headers = [
    'Authorization: ApiKey ' . $apiKey,
    'Content-Type: application/octet-stream',
];
```

#### For SDK-based Integration (gRPC)
API key is passed during client initialization, not in HTTP headers:

**Python:**
```python
handle = amaas.grpc.init_by_region(region="us-east-1", api_key="YOUR_API_KEY")
```

**Node.js:**
```javascript
const scanClient = new AmaasGrpcClient("us-east-1", "YOUR_API_KEY");
```

**Java:**
```java
AMaasClient client = new AMaasClient("us-east-1", "YOUR_API_KEY");
```

---

## 3. File Submission (POST Request Format)

### REST API Approach (Direct HTTP)
Based on the PHP snippet and standard REST practices:

```php
$ch = curl_init($apiEndpoint);
curl_setopt($ch, CURLOPT_POST, true);
curl_setopt($ch, CURLOPT_POSTFIELDS, $fileContent);
curl_setopt($ch, CURLOPT_HTTPHEADER, [
    'Authorization: ApiKey ' . $apiKey,
    'Content-Type: application/octet-stream',
]);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
$response = curl_exec($ch);
curl_close($ch);
```

**Request Body:** Raw binary file content (application/octet-stream)

### SDK-based Approach (gRPC)

**Python:**
```python
import visionone_filesecurity as amaas

# Initialize connection
handle = amaas.grpc.init_by_region(region="us-east-1", api_key="YOUR_API_KEY")

# Scan file
result = amaas.grpc.scan_file(
    handle,
    file_name="/path/to/file.exe",
    tags=["upload", "user-generated"],
    pml=True,      # Predictive Machine Learning
    feedback=True, # Smart Protection Network feedback
    verbose=False  # Concise response
)

# Clean up
amaas.grpc.quit(handle)
```

**Node.js/TypeScript:**
```javascript
import { AmaasGrpcClient } from "file-security-sdk";

const scanClient = new AmaasGrpcClient("us-east-1", "YOUR_API_KEY");

const result = await scanClient.scanFile(
  "/path/to/file.ext",
  ["tag1", "tag2"],
  true,  // pml
  true   // feedback
);

await scanClient.close();
```

**Java:**
```java
AMaasClient client = new AMaasClient("us-east-1", "YOUR_API_KEY");

AMaasScanOptions options = AMaasScanOptions.builder()
    .pml(true)
    .feedback(true)
    .verbose(false)
    .tagList(new String[]{"upload", "user"})
    .build();

String scanResult = client.scanFile("/path/to/file.exe", true, options);
client.close();
```

---

## 4. Response Format

### JSON Response Structure

#### Concise Response (Default)
```json
{
  "version": "1.0",
  "scanId": "25072030425f4f4d68953177d0628d0b",
  "scanResult": 1,
  "scanTimestamp": "2022-11-02T00:55:31Z",
  "fileName": "EICAR_TEST_FILE-1.exe",
  "foundMalwares": [
    {
      "fileName": "Eicar.exe",
      "malwareName": "Eicar_test_file"
    }
  ],
  "fileSHA1": "3395856ce81f2b7382dee72602f798b642f14140",
  "fileSHA256": "275a021bbfb6489e54d471899f7db9d1663fc695ec2fe2a2c4538aabf651fd0f"
}
```

#### Verbose Response (Optional)
Includes additional fields:
- `schemaVersion`
- `scannerVersion`
- `fileType`
- `scanDuration`
- `activeContent` (for Office documents with macros)

### Key Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `scanResult` | integer | **0 = clean**, **non-zero = malware count** |
| `scanId` | string | Unique identifier for this scan |
| `scanTimestamp` | string | ISO 8601 timestamp (e.g., "2022-11-02T00:55:31Z") |
| `fileName` | string | Name of the scanned file |
| `foundMalwares` | array | List of detected threats (empty if clean) |
| `fileSHA1` | string | SHA-1 hash of the file |
| `fileSHA256` | string | SHA-256 hash of the file |

### Malware Detection Logic

**Clean File:**
```json
{
  "scanResult": 0,
  "foundMalwares": []
}
```

**Malicious File:**
```json
{
  "scanResult": 2,
  "foundMalwares": [
    {
      "fileName": "malicious.exe",
      "malwareName": "Trojan.Win32.Generic"
    },
    {
      "fileName": "embedded.dll",
      "malwareName": "Backdoor.Win32.Agent"
    }
  ]
}
```

### PHP Response Handling Example
```php
$response = curl_exec($ch);
$httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($httpCode === 200) {
    $result = json_decode($response, true);

    if ($result['scanResult'] === 0) {
        // File is clean
        echo "File is safe to use.";
    } else {
        // Malware detected
        $malwareCount = $result['scanResult'];
        $threats = $result['foundMalwares'];

        foreach ($threats as $malware) {
            echo "Threat: {$malware['malwareName']} in {$malware['fileName']}\n";
        }
    }
} else {
    // Handle HTTP errors
    echo "Scan failed with HTTP code: $httpCode";
}
```

---

## 5. File Size Limits

### Official Documentation
The official SDK documentation **does not specify explicit file size limits** for scanning operations.

### Practical Considerations
- **gRPC SDKs:** Support streaming, which typically allows larger files (potentially 1GB+)
- **REST API:** Likely has a maximum upload size (commonly 128MB-256MB for cloud services)
- **Network timeouts:** Large files may trigger timeout errors depending on network conditions

### Recommendations
1. Test with your specific file sizes in a development environment
2. Implement chunked uploads for files larger than 100MB
3. Set appropriate timeout values in your HTTP client
4. Contact Trend Micro support for enterprise-specific file size limits

---

## 6. SDK and Code Examples

### Available Official SDKs

| Language | Repository | Package Manager |
|----------|------------|-----------------|
| Python | [tm-v1-fs-python-sdk](https://github.com/trendmicro/tm-v1-fs-python-sdk) | `pip install visionone-filesecurity` |
| Node.js | [tm-v1-fs-nodejs-sdk](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk) | `npm install file-security-sdk` |
| Java | [tm-v1-fs-java-sdk](https://github.com/trendmicro/tm-v1-fs-java-sdk) | Maven: `com.trend:file-security-java-sdk` |
| Go | [tm-v1-fs-golang-sdk](https://github.com/trendmicro/tm-v1-fs-golang-sdk) | `go get github.com/trendmicro/tm-v1-fs-golang-sdk` |

### Python Example (Complete)
```python
#!/usr/bin/env python3
import visionone_filesecurity as amaas
import json

def scan_file(file_path, api_key, region="us-east-1"):
    """
    Scan a file for malware using Trend Micro Vision One File Security.

    Args:
        file_path: Path to the file to scan
        api_key: Trend Vision One API key
        region: Service region (default: us-east-1)

    Returns:
        dict: Scan result with malware detection status
    """
    try:
        # Initialize connection
        handle = amaas.grpc.init_by_region(region=region, api_key=api_key)

        # Perform scan
        result_json = amaas.grpc.scan_file(
            handle,
            file_name=file_path,
            tags=["upload", "user-content"],
            pml=True,      # Enable Predictive ML
            feedback=True, # Enable SPN feedback
            verbose=False  # Concise response
        )

        # Parse result
        result = json.loads(result_json)

        # Clean up
        amaas.grpc.quit(handle)

        return result

    except Exception as e:
        print(f"Scan error: {str(e)}")
        return None

# Usage
if __name__ == "__main__":
    result = scan_file(
        file_path="/uploads/user_file.pdf",
        api_key="YOUR_API_KEY_HERE",
        region="us-east-1"
    )

    if result:
        if result['scanResult'] == 0:
            print("File is clean")
        else:
            print(f"Malware detected: {result['foundMalwares']}")
```

### Node.js Example (Complete)
```javascript
import { AmaasGrpcClient } from "file-security-sdk";
import { readFileSync } from "fs";

async function scanFile(filePath, apiKey, region = "us-east-1") {
  try {
    // Initialize client
    const scanClient = new AmaasGrpcClient(region, apiKey);

    // Scan file
    const result = await scanClient.scanFile(
      filePath,
      ["upload", "user-content"],
      true,  // pml
      true   // feedback
    );

    // Close connection
    await scanClient.close();

    return result;

  } catch (error) {
    console.error("Scan error:", error.message);
    return null;
  }
}

// Usage
(async () => {
  const result = await scanFile(
    "/uploads/user_file.pdf",
    "YOUR_API_KEY_HERE",
    "us-east-1"
  );

  if (result) {
    if (result.scanResult === 0) {
      console.log("File is clean");
    } else {
      console.log("Malware detected:", result.foundMalwares);
    }
  }
})();
```

### Java Example (Complete)
```java
import com.trend.cloudone.amaas.AMaasClient;
import com.trend.cloudone.amaas.AMaasException;
import com.trend.cloudone.amaas.scan.AMaasScanOptions;
import com.google.gson.Gson;
import com.google.gson.JsonObject;

public class FileScanExample {

    public static JsonObject scanFile(String filePath, String apiKey, String region) {
        AMaasClient client = null;

        try {
            // Initialize client
            client = new AMaasClient(region, apiKey);

            // Configure scan options
            AMaasScanOptions options = AMaasScanOptions.builder()
                .pml(true)
                .feedback(true)
                .verbose(false)
                .tagList(new String[]{"upload", "user-content"})
                .build();

            // Perform scan
            String resultJson = client.scanFile(filePath, true, options);

            // Parse result
            Gson gson = new Gson();
            JsonObject result = gson.fromJson(resultJson, JsonObject.class);

            return result;

        } catch (AMaasException e) {
            System.err.println("Scan error: " + e.getMessage());
            return null;

        } finally {
            if (client != null) {
                try {
                    client.close();
                } catch (AMaasException e) {
                    System.err.println("Close error: " + e.getMessage());
                }
            }
        }
    }

    public static void main(String[] args) {
        JsonObject result = scanFile(
            "/uploads/user_file.pdf",
            "YOUR_API_KEY_HERE",
            "us-east-1"
        );

        if (result != null) {
            int scanResult = result.get("scanResult").getAsInt();

            if (scanResult == 0) {
                System.out.println("File is clean");
            } else {
                System.out.println("Malware detected: " +
                    result.get("foundMalwares").toString());
            }
        }
    }
}
```

### PHP Example (REST API - Based on Provided Snippet)
```php
<?php

/**
 * Scan a file using Trend Micro Vision One File Security REST API
 *
 * @param string $filePath Path to file to scan
 * @param string $apiKey Trend Vision One API key
 * @param string $region Service region (default: us-east-1)
 * @return array|null Scan result or null on error
 */
function scanFile($filePath, $apiKey, $region = 'us-east-1') {
    // Construct endpoint URL
    $apiEndpoint = "https://antimalware.{$region}.cloudone.trendmicro.com/api/v1/scan";

    // Read file content
    $fileContent = file_get_contents($filePath);
    if ($fileContent === false) {
        error_log("Failed to read file: $filePath");
        return null;
    }

    // Prepare headers
    $headers = [
        'Authorization: ApiKey ' . $apiKey,
        'Content-Type: application/octet-stream',
    ];

    // Initialize cURL
    $ch = curl_init($apiEndpoint);
    curl_setopt($ch, CURLOPT_POST, true);
    curl_setopt($ch, CURLOPT_POSTFIELDS, $fileContent);
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_TIMEOUT, 60); // 60 second timeout
    curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, true); // Enable SSL verification

    // Execute request
    $response = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $curlError = curl_error($ch);
    curl_close($ch);

    // Handle errors
    if ($response === false) {
        error_log("cURL error: $curlError");
        return null;
    }

    if ($httpCode !== 200) {
        error_log("HTTP error: $httpCode - $response");
        return null;
    }

    // Parse JSON response
    $result = json_decode($response, true);
    if ($result === null) {
        error_log("Failed to parse JSON response");
        return null;
    }

    return $result;
}

/**
 * Check if scan result indicates malware
 *
 * @param array $scanResult Result from scanFile()
 * @return bool True if malware detected, false if clean
 */
function isMalwareDetected($scanResult) {
    return isset($scanResult['scanResult']) && $scanResult['scanResult'] > 0;
}

/**
 * Get list of detected malware
 *
 * @param array $scanResult Result from scanFile()
 * @return array List of malware names
 */
function getDetectedMalware($scanResult) {
    if (!isset($scanResult['foundMalwares']) || !is_array($scanResult['foundMalwares'])) {
        return [];
    }

    $malwareList = [];
    foreach ($scanResult['foundMalwares'] as $malware) {
        $malwareList[] = [
            'name' => $malware['malwareName'] ?? 'Unknown',
            'file' => $malware['fileName'] ?? 'Unknown',
        ];
    }

    return $malwareList;
}

// Usage example
$apiKey = getenv('TREND_MICRO_API_KEY'); // Store API key in environment variable
$region = 'us-east-1';
$uploadedFile = '/tmp/uploads/user_document.pdf';

$result = scanFile($uploadedFile, $apiKey, $region);

if ($result === null) {
    // Scan failed - handle error
    http_response_code(500);
    echo json_encode(['error' => 'Scan failed']);
    exit;
}

if (isMalwareDetected($result)) {
    // Malware detected - reject file
    $threats = getDetectedMalware($result);

    http_response_code(403);
    echo json_encode([
        'status' => 'malware_detected',
        'threats' => $threats,
        'scan_id' => $result['scanId'] ?? null,
    ]);

    // Delete infected file
    unlink($uploadedFile);

} else {
    // File is clean - proceed with upload
    echo json_encode([
        'status' => 'clean',
        'scan_id' => $result['scanId'] ?? null,
        'sha256' => $result['fileSHA256'] ?? null,
    ]);
}
```

---

## 7. Advanced Features

### Active Content Detection (Office Macros)
Vision One File Security can detect VBA macros and executable content in Microsoft Office documents:

```json
{
  "scanResult": 0,
  "activeContent": {
    "type": "macro",
    "detected": true
  }
}
```

**Possible values:**
- `"macro"` - VBA macros detected
- `"script"` - Script content detected

### Predictive Machine Learning (PML)
Enable PML for enhanced detection of zero-day threats:
- Set `pml=true` in scan options
- Requires "Smart Protection Network" to be enabled in your Trend Vision One account

### Tagging
Organize scans with custom tags:
- Maximum 8 tags per scan
- Maximum 63 characters per tag
- Useful for categorizing uploads by source, user, or file type

**Example:**
```javascript
["user-upload", "pdf", "customer-123", "high-priority"]
```

---

## 8. Error Handling

### Common HTTP Error Codes (REST API)

| Code | Meaning | Action |
|------|---------|--------|
| 200 | Success | Parse scan result |
| 400 | Bad Request | Check file format and headers |
| 401 | Unauthorized | Verify API key is valid |
| 403 | Forbidden | Check API key permissions |
| 413 | Payload Too Large | File exceeds size limit |
| 429 | Too Many Requests | Implement rate limiting/backoff |
| 500 | Internal Server Error | Retry with exponential backoff |
| 503 | Service Unavailable | Retry later |

### SDK Exception Handling

**Python:**
```python
try:
    result = amaas.grpc.scan_file(handle, file_path)
except Exception as e:
    # Handle network errors, invalid API keys, etc.
    print(f"Error: {e}")
```

**Node.js:**
```javascript
try {
  const result = await scanClient.scanFile(filePath);
} catch (error) {
  // Handle errors
  console.error(error.message);
}
```

**Java:**
```java
try {
    String result = client.scanFile(filePath, true, options);
} catch (AMaasException e) {
    System.err.println("Error: " + e.getMessage());
}
```

---

## 9. Security Considerations

### TLS Encryption
All communication between clients and Trend Vision One File Security is encrypted with TLS 1.2+:
- **gRPC:** TLS enabled by default
- **REST API:** Always use HTTPS endpoints

### API Key Protection
- Store API keys in environment variables or secure key management systems
- Never hardcode API keys in source code
- Rotate API keys regularly (annually or more frequently)
- Use the principle of least privilege when assigning API key permissions

### File Handling
- Scan files BEFORE processing or storing them permanently
- Delete infected files immediately after detection
- Implement quarantine mechanisms for suspicious files
- Log all scan results for audit trails

---

## 10. Integration Checklist

- [ ] Obtain API key from Trend Vision One console with "Run file scan via SDK" permission
- [ ] Identify your service region (e.g., us-east-1, eu-central-1)
- [ ] Choose integration method:
  - [ ] Official SDK (Python, Node.js, Java, Go)
  - [ ] Direct REST API (custom implementation)
- [ ] Implement file scanning in upload workflow:
  - [ ] Scan files before saving to permanent storage
  - [ ] Handle scan results (clean vs. malicious)
  - [ ] Implement error handling and retries
- [ ] Configure timeout values appropriate for your file sizes
- [ ] Test with EICAR test files to verify malware detection
- [ ] Set up logging and monitoring for scan operations
- [ ] Implement rate limiting to avoid API throttling
- [ ] Document API key rotation procedures

---

## 11. Additional Resources

### Official Documentation
- [Trend Vision One Documentation](https://docs.trendmicro.com/en-us/documentation/trend-vision-one/)
- [Using the SDK](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-using-sdk)
- [Code Examples](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-code-example)
- [Java API Reference](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-java-api-reference)

### GitHub Repositories
- [Python SDK](https://github.com/trendmicro/tm-v1-fs-python-sdk)
- [Node.js SDK](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk)
- [Java SDK](https://github.com/trendmicro/tm-v1-fs-java-sdk)
- [Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk)

### Support
- Trend Micro Support Portal: https://success.trendmicro.com/
- API Documentation: https://automation.trendmicro.com/xdr/Guides/API-documentation/

---

## 12. Testing with EICAR

To verify your integration, use the EICAR test file (a harmless malware signature used for testing):

```
X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*
```

Save this string to a file and scan it. A properly configured scanner will detect it as:
- Malware name: `Eicar_test_file` or `EICAR-AV-Test`
- `scanResult`: 1 (indicating 1 threat found)

**Important:** Never use real malware samples for testing.

---

## Summary

Trend Micro Vision One File Security provides robust malware scanning through:
1. **gRPC-based SDKs** (official, recommended) for Python, Node.js, Java, and Go
2. **Potential REST API** for direct HTTP integration (as indicated by the PHP snippet)
3. **API Key authentication** with regional endpoints
4. **Structured JSON responses** with clear `scanResult` field (0=clean, >0=malware)
5. **Advanced features** including PML detection, active content scanning, and tagging

The service is production-ready with TLS encryption, multiple region support, and comprehensive SDKs for popular programming languages.

---

**Document Version:** 1.0
**Last Updated:** 2025-12-09
**Author:** Research Agent
**Status:** Complete
