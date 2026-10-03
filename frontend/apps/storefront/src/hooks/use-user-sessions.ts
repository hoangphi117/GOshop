'use client'

import { useState, useEffect, useCallback } from 'react'
import { UserSession } from '@/types/session'
import { sessionService } from '@/services/session.service'
import { toast } from 'sonner'

export function useUserSessions() {
  const [sessions, setSessions] = useState<UserSession[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [revokingId, setRevokingId] = useState<string | null>(null)
  const [isRevokingAll, setIsRevokingAll] = useState(false)

  const loadSessions = useCallback(async () => {
    try {
      setIsLoading(true)
      const data = await sessionService.getSessions()
      setSessions(data)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error loading sessions'
      toast.error(msg)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    loadSessions()
  }, [loadSessions])

  const revokeSession = async (sessionUUID: string) => {
    try {
      setRevokingId(sessionUUID)
      await sessionService.revokeSession(sessionUUID)
      setSessions((prev) => prev.filter((s) => s.id !== sessionUUID))
      toast.success('Device session logged out successfully')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to log out device'
      toast.error(msg)
    } finally {
      setRevokingId(null)
    }
  }

  const revokeAllOtherSessions = async () => {
    try {
      setIsRevokingAll(true)
      await sessionService.revokeAllOtherSessions()
      setSessions((prev) => prev.filter((s) => s.is_current))
      toast.success('Logged out of all other devices successfully')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to revoke other sessions'
      toast.error(msg)
    } finally {
      setIsRevokingAll(false)
    }
  }

  const otherSessionsCount = sessions.filter((s) => !s.is_current).length

  return {
    sessions,
    isLoading,
    revokingId,
    isRevokingAll,
    otherSessionsCount,
    reload: loadSessions,
    revokeSession,
    revokeAllOtherSessions,
  }
}
