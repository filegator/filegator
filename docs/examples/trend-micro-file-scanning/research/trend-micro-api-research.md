# Trend Micro Vision One File Security API - Research Documentation

## Executive Summary

Trend Micro Vision One File Security is a malware scanning service that uses SDK-based integration rather than direct REST API calls. It detects all types of malicious software including trojans, ransomware, spyware, and polymorphic malware variants using machine learning and signature-based detection.

**Key Findings:**
- SDK-based integration (not direct REST API)
- Region-based endpoints with gRPC protocol
- API key authentication
- JSON response format with clear malware indicators
- File size limit: 1 MB for API requests
- Rate limit: 60-second windows (429 error when exceeded)
- Support for file and buffer scanning

---

## 1. API Endpoint Structure

### Base URL Pattern

The Trend Micro Vision One File Security API uses region-based endpoints with the following format:

```
antimalware.__REGION__.cloudone.trendmicro.com:443
```

**Supported Regions:**
- `us-east-1` (US East - Virginia)
- `eu-central-1` (Europe - Frankfurt)
- `ap-northeast-1` (Asia Pacific - Tokyo)
- `ap-southeast-1` (Asia Pacific - Singapore)
- `ap-southeast-2` (Asia Pacific - Sydney)
- `ap-south-1` (Asia Pacific - Mumbai)
- `me-central-1` (Middle East - UAE)

**Example:**
```
antimalware.us-east-1.cloudone.trendmicro.com:443
```

### Protocol

The API uses **gRPC** protocol over TLS 1.2 with AEAD cipher suites for secure communication. All data transmission is encrypted using publicly-signed certificates from Trend Micro Inc.

---

## 2. Authentication Method

### API Key Authentication

Authentication is handled through API keys rather than traditional HTTP headers like Bearer tokens.

**API Key Requirements:**
- Must have the role permission: **"Run file scan via SDK"**
- API keys are region-specific
- Default expiration: 1 year (configurable by Master Administrator)

**How to Obtain API Key:**
1. Log into Trend Vision One console
2. Navigate to **Administration > API Keys**
3. Click **Add API Key**
4. Configure with role containing "Run file scan via SDK" permission
5. Record the API key securely
6. Set an expiry time for security best practices

**Authentication in SDK:**
```javascript
// Node.js Example
import { AmaasGrpcClient } from "file-security-sdk";

const amaasHostName = "us-east-1"; // or full hostname
const apiKey = "YOUR_VISION_ONE_API_KEY";

let scanClient = new AmaasGrpcClient(amaasHostName, apiKey);
```

```python
# Python Example
import amaas.grpc

handle = amaas.grpc.init_by_region(
    region="us-east-1",
    api_key="YOUR_API_KEY"
)
```

---

## 3. Request Format for File Scanning

### SDK-Based Integration (Recommended)

Trend Micro provides official SDKs for multiple languages:
- **Node.js**: `file-security-sdk` (requires Node.js 20.19.0+)
- **Python**: `tm-v1-fs-python-sdk`
- **Java**: `tm-v1-fs-java-sdk`
- **Go**: `tm-v1-fs-golang-sdk`

### Scan Methods

#### A. File Scanning

Scan a file directly from the filesystem:

```javascript
// Node.js
const fileScanResult = await scanClient.scanFile(
  "path/to/file.ext",           // File path
  ["tag1", "tag2", "tag3"],     // Tags (max 8, each max 63 chars)
  true,                          // PML flag (Predictive Machine Learning)
  true                           // Feedback flag
);
```

```python
# Python
result = amaas.grpc.scan_file(
    handle,
    file_name="path/to/file.exe",
    tags=["tag1", "tag2"],
    pml=True
)
```

**Parameters:**
- **file_name/path**: Full path to the file to scan
- **tags**: Optional list of strings to tag the scan (max 8 tags, 63 chars each)
- **pml**: Enable Predictive Machine Learning detection (boolean)
- **feedback**: Enable feedback submission (boolean)

#### B. Buffer Scanning

Scan file contents from memory buffer:

```javascript
// Node.js
import { readFileSync } from "fs/promises";

const buff = await readFileSync("path/to/file.ext");
const bufferScanResult = await scanClient.scanBuffer(
  "FILE_IDENTIFIER",            // Identifier for the buffer
  buff,                          // File buffer
  ["tag1", "tag2"],             // Tags
  true,                          // PML flag
  true                           // Feedback flag
);
```

**Use Cases:**
- Scanning uploaded files before saving to disk
- Scanning files from cloud storage
- Scanning generated/temporary files

