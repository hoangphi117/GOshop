import { describe, expect, it } from 'vitest'
import { sanitizeRedirectTarget } from './auth-redirect'

describe('sanitizeRedirectTarget', () => {
  it('passes through a plain dashboard path', () => {
    expect(sanitizeRedirectTarget('/orders?page=1')).toBe('/orders?page=1')
  })

  it('unwraps nested sign-in redirects to the real destination', () => {
    expect(
      sanitizeRedirectTarget('/sign-in?redirect=%2Fsign-in%3Fredirect%3D%252Forders'),
    ).toBe('/orders')
  })

  it('drops sign-in loops with no inner target', () => {
    expect(sanitizeRedirectTarget('/sign-in?redirect=%2Fsign-in')).toBeUndefined()
  })

  it('rejects non-path values and strips the /admin basepath', () => {
    expect(sanitizeRedirectTarget('https://evil.test/phish')).toBeUndefined()
    expect(sanitizeRedirectTarget('/admin/orders')).toBe('/orders')
  })
})
