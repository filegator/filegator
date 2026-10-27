package s3

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/filegator/filegator/internal/config"
)

// maxSinglePut is the largest object that can be stored with one PutObject.
// Larger files need multipart upload, which this client does not implement.
const maxSinglePut = 5 << 30 // 5 GiB

// client performs signed S3 REST requests.
type client struct {
	bucket    string
	region    string
	endpoint  *url.URL
	pathStyle bool
	http      *http.Client
	signer    *signer
}

// newClient builds a client from configuration.
func newClient(cfg config.S3) (*client, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("s3: access_key and secret_key are required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	raw := cfg.Endpoint
	if raw == "" {
		raw = "https://s3." + region + ".amazonaws.com"
	}
	endpoint, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("s3: invalid endpoint %q: %w", raw, err)
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("s3: endpoint must be http or https")
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	if cfg.Timeout == 0 {
		timeout = 60 * time.Second
	}

	return &client{
		bucket:    cfg.Bucket,
		region:    region,
		endpoint:  endpoint,
		pathStyle: cfg.UsePathStyle,
		http:      &http.Client{Timeout: timeout},
		signer: &signer{
			accessKey: cfg.AccessKey,
			secretKey: cfg.SecretKey,
			region:    region,
			service:   "s3",
			now:       time.Now,
		},
	}, nil
}

// request builds a signed request for an object key. An empty key addresses the
// bucket itself.
func (c *client) request(method, key string, query url.Values, body []byte, contentType string) (*http.Request, error) {
	u := *c.endpoint
	if c.pathStyle {
		u.Path = "/" + c.bucket
		if key != "" {
			u.Path += "/" + strings.TrimPrefix(key, "/")
		}
	} else {
		u.Host = c.bucket + "." + u.Host
		if key != "" {
			u.Path = "/" + strings.TrimPrefix(key, "/")
		}
	}
	u.RawQuery = encodeQuery(query)

	var reader io.Reader
	payloadHash := sha256Empty
	if body != nil {
		reader = bytes.NewReader(body)
		payloadHash = hashHex(body)
	}
	req, err := http.NewRequest(method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.signer.sign(req, payloadHash)
	return req, nil
}

// encodeQuery builds a canonical query string from values.
func encodeQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return values.Encode()
}

// do runs a request and returns the response body, mapping errors to readable
// messages that include the S3 error code when present.
func (c *client) do(req *http.Request) (*http.Response, []byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("s3: %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	// Read a bounded amount: error bodies are small, and object downloads are
	// streamed by the caller instead of going through here.
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		return nil, nil, fmt.Errorf("s3: read response: %w", readErr)
	}
	if resp.StatusCode >= 400 {
		return resp, data, fmt.Errorf("s3: %s %s: HTTP %d: %s", req.Method, req.URL.Path, resp.StatusCode, s3Error(data))
	}
	return resp, data, nil
}

// s3Error extracts the message from an S3 XML error body.
func s3Error(body []byte) string {
	var e struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if err := xml.Unmarshal(body, &e); err == nil && e.Code != "" {
		return e.Code + ": " + e.Message
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 200 {
		text = text[:200]
	}
	if text == "" {
		text = "no response body"
	}
	return text
}

// putObject stores one object.
func (c *client) putObject(key string, body []byte, contentType string) error {
	if int64(len(body)) > maxSinglePut {
		return fmt.Errorf("s3: object is larger than the %d GiB single-upload limit", maxSinglePut>>30)
	}
	req, err := c.request(http.MethodPut, key, nil, body, contentType)
	if err != nil {
		return err
	}
	_, _, err = c.do(req)
	return err
}

// getObject reads one object. The caller closes the body.
func (c *client) getObject(key string) (io.ReadCloser, error) {
	req, err := c.request(http.MethodGet, key, nil, nil, "")
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3: get %s: %w", key, err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		return nil, fmt.Errorf("s3: get %s: HTTP %d: %s", key, resp.StatusCode, s3Error(body))
	}
	return resp.Body, nil
}

// headObject returns metadata for one object.
func (c *client) headObject(key string) (*objectMeta, error) {
	req, err := c.request(http.MethodHead, key, nil, nil, "")
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3: head %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("s3: head %s: HTTP %d", key, resp.StatusCode)
	}

	size, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	modified, _ := time.Parse(http.TimeFormat, resp.Header.Get("Last-Modified"))
	return &objectMeta{
		key:      key,
		size:     size,
		modified: modified,
	}, nil
}

