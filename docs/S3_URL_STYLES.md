# S3 URL Styles Configuration Guide

## Overview

The WhatsApp Gateway supports three S3 URL formatting styles to accommodate different CDN and storage configurations.

## URL Style Comparison

| Style | AWS S3 | Custom Endpoint (MinIO, R2, etc.) |
|-------|--------|-----------------------------------|
| `path` | `https://s3.region.amazonaws.com/bucket/key` | `https://endpoint/bucket/key` |
| `subdomain` | `https://bucket.s3.region.amazonaws.com/key` | `https://bucket.endpoint/key` |
| `auto` | `https://bucket.s3.region.amazonaws.com/key` | `https://endpoint/bucket/key` |

## Configuration

### Environment Variable

```bash
PLUGINS_WEBHOOK_S3_URL_STYLE=<style>
```

**Valid values:** `path`, `subdomain`, `auto`
**Default:** `path` (backward compatible)

## Usage Examples

### 1. Path-style URLs (Default)

**Use case:** MinIO, legacy S3, buckets with underscores

```bash
# Configuration
PLUGINS_WEBHOOK_S3_ENDPOINT=https://t3.storage.dev
PLUGINS_WEBHOOK_S3_BUCKET=shared-renr
PLUGINS_WEBHOOK_S3_URL_STYLE=path

# Generated URL
https://t3.storage.dev/shared-renr/whatsapp/8102/file.jpg
```

**URL Structure:**
```
protocol://endpoint/bucket/key
```

### 2. Subdomain-style URLs

**Use case:** CDN integration (CloudFlare R2, Bunny CDN, custom CDNs)

```bash
# Configuration
PLUGINS_WEBHOOK_S3_ENDPOINT=https://t3.storage.dev
PLUGINS_WEBHOOK_S3_BUCKET=shared-renr
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain

# Generated URL
https://shared-renr.t3.storage.dev/whatsapp/8102/file.jpg
```

**URL Structure:**
```
protocol://bucket.endpoint/key
```

**Important:** Bucket name must be DNS-compatible (see Bucket Naming Rules below).

### 3. Auto-detect

**Use case:** Automatic selection based on S3 provider

```bash
# Configuration
PLUGINS_WEBHOOK_S3_URL_STYLE=auto

# With custom endpoint (e.g., MinIO)
PLUGINS_WEBHOOK_S3_ENDPOINT=http://localhost:9000
# Uses path style: http://localhost:9000/bucket/key

# Without endpoint (AWS S3)
# Uses subdomain style: https://bucket.s3.region.amazonaws.com/key
```

**Logic:**
- Custom endpoint (MinIO, R2, etc.) → Path-style
- AWS S3 (no endpoint) → Subdomain-style

## Bucket Naming Rules for Subdomain Style

Subdomain-style URLs require DNS-compatible bucket names.

### Valid Bucket Names ✓

- `my-bucket` - lowercase, hyphen separator
- `shared-renr` - lowercase, hyphen separator
- `test123` - lowercase, alphanumeric
- `app-data-2024` - lowercase, hyphens, numbers

### Invalid Bucket Names ✗

- `my_bucket` - contains underscore (not DNS-safe)
- `My-Bucket` - contains uppercase letters
- `-mybucket` - starts with hyphen
- `mybucket-` - ends with hyphen
- `my..bucket` - consecutive dots
- `ab` - too short (< 3 characters)

**Warning:** The application will warn you if you configure subdomain style with an invalid bucket name:

```
⚠️  WARNING: Bucket 'my_bucket' contains characters invalid for subdomain URLs
⚠️  Subdomain-style URLs may not work. Consider using URL_STYLE=path
```

## CDN Integration Examples

### CloudFlare R2

```bash
PLUGINS_WEBHOOK_S3_ENDPOINT=https://r2.example.com
PLUGINS_WEBHOOK_S3_BUCKET=media
PLUGINS_WEBHOOK_S3_REGION=auto
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain

# Generated URL
https://media.r2.example.com/whatsapp/file.jpg
```

### Bunny CDN

```bash
PLUGINS_WEBHOOK_S3_ENDPOINT=https://storage.bunnycdn.com
PLUGINS_WEBHOOK_S3_BUCKET=my-storage
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain

# Generated URL
https://my-storage.storage.bunnycdn.com/whatsapp/file.jpg
```

### AWS S3 with CloudFront