---

## 4. Response Format

### Response Structure

The API returns JSON-formatted scan results with the following structure:

```json
{
  "version": "1.0",
  "scannerVersion": "1.0.0-27",
  "schemaVersion": "1.0.0",
  "scanResult": 1,
  "scanId": "25072030425f4f4d68953177d0628d0b",
  "scanTimestamp": "2022-11-02T00:55:31Z",
  "fileName": "EICAR_TEST_FILE-1.exe",
  "filePath": "path/to/file.exe",
  "foundMalwares": [
    {
      "fileName": "Eicar.exe",
      "malwareName": "Eicar_test_file"
    }
  ],
  "fileSHA1": "3395856ce81f2b7382dee72602f798b642f14140",
  "fileSHA256": "7dddcd0f64165f51291a41f49b6246cf85c3e6e599c096612cccce09566091f2"
}
```

### Key Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | String | API version |
| `scannerVersion` | String | Scanner engine version |
| `schemaVersion` | String | Response schema version |
| `scanResult` | Integer | **Number of malware items found** (0 = clean) |
| `scanId` | String | Unique scan identifier (UUID) |
| `scanTimestamp` | String | ISO 8601 timestamp of scan |
| `fileName` | String | Name of the scanned file |
| `filePath` | String | Path of the scanned file |
| `foundMalwares` | Array | List of detected malware (empty if clean) |
| `fileSHA1` | String | SHA-1 hash of the file |
| `fileSHA256` | String | SHA-256 hash of the file |

### Determining Scan Results

#### Clean File (No Malware)

```json
{
  "scanResult": 0,
  "foundMalwares": [],
  // ... other fields
}
```

**Detection Logic:**
```javascript
if (result.scanResult === 0 && result.foundMalwares.length === 0) {
  console.log("File is clean");
}
```

#### Malware Detected

```json
{
  "scanResult": 1,
  "foundMalwares": [
    {
      "fileName": "malicious_file.exe",
      "malwareName": "Trojan.Win32.Generic"
    }
  ],
  // ... other fields
}
```

**Detection Logic:**
```javascript
if (result.scanResult > 0 || result.foundMalwares.length > 0) {
  console.log(`Malware detected: ${result.foundMalwares.length} threats`);
  result.foundMalwares.forEach(malware => {
    console.log(`- ${malware.malwareName} in ${malware.fileName}`);
  });
}
```

### foundMalwares Array Structure

Each object in the `foundMalwares` array contains:
- **fileName**: The name of the file where malware was detected
- **malwareName**: The identification name of the virus/malware
- **type** (optional): Type of active content detected (`macro` or `script`)

### Active Content Detection

The scanner can detect potentially malicious active content:
- **PDF scripts**: Embedded JavaScript in PDF files
- **Office macros**: VBA macros in Microsoft Office documents

When active content is detected, the response includes a `type` field with values:
- `macro` - Office macros detected
- `script` - PDF scripts detected

---

## 5. Error Response Formats

### Common Error Types

#### A. File Access Errors

```json
{
  "error": "EACCES: permission denied, open /path/to/file"
}
```

**Cause:** SDK lacks read permission for the specified file path

**Resolution:** Ensure the application has proper file system permissions

#### B. Invalid Region

```json
{
  "error": "Invalid region specified"
}
```

**Cause:** The region parameter doesn't match API key region or is invalid

**Resolution:** Verify region matches one of the supported regions and matches API key

#### C. Authentication Errors

```json
{
  "error": "Invalid API key or insufficient permissions"
}
```

**Cause:** API key is invalid, expired, or lacks "Run file scan via SDK" permission

**Resolution:** Regenerate API key with proper permissions

#### D. gRPC Status Codes

The SDK reports gRPC errors with standard status codes:
- `UNAUTHENTICATED` (16): Invalid credentials
- `PERMISSION_DENIED` (7): Insufficient permissions
- `RESOURCE_EXHAUSTED` (8): Rate limit exceeded
- `INVALID_ARGUMENT` (3): Invalid parameters
- `UNAVAILABLE` (14): Service temporarily unavailable

