# R2 CORS

The browser uploads straight to R2 with a presigned PUT. `presignedEndpointFor`
(backend/pkg/storage/s3_storage.go) signs against the browser-facing host, so the
request never passes the edge — R2 itself must answer the CORS preflight or the
upload dies with "No 'Access-Control-Allow-Origin' header is present".

`r2-cors.json` is the policy the bucket needs.

## Apply

Cloudflare dash → R2 → bucket `goshop-media` → Settings → CORS policy → paste
the contents of `r2-cors.json`.

CLI alternative (needs an R2 API token with Object Read & Write):

```powershell
# aws CLI with R2 as an S3 endpoint
$env:AWS_ACCESS_KEY_ID="<r2 access key>"
$env:AWS_SECRET_ACCESS_KEY="<r2 secret>"
aws s3api put-bucket-cors --bucket goshop-media `
  --cors-configuration file://system/r2/r2-cors.json `
  --endpoint-url https://<CF_ACCOUNT_ID>.r2.cloudflarestorage.com
```

## Why these values

| Field | Value | Reason |
|---|---|---|
| AllowedOrigins | localhost / 127.0.0.1 on :80, :3000, :3001 | dev edge + storefront dev server + dashboard dev server |
| AllowedMethods | GET, PUT | presigned upload is PUT; images/banners are GET |
| AllowedHeaders | content-type | the only header the presigned PUT signs |
| ExposeHeaders | ETag | the upload hook reads it back |
| MaxAgeSeconds | 3000 | preflight cached ~50 min, fewer round trips |

## When the domain exists

Add the real origin (e.g. `https://shop.example.com`) to `AllowedOrigins` and
re-apply. Drop the localhost entries once dev no longer needs them.