```bash
# Standard AWS S3
PLUGINS_WEBHOOK_S3_REGION=us-east-1
PLUGINS_WEBHOOK_S3_BUCKET=my-media
PLUGINS_WEBHOOK_S3_URL_STYLE=auto

# Generated URL
https://my-media.s3.us-east-1.amazonaws.com/whatsapp/file.jpg

# Note: For CloudFront, you may need custom domain configuration
```

## Protocol Preservation

The implementation preserves the protocol (http/https) from your endpoint configuration:

```bash
# HTTP endpoint
PLUGINS_WEBHOOK_S3_ENDPOINT=http://localhost:9000
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain
# Generated: http://bucket.localhost:9000/key

# HTTPS endpoint
PLUGINS_WEBHOOK_S3_ENDPOINT=https://storage.example.com
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain
# Generated: https://bucket.storage.example.com/key
```

## Migration from Existing Setup

### No Configuration Change Required

If you don't set `PLUGINS_WEBHOOK_S3_URL_STYLE`, the default is `path`, which matches the current behavior.

**Current behavior:**
```bash
# No URL_STYLE set (or URL_STYLE=path)
https://endpoint/bucket/key
```

**This is maintained for backward compatibility.**

### Switching to Subdomain Style

1. **Check bucket name compatibility:**
   ```bash
   # Bucket name: shared-renr ✓ (DNS-compatible)
   # Bucket name: shared_renr ✗ (contains underscore)
   ```

2. **Update .env:**
   ```bash
   PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain
   ```

3. **Restart application:**
   ```bash
   # Stop current instance
   # Restart with new configuration
   ```

4. **Verify URL format:**
   - Send a test image message
   - Check webhook payload for new URL format

### Rollback

Simply remove the environment variable or set to `path`:

```bash
PLUGINS_WEBHOOK_S3_URL_STYLE=path
```

## Troubleshooting

### URLs Not Working with Subdomain Style

**Symptoms:**
- 404 Not Found errors
- DNS resolution failures
- Media not loading in webhook receivers

**Possible causes:**

1. **Bucket name not DNS-compatible**
   - Solution: Use `path` style or rename bucket

2. **DNS/CDN not configured for subdomain routing**
   - Solution: Configure wildcard DNS or CDN rules

3. **Bucket policy doesn't allow subdomain access**
   - Solution: Update bucket CORS/access policies

### Warning Message on Startup

```
⚠️  WARNING: Bucket 'my_bucket' contains characters invalid for subdomain URLs
```

**Solution:**
- Option 1: Rename bucket to be DNS-compatible (e.g., `my-bucket`)
- Option 2: Use `path` style instead: `PLUGINS_WEBHOOK_S3_URL_STYLE=path`

## Complete Configuration Example

```bash
# .env file

# S3 Configuration
PLUGINS_WEBHOOK_S3_ENABLED=true
PLUGINS_WEBHOOK_S3_REGION=us-east-1
PLUGINS_WEBHOOK_S3_BUCKET=whatsapp-media
PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Custom endpoint (MinIO, R2, etc.)
PLUGINS_WEBHOOK_S3_ENDPOINT=https://storage.example.com

# URL Style
PLUGINS_WEBHOOK_S3_URL_STYLE=subdomain

# Result: https://whatsapp-media.storage.example.com/whatsapp/device-id/file.jpg
```

## Decision Tree

```
Do you need CDN integration?
│
├─ Yes → Is your bucket name DNS-compatible?
│        │
│        ├─ Yes → Use "subdomain"
│        │
│        └─ No → Rename bucket OR use "path"
│
└─ No → Do you want automatic selection?
         │
         ├─ Yes → Use "auto"
         │
         └─ No → Use "path" (default)
```

## Performance

URL generation is a string formatting operation with minimal overhead:
- No additional network calls
- No database queries
- Negligible CPU impact

## Security

- Protocol (http/https) is preserved from endpoint configuration
- No automatic protocol downgrades
- Bucket name validation prevents common misconfigurations

## Supported by Both Plugins

The URL style configuration is shared and consistent across:
- Webhook Plugin (`src/plugins/webhook/`)
- Renr Plugin (`src/plugins/renr/`)

Both plugins read `PLUGINS_WEBHOOK_S3_URL_STYLE` and generate identical URL formats.

## Related Documentation

- [S3 URL Style Implementation](../S3_URL_STYLE_IMPLEMENTATION.md) - Technical implementation details
- [AWS S3 Virtual Hosting](https://docs.aws.amazon.com/AmazonS3/latest/userguide/VirtualHosting.html)
- [Bucket Naming Rules](https://docs.aws.amazon.com/AmazonS3/latest/userguide/bucketnamingrules.html)
