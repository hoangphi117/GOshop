import { authFetch, API_BASE_URL } from '@/lib/api'

// Assistant client.
//
// Plain fetch through authFetch instead of the openapi-fetch client: the
// assistant routes are not in the generated OpenAPI schema yet, and openapi-fetch
// only wins when its types are actually inferred. authFetch already carries the
// API key, the bearer token for signed-in users, and silent refresh-on-401.
//
// ponytail: switch to `api.POST('/api/v1/assistant/chat', ...)` once the
// assistant routes land in the spec.

export interface AssistantProduct {
  uuid: string
  name: string
  slug: string
  price: number
  stock: number
  category_name?: string
  image_url?: string
}

export interface AssistantReply {
  sessionId: string
  answer: string
  products: AssistantProduct[]
  fallbackToHuman: boolean
}

export class AssistantError extends Error {
  constructor(
    message: string,
    readonly code: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'AssistantError'
  }
}

async function parse<T>(res: Response, fallback: string): Promise<T> {
  let body: { data?: T; error?: string; code?: string } | null = null
  try {
    body = await res.json()
  } catch {
    body = null
  }
  if (!res.ok || !body?.data) {
    throw new AssistantError(
      body?.error || fallback,
      body?.code || 'UNKNOWN',
      res.status,
    )
  }
  return body.data
}

/** Streams one question and returns the assembled answer. */
export async function askAssistant(input: {
  message: string
  sessionId?: string | null
  history?: { role: 'user' | 'assistant'; content: string }[]
}): Promise<AssistantReply> {
  const res = await authFetch(`${API_BASE_URL}/api/v1/assistant/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      message: input.message,
      session_id: input.sessionId || undefined,
      history: input.history?.length ? input.history : undefined,
    }),
  })

  const data = await parse<{
    session_id: string
    answer: string
    products: AssistantProduct[] | null
    fallback_to_human: boolean
  }>(res, 'The assistant is unavailable right now.')

  return {
    sessionId: data.session_id,
    answer: data.answer,
    products: data.products ?? [],
    fallbackToHuman: data.fallback_to_human,
  }
}

/** One SSE frame from GET /api/v1/assistant/stream. */
type StreamEvent =
  | { type: 'delta'; delta: string }
  | {
      type: 'done'
      session_id: string
      products?: AssistantProduct[]
      fallback_to_human?: boolean
    }
  | { type: 'error'; error: string; code: string }

/**
 * Sends one question and streams the answer through onDelta.
 *
 * Plain fetch + ReadableStream: EventSource cannot send the API key header and
 * would not carry the bearer token, so SSE is read by hand. Chat history is not
 * replayed here; the server keeps it in Redis under the session id.
 */export async function streamAssistant(
  input: { message: string; sessionId?: string | null },
  onDelta: (delta: string) => void,
  signal?: AbortSignal,
): Promise<AssistantReply> {
  const params = new URLSearchParams({ message: input.message })
  if (input.sessionId) params.set('session_id', input.sessionId)

  const res = await authFetch(
    `${API_BASE_URL}/api/v1/assistant/stream?${params}`,
    // ask the edge not to compress; a buffered gzip response defeats streaming
    { headers: { Accept: 'text/event-stream' }, signal },
  )

  if (!res.ok || !res.body) {
    // Pre-stream failures are still plain JSON (400/429/503).
    await parse<never>(res, 'The assistant is unavailable right now.')
    throw new AssistantError('The assistant is unavailable right now.', 'UNKNOWN', res.status)
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let answer = ''
  let done: Extract<StreamEvent, { type: 'done' }> | null = null

  for (;;) {
    const { value, done: finished } = await reader.read()
    if (finished) break
    buffer += decoder.decode(value, { stream: true })

    // Frames are separated by a blank line; keep the trailing partial frame.
    const frames = buffer.split('\n\n')
    buffer = frames.pop() ?? ''

    for (const frame of frames) {
      const line = frame.split('\n').find((l) => l.startsWith('data: '))
      if (!line) continue
      let ev: StreamEvent
      try {
        ev = JSON.parse(line.slice(6)) as StreamEvent
      } catch {
        continue
      }
      if (ev.type === 'delta') {
        answer += ev.delta
        onDelta(ev.delta)
      } else if (ev.type === 'done') {
        done = ev
      } else if (ev.type === 'error') {
        throw new AssistantError(ev.error, ev.code, res.status)
      }
    }
  }

  if (!done) {
    throw new AssistantError('The answer was cut off. Please try again.', 'STREAM_TRUNCATED', res.status)
  }
  return {
    sessionId: done.session_id,
    answer,
    products: done.products ?? [],
    fallbackToHuman: Boolean(done.fallback_to_human),
  }
}

/** Opens a server-side session so history survives a page reload. */
export async function createAssistantSession(): Promise<string> {  const res = await authFetch(`${API_BASE_URL}/api/v1/assistant/session`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
  })
  const data = await parse<{ session_id: string }>(
    res,
    'The assistant is unavailable right now.',
  )
  return data.session_id
}
