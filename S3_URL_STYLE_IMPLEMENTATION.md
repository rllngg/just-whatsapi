# S3 URL Style Implementation

## Overview

This document describes the implementation of custom S3 URL formatting support for the WhatsApp Gateway application. Users can now configure the URL style for S3-uploaded media files to support both path-style and subdomain-style URLs.

## Motivation

CDN configurations (CloudFlare R2, Bunny CDN, custom CDNs) often require subdomain-style URLs instead of path-style URLs for optimal performance and caching.

**Before:**
```
https://t3.storage.dev/shared-renr/whatsapp/8102/file.jpg
```

**After (with subdomain style):**
```
https://shared-renr.t3.storage.dev/whatsapp/8102/file.jpg
```

## Environment Variable

### `PLUGINS_WEBHOOK_S3_URL_STYLE`

Configures the URL format style for S3-uploaded media files.

**Valid values:**
- `path` (default) - Path-style URLs for backward compatibility
- `subdomain` - Subdomain-style URLs for CDN integration
- `auto` - Auto-detect (subdomain for AWS S3, path for custom endpoints)

**Default:** `path` (if not set)

### Usage Examples

#### Path-style (Default/Current Behavior)
```bash
PLUGINS_WEBHOOK_S3_ENDPOINT=https://t3.storage.dev
PLUGINS_WEBHOOK_S3_BUCKET=shared-renr
PLUGINS_WEBHOOK_S3_URL_STYLE=path

# Generates: https://t3.storage.dev/shared-renr/whatsapp/file.jpg
```

#### Subdomain-style (CDN-friendly)
```bash
PLUGINS_WEBHOOK_S3_ENDPOINT=https://t3.storage.dev
PLUGINS_WEBHOOK_S3_BUCKET=shared-renr
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain

# Generates: https://shared-renr.t3.storage.dev/whatsapp/file.jpg
```

#### Auto-detect
```bash
PLUGINS_WEBHOOK_S3_URL_STYLE=auto

# With custom endpoint: uses path style
# Without custom endpoint (AWS S3): uses subdomain style
```

## Implementation Details

### Files Modified

1. **`src/plugins/webhook/config.go`**
   - Added `URLStyle string` field to `S3Config` struct
   - Implemented configuration loading with default value
   - Added validation for valid URL style values
   - Added `containsInvalidDNSChars()` helper for bucket name validation
   - Warns users if subdomain style is used with DNS-incompatible bucket names

2. **`src/plugins/webhook/s3.go`**
   - Updated `NewS3Client()` to use dynamic `UsePathStyle` based on `URLStyle`
   - Implemented `shouldUsePathStyle()` helper function
   - Implemented `buildCustomEndpointURL()` for dynamic URL generation
   - Updated `UploadFile()` to generate URLs based on configured style

3. **`src/plugins/renr/media.go`**
   - Updated `uploadToS3()` to read `URLStyle` from environment
   - Implemented `shouldUsePathStyleV1()` helper for AWS SDK v1
   - Implemented `buildCustomEndpointURLV1()` for dynamic URL generation
   - Updated URL generation logic to support all three styles

4. **`.env.example`**
   - Added comprehensive documentation for `PLUGINS_WEBHOOK_S3_URL_STYLE`
   - Included usage examples and warnings

### Architecture Decisions

#### 1. Backward Compatibility
- Default value is `path` to maintain current behavior
- Existing deployments work without configuration changes
- No breaking changes to existing functionality

#### 2. Consistency Across Plugins
Both webhook and Renr plugins share the same environment variable (`PLUGINS_WEBHOOK_S3_URL_STYLE`) and generate identical URL formats.

