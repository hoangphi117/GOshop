import { NextResponse, type NextRequest } from 'next/server'

/**
 * Gives every visitor a guest identity before the first page renders.
 *
 * The recommendations API mints `goshop_anon_id` on demand, but it is HttpOnly
 * and issued on an API domain, so the first SSR pass never sees it — the home
 * shelf therefore rendered as the best-seller fallback and only the client
 * refetch could personalize it. Minting the cookie here makes it readable from
 * `cookies()` during that same render, so the personalized shelf is in the
 * first HTML byte and survives a reload.
 *
 * The API still accepts any value the browser sends (it takes the cookie as-is
 * and validates it as a UUID), so this does not need a shared HMAC secret.
 */
const ANON_COOKIE = 'goshop_anon_id'
const ANON_MAX_AGE = 365 * 24 * 60 * 60
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function middleware(request: NextRequest) {
  const existing = request.cookies.get(ANON_COOKIE)?.value
  if (existing && UUID_RE.test(existing)) return NextResponse.next()

  const id = crypto.randomUUID()
  // rsc=1 is a client-side navigation: the payload is not HTML, so a cookie set
  // on it is ignored. Mirror the value as a request cookie so the render reads
  // it now, and let the next full document load persist it.
  const requestHeaders = new Headers(request.headers)
  requestHeaders.set('cookie', mergeCookie(request.headers.get('cookie'), id))
  request.cookies.set(ANON_COOKIE, id)

  const response = NextResponse.next({ request: { headers: requestHeaders } })
  if (!request.headers.get('rsc')) {
    response.cookies.set(ANON_COOKIE, id, {
      path: '/',
      maxAge: ANON_MAX_AGE,
      httpOnly: true,
      sameSite: 'lax',
      // localhost is treated as a secure context, so http dev still accepts it.
      secure: process.env.NODE_ENV === 'production',
    })
  }
  return response
}

function mergeCookie(header: string | null, id: string): string {
  const pair = `${ANON_COOKIE}=${id}`
  if (!header) return pair
  const rest = header
    .split(';')
    .map((c) => c.trim())
    .filter((c) => c && !c.startsWith(`${ANON_COOKIE}=`))
  return [pair, ...rest].join('; ')
}

export const config = {
  // API routes and static assets never personalize a page.
  matcher: ['/((?!api|_next/static|_next/image|favicon.ico|.*\\.(?:png|jpg|jpeg|svg|webp|ico)$).*)'],
}