// deleteObject removes one object.
func (c *client) deleteObject(key string) error {
	req, err := c.request(http.MethodDelete, key, nil, nil, "")
	if err != nil {
		return err
	}
	_, _, err = c.do(req)
	return err
}

// copyObject copies one object server-side.
func (c *client) copyObject(srcKey, dstKey string) error {
	req, err := c.request(http.MethodPut, dstKey, nil, nil, "")
	if err != nil {
		return err
	}
	// The source is identified in a header, not the body.
	req.Header.Set("X-Amz-Copy-Source", encodeCopySource(c.bucket, srcKey))
	_, _, err = c.do(req)
	return err
}

// encodeCopySource formats the copy source header value.
func encodeCopySource(bucket, key string) string {
	return "/" + bucket + "/" + strings.TrimPrefix(key, "/")
}

// listEntry is one item returned by ListObjectsV2.
type listEntry struct {
	Key          string
	Size         int64
	LastModified time.Time
	// IsPrefix reports whether this is a pseudo-directory.
	IsPrefix bool
}

// listResult holds one page of a listing.
type listResult struct {
	Entries []listEntry
	// Truncated reports whether more pages exist.
	Truncated bool
	// NextToken is the continuation token for the next page.
	NextToken string
}

// listPrefix lists objects under prefix. When delimiter is "/", entries are
// grouped into pseudo-directories, which is how S3 represents folders.
func (c *client) listPrefix(prefix, delimiter, token string) (*listResult, error) {
	query := url.Values{}
	query.Set("list-type", "2")
	if prefix != "" {
		query.Set("prefix", prefix)
	}
	if delimiter != "" {
		query.Set("delimiter", delimiter)
	}
	if token != "" {
		query.Set("continuation-token", token)
	}
	query.Set("max-keys", "1000")

	req, err := c.request(http.MethodGet, "", query, nil, "")
	if err != nil {
		return nil, err
	}
	_, body, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		IsTruncated           bool   `xml:"IsTruncated"`
		NextContinuationToken string `xml:"NextContinuationToken"`
		Contents              []struct {
			Key          string `xml:"Key"`
			Size         int64  `xml:"Size"`
			LastModified string `xml:"LastModified"`
		} `xml:"Contents"`
		CommonPrefixes []struct {
			Prefix string `xml:"Prefix"`
		} `xml:"CommonPrefixes"`
	}
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("s3: parse listing: %w", err)
	}

	result := &listResult{
		Truncated: parsed.IsTruncated,
		NextToken: parsed.NextContinuationToken,
	}
	for _, p := range parsed.CommonPrefixes {
		result.Entries = append(result.Entries, listEntry{Key: p.Prefix, IsPrefix: true})
	}
	for _, obj := range parsed.Contents {
		// Some S3-compatible services return folder placeholder objects in
		// Contents even with a delimiter. A key ending in "/" is a directory.
		if strings.HasSuffix(obj.Key, "/") {
			result.Entries = append(result.Entries, listEntry{Key: obj.Key, IsPrefix: true})
			continue
		}
		modified, _ := time.Parse(time.RFC3339, obj.LastModified)
		result.Entries = append(result.Entries, listEntry{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: modified,
		})
	}
	return result, nil
}

// objectMeta is the metadata of one object.
type objectMeta struct {
	key      string
	size     int64
	modified time.Time
}