Reference: [gRPC Status Codes Documentation](https://grpc.github.io/grpc/core/md_doc_statuscodes.html)

#### E. Rate Limit Exceeded (HTTP 429)

```json
{
  "error": "Too many API requests",
  "statusCode": 429
}
```

**Cause:** Exceeded API rate limit within 60-second window

**Resolution:** Implement exponential backoff and retry logic

---

## 6. Rate Limits and File Size Limits

### Rate Limits

**General API Rate Limits:**
- Measured within **60-second windows**
- Returns HTTP **429 (Too Many Requests)** when exceeded
- Rate limits are subject to change
- Requests timeout after **60 seconds**

**Best Practice:**
```javascript
async function scanWithRetry(scanClient, filePath, maxRetries = 3) {
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await scanClient.scanFile(filePath, [], true, true);
    } catch (error) {
      if (error.statusCode === 429 && attempt < maxRetries) {
        const backoffMs = Math.pow(2, attempt) * 1000; // Exponential backoff
        await new Promise(resolve => setTimeout(resolve, backoffMs));
        continue;
      }
      throw error;
    }
  }
}
```

### File Size Limits

**API Request Limits:**
- **Maximum body size**: 1 MB
- Requests exceeding this limit receive **HTTP 413 (Payload Too Large)**

**Tag Constraints:**
- Maximum tags per scan: **8**
- Maximum length per tag: **63 characters**

**Considerations:**
- For large file scanning, ensure files are under 1 MB
- Consider chunked uploads or alternative storage scanning methods for larger files
- The SDK handles file streaming efficiently within the limit

---

## 7. Best Practices for Integration

### Security Best Practices

#### A. API Key Management
- Store API keys securely (environment variables, secrets manager)
- Never hardcode API keys in source code
- Set expiration dates for API keys
- Rotate keys regularly
- Use different keys for different environments (dev, staging, prod)

```javascript
// Good Practice
const apiKey = process.env.TRENDMICRO_API_KEY;

// Bad Practice
const apiKey = "sk-1234567890abcdef"; // Hardcoded
```

#### B. TLS/Encryption
- Always enable TLS (default in SDK)
- SDK uses TLS 1.2 with AEAD cipher suites
- Server certificates are publicly signed by trusted CA
- Never disable TLS in production

```javascript
// TLS is enabled by default - no action needed
let scanClient = new AmaasGrpcClient(region, apiKey);
```

#### C. Data Privacy
- Customer data is segregated by Customer ID
- Data is encrypted at rest and in transit
- Follows ISO 27001 and ISO 27017 compliance standards
- Implements SANS 25/OWASP Top 10 secure coding practices

### Performance Best Practices

#### A. Thread Safety
```javascript
// scanFile() and scanBuffer() are thread-safe
// Safe for concurrent execution from multiple threads
const promises = files.map(file =>
  scanClient.scanFile(file, [], true, true)
);
const results = await Promise.all(promises);
```

#### B. Connection Management
```javascript
// Create client once and reuse
const scanClient = new AmaasGrpcClient(region, apiKey);

// Scan multiple files
for (const file of files) {
  await scanClient.scanFile(file, [], true, true);
}

// Close when done
scanClient.close();
```

#### C. Error Handling
```javascript
try {
  const result = await scanClient.scanFile(filePath, [], true, true);

  if (result.scanResult > 0) {
    // Handle malware detection
    console.error(`Malware found: ${result.foundMalwares[0].malwareName}`);
    // Quarantine file, notify admin, etc.
  } else {
    // File is clean
    console.log("File is safe");
  }
} catch (error) {
  if (error.code === 'EACCES') {
    console.error("Permission denied accessing file");
  } else if (error.statusCode === 429) {
    console.error("Rate limit exceeded - retry later");
  } else {
    console.error("Scan error:", error.message);
  }
}
```

### Integration Architecture

#### A. Workflow Integration
```
Upload → Temporary Storage → Scan → Decision (Clean/Malware) → Action
                                                |
                                                ├─ Clean: Move to permanent storage
                                                └─ Malware: Quarantine & notify
```

#### B. CI/CD Pipeline Integration
- Scan files before distribution
- Add scanning step in build pipeline
- Fail builds if malware detected
- Use SDK or CLI in automated workflows

#### C. Event-Driven Architecture
```javascript
// Example: Scan on file upload event
app.post('/upload', upload.single('file'), async (req, res) => {
  try {
    // Scan uploaded file
    const result = await scanClient.scanFile(req.file.path, [], true, true);

    if (result.scanResult > 0) {
      // Delete malicious file
      fs.unlinkSync(req.file.path);
      return res.status(400).json({
        error: 'Malware detected',
        details: result.foundMalwares
      });
    }

    // File is clean - proceed with storage
    await moveToStorage(req.file.path);
    res.json({ success: true, scanId: result.scanId });
  } catch (error) {
    console.error('Scan failed:', error);
    res.status(500).json({ error: 'Scan failed' });
  }
});
```

### Monitoring and Logging

#### A. Logging Best Practices
```javascript
// Log scan results for audit trail
logger.info({
  event: 'file_scan_completed',
  scanId: result.scanId,
  fileName: result.fileName,
  scanResult: result.scanResult,
  malwareDetected: result.scanResult > 0,
  timestamp: result.scanTimestamp,
  fileSHA256: result.fileSHA256
});
```

#### B. Metrics to Track
- Total scans performed
- Malware detection rate
- Scan duration/performance
- Error rates by type
- Rate limit hits
- File types scanned

### Deployment Options

Trend Vision One File Security supports multiple deployment methods:

1. **SDK Integration** (Recommended)
   - Node.js, Python, Java, Go SDKs
   - Direct integration into applications
   - Flexible and customizable

2. **CLI (Command Line Interface)**
   - For shell scripts and automation
   - Batch file scanning

3. **AWS CloudFormation Templates**
   - For cloud storage scanning
   - S3 bucket integration

4. **Virtual Appliance**
   - On-premises deployment
   - Behind firewall in cloud/on-prem environments

---

## 8. SDK Installation and Setup

### Node.js SDK

```bash
npm install file-security-sdk
```

**Requirements:**
- Node.js 20.19.0 or higher

**Basic Setup:**
```javascript
import { AmaasGrpcClient, LogLevel } from "file-security-sdk";

const region = "us-east-1";
const apiKey = process.env.TRENDMICRO_API_KEY;

let scanClient = new AmaasGrpcClient(region, apiKey);

// Optional: Set log level
scanClient.setLogLevel(LogLevel.INFO);
```

### Python SDK

```bash
pip install amaas
```

**Basic Setup:**
```python
import amaas.grpc

handle = amaas.grpc.init_by_region(
    region="us-east-1",
    api_key=os.environ.get("TRENDMICRO_API_KEY")
)
```

### Java SDK

```xml
<!-- Add to pom.xml -->
<dependency>
    <groupId>com.trendmicro</groupId>
    <artifactId>tm-v1-fs-java-sdk</artifactId>
    <version>LATEST</version>
</dependency>
```

### Go SDK

```bash
go get github.com/trendmicro/tm-v1-fs-golang-sdk
```

---

## 9. Complete Integration Example

### Node.js Express File Upload with Scanning

```javascript
import express from 'express';
import multer from 'multer';
import { AmaasGrpcClient } from 'file-security-sdk';
import fs from 'fs/promises';
import path from 'path';

const app = express();
const upload = multer({ dest: 'uploads/temp/' });

// Initialize Trend Micro client
const scanClient = new AmaasGrpcClient(
  process.env.TRENDMICRO_REGION || 'us-east-1',
  process.env.TRENDMICRO_API_KEY
);

app.post('/api/upload', upload.single('file'), async (req, res) => {
  const tempPath = req.file.path;
  const targetPath = path.join('uploads/safe/', req.file.originalname);

  try {
    // Scan the uploaded file
    console.log(`Scanning file: ${req.file.originalname}`);
    const scanResult = await scanClient.scanFile(
      tempPath,
      ['user-upload', 'web-api'],
      true,  // Enable PML
      true   // Enable feedback
    );

    // Check scan results
    if (scanResult.scanResult > 0 || scanResult.foundMalwares.length > 0) {
      // Malware detected - delete and reject
      await fs.unlink(tempPath);

      return res.status(400).json({
        success: false,
        error: 'Malware detected in uploaded file',
        scanId: scanResult.scanId,
        threats: scanResult.foundMalwares.map(m => ({
          file: m.fileName,
          malware: m.malwareName
        }))
      });
    }

    // File is clean - move to safe storage
    await fs.rename(tempPath, targetPath);

    res.json({
      success: true,
      message: 'File uploaded and scanned successfully',
      scanId: scanResult.scanId,
      fileName: req.file.originalname,
      fileHash: scanResult.fileSHA256,
      scanTimestamp: scanResult.scanTimestamp
    });

  } catch (error) {
    // Clean up temp file
    try {
      await fs.unlink(tempPath);
    } catch {}

    console.error('Scan error:', error);
    res.status(500).json({
      success: false,
      error: 'File scanning failed',
      message: error.message
    });
  }
});

// Cleanup on shutdown
process.on('SIGTERM', () => {
  scanClient.close();
});

app.listen(3000, () => {
  console.log('Server running on port 3000');
});
```

---

## 10. Testing and Validation

### EICAR Test File

Use the EICAR test file to validate integration without using actual malware:

```javascript
// EICAR test string
const eicarString = 'X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*';

// Create test file
await fs.writeFile('test-eicar.txt', eicarString);

// Scan should detect as: Eicar_test_file
const result = await scanClient.scanFile('test-eicar.txt', ['test'], true, true);
console.log(result.foundMalwares); // Should show Eicar_test_file
```

### Test Checklist

- [ ] Clean file scan returns `scanResult: 0`
- [ ] EICAR test file detected as malware
- [ ] Large files (>1MB) handled gracefully
- [ ] Invalid API key returns authentication error
- [ ] Rate limiting triggers HTTP 429
- [ ] File permission errors caught and handled
- [ ] Multiple concurrent scans work correctly
- [ ] Client connection cleanup on shutdown

---

## 11. Additional Resources

### Official Documentation
- [Trend Vision One File Security Overview](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-file-security-intro-origin)
- [Automation Center API v3.0](https://automation.trendmicro.com/xdr/api-v3/)
- [First Steps with APIs](https://docs.trendmicro.com/en-us/documentation/article/trend-micro-vision-one-automation-center-first-steps-toward-u)
- [Code Examples](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-code-example)
- [API Rate Limits](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-api-rate-limits)

### SDK Repositories
- [Node.js SDK](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk)
- [Python SDK](https://github.com/trendmicro/tm-v1-fs-python-sdk)
- [Java SDK](https://github.com/trendmicro/tm-v1-fs-java-sdk)
- [Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk)

### Community Resources
- [Trend Vision One Documentation Portal](https://docs.trendmicro.com/en-us/documentation/trend-vision-one/)
- [Update to Vision One File Security using APIs](https://cloudone.trendmicro.com/docs/file-storage-security/update-to-V1-api/)
- [Send Your First Request](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-send-your-first-request-using-api)

---

## 12. Summary and Quick Reference

### Key Implementation Points

1. **Use SDK, not REST API** - Trend Micro provides SDKs as the primary integration method
2. **Region Matters** - API key and region must match
3. **scanResult = 0 means clean** - This is the primary indicator
4. **foundMalwares array** - Check both scanResult and this array
5. **Thread-safe scanning** - Safe for concurrent operations
6. **1 MB file size limit** - Plan for larger files accordingly
7. **Rate limits apply** - Implement retry logic with exponential backoff
8. **TLS is mandatory** - Always enabled by default

### Quick Start Checklist

- [ ] Obtain API key with "Run file scan via SDK" permission
- [ ] Install SDK for your language (Node.js, Python, Java, Go)
- [ ] Configure region matching your API key
- [ ] Initialize client with region and API key
- [ ] Implement error handling for common errors
- [ ] Add retry logic for rate limits
- [ ] Test with EICAR file
- [ ] Log scan results for audit trail
- [ ] Close client connections on shutdown

### Response Decision Tree

```
Scan Response
    |
    ├─ scanResult === 0 AND foundMalwares.length === 0
    |   └─ File is CLEAN ✓
    |
    └─ scanResult > 0 OR foundMalwares.length > 0
        └─ MALWARE DETECTED ✗
            └─ Check foundMalwares array for details
```

---

**Document Version:** 1.0
**Last Updated:** 2025-12-09
**Research Conducted By:** Claude Code Research Agent
**Status:** Complete

---

## Sources

- [Trend Vision One File Security Overview](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-file-security-intro-origin)
- [Code Examples - Trend Micro Service Central](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-code-example)
- [Trend Vision One Public API (v3.0)](https://automation.trendmicro.com/xdr/api-v3/)
- [First Steps Toward Using the APIs](https://docs.trendmicro.com/en-us/documentation/article/trend-micro-vision-one-automation-center-first-steps-toward-u)
- [GitHub - Node.js SDK](https://github.com/trendmicro/tm-v1-fs-nodejs-sdk)
- [GitHub - Python SDK](https://github.com/trendmicro/tm-v1-fs-python-sdk)
- [GitHub - Java SDK](https://github.com/trendmicro/tm-v1-fs-java-sdk)
- [GitHub - Go SDK](https://github.com/trendmicro/tm-v1-fs-golang-sdk)
- [API Rate Limits Documentation](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-api-rate-limits)
- [API Request Limits - Automation Center](https://automation.trendmicro.com/xdr/Guides/API-Request-Limits/)
- [Send Your First Request Using API](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-send-your-first-request-using-api)
- [Trend Vision One Data Privacy and Security](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-data-privacy-security-compliance)
- [Malware Scanning Documentation](https://docs.trendmicro.com/en-us/documentation/article/trend-vision-one-malware-scanning)
