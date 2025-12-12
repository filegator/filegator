<?php
/**
 * Trend Micro Vision One File Security SDK for PHP
 *
 * @package TrendAndrew\FileSecurity
 * @license MIT
 */

namespace TrendAndrew\FileSecurity\Http;

use TrendAndrew\FileSecurity\Exception\AmaasException;
use TrendAndrew\FileSecurity\Exception\AuthenticationException;
use TrendAndrew\FileSecurity\Exception\ConnectionException;
use TrendAndrew\FileSecurity\Exception\TimeoutException;
use TrendAndrew\FileSecurity\Model\ScanOptions;

/**
 * HTTP Client for AMaaS API communication
 *
 * Handles all HTTP communication with the Trend Micro Vision One
 * File Security API including:
 * - File upload for scanning
 * - Request signing and authentication
 * - Response parsing
 * - Error handling
 */
class HttpClient
{
    /**
     * API endpoint base path
     */
    private const API_PATH = '/api/v1/scan';

    /**
     * User agent string
     */
    private const USER_AGENT = 'TrendMicro-FileSecurity-PHP-SDK/1.0.0';

    /**
     * @var string Base endpoint URL
     */
    private string $endpoint;

    /**
     * @var string API key for authentication
     */
    private string $apiKey;

    /**
     * @var int Timeout in seconds
     */
    private int $timeout;

    /**
     * @var bool Whether TLS is enabled
     */
    private bool $enableTls;

    /**
     * @var string|null Custom CA certificate path
     */
    private ?string $caCert;

    /**
     * @var resource|null cURL handle for reuse
     */
    private $curlHandle = null;

    /**
     * Create a new HTTP client
     *
     * @param string $endpoint Base API endpoint URL
     * @param string $apiKey API key for authentication
     * @param int $timeout Timeout in seconds
     * @param bool $enableTls Whether to enable TLS
     * @param string|null $caCert Custom CA certificate path
     */
    public function __construct(
        string $endpoint,
        string $apiKey,
        int $timeout = 300,
        bool $enableTls = true,
        ?string $caCert = null
    ) {
        $this->endpoint = rtrim($endpoint, '/');
        $this->apiKey = $apiKey;
        $this->timeout = $timeout;
        $this->enableTls = $enableTls;
        $this->caCert = $caCert;
    }

    /**
     * Perform a file scan
     *
     * @param string $fileName Name of the file
     * @param string $content File content
     * @param ScanOptions $options Scan options
     * @return array Scan result data
     *
     * @throws AmaasException If scan fails
     * @throws AuthenticationException If authentication fails
     * @throws ConnectionException If connection fails
     * @throws TimeoutException If request times out
     */
    public function scan(string $fileName, string $content, ScanOptions $options): array
    {
        $url = $this->endpoint . self::API_PATH;

        // Prepare multipart form data
        $boundary = '----TrendMicro' . uniqid();
        $body = $this->buildMultipartBody($boundary, $fileName, $content, $options);

        $headers = [
            'Authorization: Bearer ' . $this->apiKey,
            'Content-Type: multipart/form-data; boundary=' . $boundary,
            'Accept: application/json',
            'User-Agent: ' . self::USER_AGENT,
        ];

        return $this->sendRequest('POST', $url, $body, $headers);
    }

    /**
     * Build multipart form body
     *
     * @param string $boundary Boundary string
     * @param string $fileName File name
     * @param string $content File content
     * @param ScanOptions $options Scan options
     * @return string Multipart body
     */
    private function buildMultipartBody(
        string $boundary,
        string $fileName,
        string $content,
        ScanOptions $options
    ): string {
        $parts = [];

        // File part
        $parts[] = "--{$boundary}";
        $parts[] = 'Content-Disposition: form-data; name="file"; filename="' . $fileName . '"';
        $parts[] = 'Content-Type: application/octet-stream';
        $parts[] = '';
        $parts[] = $content;

        // Tags
        $tags = $options->getTags();
        if (!empty($tags)) {
            $parts[] = "--{$boundary}";
            $parts[] = 'Content-Disposition: form-data; name="tags"';
            $parts[] = '';
            $parts[] = json_encode($tags);
        }

        // PML setting
        if (!$options->isPmlEnabled()) {
            $parts[] = "--{$boundary}";
            $parts[] = 'Content-Disposition: form-data; name="pml"';
            $parts[] = '';
            $parts[] = 'false';
        }

        // Feedback setting
        if (!$options->isFeedbackEnabled()) {
            $parts[] = "--{$boundary}";
            $parts[] = 'Content-Disposition: form-data; name="feedback"';
            $parts[] = '';
            $parts[] = 'false';
        }

        $parts[] = "--{$boundary}--";
        $parts[] = '';

        return implode("\r\n", $parts);
    }

