/**
 * Direct-to-storage upload helper.
 *
 * Flow: POST /upload/presign → PUT the bytes straight to S3/MinIO/R2 → POST /upload/confirm.
 * The confirm response is the single source of truth for the URL: it HEADs the object so
 * the domain API never trusts a client-supplied path.
 *
 * Byte uploads intentionally bypass the api-client auth wrapper: the presigned URL is
 * pre-signed in the signature and any extra header (X-API-KEY, Authorization) breaks it.
 */

export type UploadFolder =
  | 'products'
  | 'categories'
  | 'banners'
  | 'avatars'
  | 'reviews'
  | 'returns'
  | 'common'
  | 'chat'

export interface PresignedUpload {
  upload_url: string
  public_url: string
  file_key: string
  expires_in: number
  content_type: string
  max_size: number
}

export interface ConfirmedUpload {
  file_key: string
  public_url: string
  content_type: string
  size: number
}

export interface DirectUploadResult {
  url: string
  name: string
  size: number
  fileKey: string
}

export class UploadError extends Error {
  constructor(
    message: string,
    readonly status?: number,
  ) {
    super(message)
    this.name = 'UploadError'
  }
}

const CONTENT_TYPE_BY_EXT: Record<string, string> = {
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  webp: 'image/webp',
  mp4: 'video/mp4',
  webm: 'video/webm',
  mov: 'video/quicktime',
}

export function contentTypeForFile(file: File): string {
  if (file.type) return file.type
  const ext = file.name.split('.').pop()?.toLowerCase() ?? ''
  return CONTENT_TYPE_BY_EXT[ext] ?? ''
}

/** Thin fetch adapter so each app can inject its auth-aware request function. */
export type AuthorizedFetch = (path: string, init?: RequestInit) => Promise<Response>

async function readError(res: Response, fallback: string): Promise<string> {
  const json = await res.json().catch(() => null)
  const payload = json as { error?: string; detail?: string; message?: string } | null
  return payload?.error || payload?.detail || payload?.message || fallback
}

export async function directUpload(
  file: File,
  folder: UploadFolder,
  authFetch: AuthorizedFetch,
): Promise<DirectUploadResult> {
  const contentType = contentTypeForFile(file)
  if (!contentType) {
    throw new UploadError(`Unsupported file type: ${file.name}`)
  }

  const presignRes = await authFetch('/api/v1/upload/presign', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      folder,
      filename: file.name,
      content_type: contentType,
    }),
  })
  if (!presignRes.ok) {
    throw new UploadError(await readError(presignRes, 'Could not start upload'), presignRes.status)
  }

  const presignBody = (await presignRes.json()) as { data?: PresignedUpload }
  const presign = presignBody.data
  if (!presign?.upload_url) {
    throw new UploadError('Upload service returned no presigned URL')
  }
  if (presign.max_size > 0 && file.size > presign.max_size) {
    const limitMb = Math.floor(presign.max_size / (1 << 20))
    throw new UploadError(`File is too large (max ${limitMb}MB for this type)`)
  }

  const putRes = await fetch(presign.upload_url, {
    method: 'PUT',
    // Content-Type must match the value signed at presign time exactly.
    headers: { 'Content-Type': presign.content_type || contentType },
    body: file,
  })
  if (!putRes.ok) {
    throw new UploadError('Upload to storage failed', putRes.status)
  }

  const confirmRes = await authFetch('/api/v1/upload/confirm', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ file_key: presign.file_key }),
  })
  if (!confirmRes.ok) {
    throw new UploadError(await readError(confirmRes, 'Upload could not be verified'), confirmRes.status)
  }

  const confirmBody = (await confirmRes.json()) as { data?: ConfirmedUpload }
  const confirmed = confirmBody.data
  if (!confirmed?.public_url) {
    throw new UploadError('Upload verification returned no URL')
  }

  return {
    url: confirmed.public_url,
    name: file.name,
    size: confirmed.size,
    fileKey: confirmed.file_key,
  }
}