#### 3. Protocol Preservation
The implementation preserves the protocol (http:// vs https://) from the endpoint configuration.

#### 4. DNS Validation
The system warns users if subdomain style is configured with DNS-incompatible bucket names:
- Contains underscores (`_`)
- Contains consecutive dots (`..`)
- Starts or ends with hyphen (`-`)
- Contains uppercase letters
- Length < 3 or > 63 characters

### Helper Functions

#### `shouldUsePathStyle(cfg S3Config) bool` (webhook plugin)
Determines whether AWS SDK v2 should use path-style URLs.

**Logic:**
- `subdomain` → returns `false` (use subdomain style)
- `path` → returns `true` (use path style)
- `auto` → returns `true` if custom endpoint, `false` for AWS S3
- default → returns `true` (fallback to path style)

#### `buildCustomEndpointURL(endpoint, bucket, key, urlStyle string) string`
Generates S3 URLs for custom endpoints based on URL style.

**Logic:**
1. Extracts and preserves protocol (http/https)
2. Removes trailing slashes from endpoint
3. Generates URL based on style:
   - `subdomain`: `protocol://bucket.endpoint/key`
   - `path`: `endpoint/bucket/key`
   - `auto`: `endpoint/bucket/key` (path for custom endpoints)

#### `containsInvalidDNSChars(bucket string) bool`
Validates bucket name for DNS/subdomain compatibility.

**Checks:**
- No underscores
- No consecutive dots
- No leading/trailing hyphens
- Lowercase only
- Length between 3-63 characters

### SDK Differences

#### Webhook Plugin (AWS SDK v2)
Uses `s3.Options.UsePathStyle` boolean flag.

```go
s3Options := func(o *s3.Options) {
    if cfg.Endpoint != "" {
        o.BaseEndpoint = aws.String(cfg.Endpoint)
        o.UsePathStyle = shouldUsePathStyle(cfg)
    }
}
```

#### Renr Plugin (AWS SDK v1)
Uses `aws.Config.S3ForcePathStyle` pointer to boolean.

```go
if endpoint != "" {
    awsConfig.Endpoint = aws.String(endpoint)
    awsConfig.S3ForcePathStyle = aws.Bool(shouldUsePathStyleV1(urlStyle, endpoint))
}
```

## Testing

### Build Verification
```bash
CGO_ENABLED=1 go build -o whatsapp-gateway .
# Build succeeds without errors
```

### Manual Testing Scenarios

#### Test 1: Path-style (Backward Compatibility)
```bash
# .env configuration
PLUGINS_WEBHOOK_S3_ENABLED=true
PLUGINS_WEBHOOK_S3_ENDPOINT=http://localhost:9000
PLUGINS_WEBHOOK_S3_BUCKET=test-bucket
PLUGINS_WEBHOOK_S3_URL_STYLE=path

# Expected URL: http://localhost:9000/test-bucket/whatsapp/file.jpg
```

#### Test 2: Subdomain-style
```bash
# .env configuration
PLUGINS_WEBHOOK_S3_ENABLED=true
PLUGINS_WEBHOOK_S3_ENDPOINT=https://t3.storage.dev
PLUGINS_WEBHOOK_S3_BUCKET=shared-renr
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain

# Expected URL: https://shared-renr.t3.storage.dev/whatsapp/file.jpg
```

#### Test 3: Auto-detect with Custom Endpoint
```bash
# .env configuration
PLUGINS_WEBHOOK_S3_ENABLED=true
PLUGINS_WEBHOOK_S3_ENDPOINT=http://localhost:9000
PLUGINS_WEBHOOK_S3_BUCKET=test-bucket
PLUGINS_WEBHOOK_S3_URL_STYLE=auto

# Expected URL: http://localhost:9000/test-bucket/whatsapp/file.jpg (path style)
```

#### Test 4: Auto-detect with AWS S3
```bash
# .env configuration
PLUGINS_WEBHOOK_S3_ENABLED=true
PLUGINS_WEBHOOK_S3_REGION=us-east-1
PLUGINS_WEBHOOK_S3_BUCKET=my-bucket
PLUGINS_WEBHOOK_S3_URL_STYLE=auto
# No ENDPOINT set

# Expected URL: https://my-bucket.s3.us-east-1.amazonaws.com/whatsapp/file.jpg (subdomain style)
```

#### Test 5: Invalid Bucket Name Warning
```bash
# .env configuration
PLUGINS_WEBHOOK_S3_BUCKET=shared_renr  # Contains underscore
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain

# Expected: Warning message on startup
# ⚠️  WARNING: Bucket 'shared_renr' contains characters invalid for subdomain URLs
# ⚠️  Subdomain-style URLs may not work. Consider using URL_STYLE=path
```

### Integration Test with MinIO

Using the existing `test/docker-compose.yml` setup:

```bash
# Start MinIO
cd test && docker-compose up -d

# Configure .env
PLUGINS_WEBHOOK_S3_ENABLED=true
PLUGINS_WEBHOOK_S3_ENDPOINT=http://localhost:9000
PLUGINS_WEBHOOK_S3_BUCKET=whatsapp
PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID=minioadmin
PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY=minioadmin
PLUGINS_WEBHOOK_S3_REGION=us-east-1
PLUGINS_WEBHOOK_S3_URL_STYLE=path  # or subdomain

# Run application and send image message
# Verify URL format in webhook payload
```

## Use Cases

### 1. CloudFlare R2 + CDN
```bash
PLUGINS_WEBHOOK_S3_ENDPOINT=https://r2.example.com
PLUGINS_WEBHOOK_S3_BUCKET=media
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain
# URLs: https://media.r2.example.com/whatsapp/file.jpg
```

### 2. Bunny CDN
```bash
PLUGINS_WEBHOOK_S3_ENDPOINT=https://storage.bunnycdn.com
PLUGINS_WEBHOOK_S3_BUCKET=my-storage
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain
# URLs: https://my-storage.storage.bunnycdn.com/whatsapp/file.jpg
```

### 3. MinIO (Local Development)
```bash
PLUGINS_WEBHOOK_S3_ENDPOINT=http://localhost:9000
PLUGINS_WEBHOOK_S3_BUCKET=dev-bucket
PLUGINS_WEBHOOK_S3_URL_STYLE=path
# URLs: http://localhost:9000/dev-bucket/whatsapp/file.jpg
```

### 4. AWS S3 (Traditional)
```bash
PLUGINS_WEBHOOK_S3_REGION=ap-southeast-1
PLUGINS_WEBHOOK_S3_BUCKET=my-whatsapp-media
PLUGINS_WEBHOOK_S3_URL_STYLE=auto
# URLs: https://my-whatsapp-media.s3.ap-southeast-1.amazonaws.com/whatsapp/file.jpg
```

## Error Handling

### Invalid URL Style Value
```bash
PLUGINS_WEBHOOK_S3_URL_STYLE=invalid
```
**Result:** Application fails to start with error:
```
Error: PLUGINS_WEBHOOK_S3_URL_STYLE must be one of: path, subdomain, auto (got: invalid)
```

### DNS-Incompatible Bucket Name
```bash
PLUGINS_WEBHOOK_S3_BUCKET=my_bucket_name
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain
```
**Result:** Warning on startup (application continues):
```
⚠️  WARNING: Bucket 'my_bucket_name' contains characters invalid for subdomain URLs
⚠️  Subdomain-style URLs may not work. Consider using URL_STYLE=path
```

## Migration Guide

### Existing Deployments

**No action required.** The default behavior (`path` style) matches the current implementation. Existing deployments will continue to work without any configuration changes.

### Enabling Subdomain Style

1. Verify bucket name is DNS-compatible (no underscores, lowercase only)
2. Add to `.env`:
   ```bash
   PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain
   ```
3. Restart application
4. Test URL generation by sending a media message

### Rollback

Simply remove `PLUGINS_WEBHOOK_S3_URL_STYLE` from `.env` or set to `path`:
```bash
PLUGINS_WEBHOOK_S3_URL_STYLE=path
```

## Performance Impact

**Minimal.** URL generation is a simple string formatting operation. The implementation adds:
- One environment variable read per plugin initialization
- One switch statement per URL generation
- No additional network calls or database queries

## Security Considerations

### Protocol Preservation
The implementation preserves the protocol (http/https) from the endpoint configuration, preventing unintended protocol downgrades.

### Bucket Name Validation
DNS validation warns users about potentially problematic bucket names but doesn't block them, allowing flexibility while promoting best practices.

### Backward Compatibility
Default `path` style ensures existing security configurations (bucket policies, CORS, etc.) remain effective.

## Future Enhancements

1. **Per-plugin URL style**: Allow different URL styles for webhook vs Renr plugins
2. **Custom URL templates**: Support user-defined URL format templates
3. **URL signing**: Support for signed/presigned URLs with expiration
4. **CloudFront/CDN URL rewriting**: Automatic CDN domain substitution

## Related Documentation

- AWS S3 Path-style vs Virtual-hosted-style: https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html
- CloudFlare R2 URL formats: https://developers.cloudflare.com/r2/
- Bucket naming rules: https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html

## Summary

The S3 URL style implementation provides:
- ✅ Backward compatibility (default `path` style)
- ✅ CDN-friendly subdomain URLs
- ✅ Auto-detection for AWS vs custom endpoints
- ✅ Validation and warnings for DNS compatibility
- ✅ Consistent behavior across webhook and Renr plugins
- ✅ Protocol preservation (http/https)
- ✅ Clear documentation and examples
- ✅ No breaking changes