    /**
     * Send HTTP request
     *
     * @param string $method HTTP method
     * @param string $url Request URL
     * @param string $body Request body
     * @param array $headers Request headers
     * @return array Response data
     *
     * @throws AmaasException If request fails
     * @throws AuthenticationException If authentication fails
     * @throws ConnectionException If connection fails
     * @throws TimeoutException If request times out
     */
    private function sendRequest(string $method, string $url, string $body, array $headers): array
    {
        $ch = $this->getCurlHandle();

        curl_setopt_array($ch, [
            CURLOPT_URL => $url,
            CURLOPT_CUSTOMREQUEST => $method,
            CURLOPT_POSTFIELDS => $body,
            CURLOPT_HTTPHEADER => $headers,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT => $this->timeout,
            CURLOPT_CONNECTTIMEOUT => 30,
            CURLOPT_FOLLOWLOCATION => true,
            CURLOPT_MAXREDIRS => 3,
        ]);

        // TLS settings
        if ($this->enableTls) {
            curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, true);
            curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, 2);

            if ($this->caCert) {
                curl_setopt($ch, CURLOPT_CAINFO, $this->caCert);
            }
        } else {
            curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, false);
            curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, 0);
        }

        // Execute request
        $response = curl_exec($ch);
        $errno = curl_errno($ch);
        $error = curl_error($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);

        // Handle cURL errors
        if ($errno !== 0) {
            $this->handleCurlError($errno, $error, $url);
        }

        // Handle HTTP errors
        if ($httpCode >= 400) {
            $this->handleHttpError($httpCode, $response);
        }

        // Parse JSON response
        $data = json_decode($response, true);
        if (json_last_error() !== JSON_ERROR_NONE) {
            throw new AmaasException('Invalid JSON response from API: ' . json_last_error_msg());
        }

        return $data;
    }

    /**
     * Handle cURL errors
     *
     * @param int $errno cURL error number
     * @param string $error cURL error message
     * @param string $url Request URL
     *
     * @throws ConnectionException
     * @throws TimeoutException
     */
    private function handleCurlError(int $errno, string $error, string $url): void
    {
        // Timeout errors
        if ($errno === CURLE_OPERATION_TIMEDOUT || $errno === CURLE_OPERATION_TIMEOUTED) {
            throw TimeoutException::requestTimeout($this->timeout);
        }

        // Connection errors
        if (in_array($errno, [
            CURLE_COULDNT_CONNECT,
            CURLE_COULDNT_RESOLVE_HOST,
            CURLE_COULDNT_RESOLVE_PROXY,
        ])) {
            throw ConnectionException::fromCurlError($errno, $error, $url);
        }

        // SSL errors
        if (in_array($errno, [
            CURLE_SSL_CONNECT_ERROR,
            CURLE_SSL_CERTPROBLEM,
            CURLE_SSL_CIPHER,
            CURLE_SSL_CACERT,
            CURLE_SSL_CACERT_BADFILE,
        ])) {
            throw ConnectionException::tlsError($url, $error);
        }

        // Generic connection error
        throw ConnectionException::fromCurlError($errno, $error, $url);
    }

    /**
     * Handle HTTP errors
     *
     * @param int $httpCode HTTP status code
     * @param string $response Response body
     *
     * @throws AuthenticationException
     * @throws AmaasException
     */
    private function handleHttpError(int $httpCode, string $response): void
    {
        $data = json_decode($response, true) ?? [];
        $message = $data['message'] ?? $data['error'] ?? 'Unknown error';

        // Authentication errors
        if ($httpCode === 401) {
            throw AuthenticationException::invalidApiKey();
        }

        if ($httpCode === 403) {
            throw AuthenticationException::insufficientPermissions();
        }

        // Rate limiting
        if ($httpCode === 429) {
            throw new AmaasException('Rate limit exceeded. Please try again later.', $httpCode);
        }

        // Server errors
        if ($httpCode >= 500) {
            throw new AmaasException("Server error: {$message}", $httpCode, null, null, $data);
        }

        // Other client errors
        throw new AmaasException("API error ({$httpCode}): {$message}", $httpCode, null, null, $data);
    }

    /**
     * Get or create cURL handle
     *
     * @return resource cURL handle
     */
    private function getCurlHandle()
    {
        if ($this->curlHandle === null) {
            $this->curlHandle = curl_init();
        } else {
            curl_reset($this->curlHandle);
        }

        return $this->curlHandle;
    }

    /**
     * Close the HTTP client and release resources
     */
    public function close(): void
    {
        if ($this->curlHandle !== null) {
            curl_close($this->curlHandle);
            $this->curlHandle = null;
        }
    }

    /**
     * Destructor
     */
    public function __destruct()
    {
        $this->close();
    }
}
