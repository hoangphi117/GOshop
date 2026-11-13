# ADR 0009 — Cloudflare R2 in production, MinIO in development

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** media service, storage config, compose
- **Code:** `backend/gomod/shared/pkg/storage`, `backend/.env.example`, `docker-compose.prod.yml`

## Context

Product media (product images, review photos, chat attachments) must be reachable by every
service and uploaded by browsers directly. Keeping a storage container in production means
running, backing up and paying for a stateful service whose only job is to hold bytes — plus
egress bills as soon as images start being served.

## Decision

Uploads and reads use **presigned URLs against an S3-compatible bucket**, behind one driver
switch (`STORAGE_DRIVER`, `S3_*` env vars, `pkg/storage`):

- **Production:** Cloudflare R2. `STORAGE_DRIVER=r2`, `S3_USE_PATH_STYLE=false`, public reads
  via `S3_PUBLIC_BASE_URL=https://pub-<id>.r2.dev`. **No media container is started** — the
  `minio` service in `docker-compose.prod.yml` sits behind `profiles: ["minio"]` and never
  runs unless explicitly requested.
- **Development:** MinIO, started with the same profile. Path style on, plain HTTP, throwaway
  credentials, no network dependency on the internet.

The code path is identical in both: `PresignPut` / `PresignGet`, never a disk write.

## Consequences

**Positive**

- No egress bill for image traffic and no stateful container to operate in production.
- Production cannot accidentally depend on a local disk: the driver refuses `local`.
- Developers work offline; `dev-host` boots without internet storage.

**Trade-offs**

- Two configurations to keep honest; `.env.prod.example` must be updated whenever an `S3_*`
  knob is added.
- Uploads are only as safe as the bucket's public-read setting: a misconfigured
  `S3_PUBLIC_BASE_URL` silently breaks every image.
- Browser uploads need CORS configured on the bucket, not in nginx.

## Rejected alternatives

- **Run MinIO in production** — operational and backup cost for a feature R2 already provides,
  plus egress.
- **Store media on the local filesystem of the media service** — dies with the container and
  cannot be shared by replicas; already removed from the codebase.
- **AWS S3 directly** — supported as a driver (`STORAGE_DRIVER=s3`) but R2 is cheaper for a
  read-heavy, egress-dominated workload.
- **Proxy every image through the edge** — turns the edge into a bandwidth bottleneck and
  defeats browser caching.

## Related

- `system/r2/README.md` — bucket setup
- `backend/.env.example` §6 — the three driver options, side by side
