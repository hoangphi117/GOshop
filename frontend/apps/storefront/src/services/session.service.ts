import { authFetch } from '@/lib/api'
import { UserSession } from '@/types/session'

export const sessionService = {
  async getSessions(): Promise<UserSession[]> {
    const res = await authFetch('/api/v1/users/sessions', {
      method: 'GET',
    })

    const data = await res.json().catch(() => null)
    if (!res.ok) {
      throw new Error(data?.message || 'Failed to load active sessions')
    }

    return (data?.data || []) as UserSession[]
  },

  async revokeSession(sessionUUID: string): Promise<void> {
    const res = await authFetch(`/api/v1/users/sessions/${sessionUUID}`, {
      method: 'DELETE',
    })

    const data = await res.json().catch(() => null)
    if (!res.ok) {
      throw new Error(data?.message || 'Failed to revoke session')
    }
  },

  async revokeAllOtherSessions(): Promise<void> {
    const res = await authFetch('/api/v1/users/sessions', {
      method: 'DELETE',
    })

    const data = await res.json().catch(() => null)
    if (!res.ok) {
      throw new Error(data?.message || 'Failed to revoke other sessions')
    }
  },
}
