const SIGN_IN_PATHS = new Set(['/sign-in', '/admin/sign-in'])

// Unwrap nested `?redirect=/sign-in?redirect=...` values down to the real
// destination. Every 401 handler re-wraps the current URL, so after a few
// expired-session hits the param nests exponentially; each redirect site must
// sanitize instead of trusting it raw.
export function sanitizeRedirectTarget(raw?: string | null): string | undefined {
  if (!raw) return undefined
  let current = raw.trim()
  if (!current) return undefined
  for (let i = 0; i < 10; i++) {
    let path: string
    let search: string
    let hash: string
    try {
      const url = new URL(current, 'http://local')
      // Absolute URLs to another origin are never valid redirect targets.
      const isAbsolute = /^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(current) && url.origin !== 'http://local'
      if (isAbsolute) return undefined
      path = url.pathname
      search = url.search
      hash = url.hash
    } catch {
      // Not URL-parseable: keep as a raw path only if it looks like one.
      return current.startsWith('/') ? current : undefined
    }
    if (path.length > 1 && path.endsWith('/')) path = path.slice(0, -1)
    // SPA lives under /admin basepath; sign-in navigate() wants the inner path.
    if (path === '/admin') path = '/'
    else if (path.startsWith('/admin/')) path = path.slice('/admin'.length)
    if (SIGN_IN_PATHS.has(path)) {
      try {
        const inner = new URL(current, 'http://local').searchParams.get('redirect')
        if (!inner) return undefined
        current = inner
        continue
      } catch {
        return undefined
      }
    }
    const target = `${path}${search}${hash}`
    return target.startsWith('/') ? target : `/${target}`
  }
  return undefined
}

export function isSignInPath(pathname?: string | null): boolean {
  if (!pathname) return false
  const clean = pathname.endsWith('/') && pathname.length > 1 ? pathname.slice(0, -1) : pathname
  return SIGN_IN_PATHS.has(clean) || clean.endsWith('/sign-in')
}
