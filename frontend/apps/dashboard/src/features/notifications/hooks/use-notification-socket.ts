import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { useNotificationSettingsStore } from '../stores/notification-settings-store'
import { notificationKeys } from './use-notifications'
import type { NotificationDTO } from '../types'

function playChimeSound() {
  try {
    const AudioCtx =
      window.AudioContext ||
      (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
    if (!AudioCtx) return
    const ctx = new AudioCtx()
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()

    osc.type = 'sine'
    // Crisp two-tone chime (D5 -> A5)
    osc.frequency.setValueAtTime(587.33, ctx.currentTime)
    osc.frequency.exponentialRampToValueAtTime(880, ctx.currentTime + 0.08)

    gain.gain.setValueAtTime(0.12, ctx.currentTime)
    gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.35)

    osc.connect(gain)
    gain.connect(ctx.destination)

    osc.start()
    osc.stop(ctx.currentTime + 0.35)
  } catch {
    // Suppress audio policy errors if user has not interacted yet
  }
}

export function useNotificationSocket() {
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.auth.accessToken)
  const isAuthenticated = Boolean(useAuthStore((state) => state.auth.user))

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const retryCountRef = useRef(0)

  useEffect(() => {
    if (!isAuthenticated || !accessToken) {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      return
    }

    let isCleanedUp = false

    const connect = () => {
      if (isCleanedUp) return

      try {
        const rawBaseUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
        const wsBaseUrl = rawBaseUrl.replace(/^http/, 'ws')
        const socketUrl = `${wsBaseUrl}/api/v1/ws?token=${encodeURIComponent(accessToken)}`

        const ws = new WebSocket(socketUrl)
        wsRef.current = ws

        ws.onopen = () => {
          retryCountRef.current = 0
        }

        ws.onmessage = (event) => {
          try {
            const payload = JSON.parse(event.data)
            if (payload?.type === 'notification' && payload?.data) {
              const notif = payload.data as NotificationDTO
              const notifType = (notif.type || '').toLowerCase()
              const { soundEnabled, desktopPushEnabled, mutedTypes } =
                useNotificationSettingsStore.getState()

              const isMuted = mutedTypes.some((t) => notifType.includes(t))

              if (!isMuted) {
                // Play audio chime if enabled
                if (soundEnabled) {
                  playChimeSound()
                }

                // Show in-app toast
                toast.info(notif.title || 'New Notification', {
                  description: notif.body,
                  duration: 5000,
                })

                // Show native OS desktop notification when window/tab is in background
                if (
                  desktopPushEnabled &&
                  document.visibilityState === 'hidden' &&
                  'Notification' in window &&
                  Notification.permission === 'granted'
                ) {
                  try {
                    const nativeNotif = new Notification(notif.title || 'GOshop Notification', {
                      body: notif.body || '',
                      icon: '/favicon.ico',
                      tag: notif.uuid || notif.notification_uuid,
                    })
                    nativeNotif.onclick = () => {
                      window.focus()
                      nativeNotif.close()
                    }
                  } catch {
                    // Safe fallback if browser blocks
                  }
                }
              }

              // Invalidate notifications queries so UI updates instantly
              queryClient.invalidateQueries({ queryKey: notificationKeys.all })

              // Real-time table invalidation based on notification category
              if (notifType.includes('order')) {
                queryClient.invalidateQueries({ queryKey: ['adminOrders'] })
                queryClient.invalidateQueries({ queryKey: ['orders'] })
                queryClient.invalidateQueries({ queryKey: ['orderDetails'] })
                queryClient.invalidateQueries({ queryKey: ['analyticsOverview'] })
                queryClient.invalidateQueries({ queryKey: ['analyticsRecentOrders'] })
              } else if (
                notifType.includes('stock') ||
                notifType.includes('inventory') ||
                notifType.includes('product')
              ) {
                queryClient.invalidateQueries({ queryKey: ['stockMovements'] })
                queryClient.invalidateQueries({ queryKey: ['products'] })
                queryClient.invalidateQueries({ queryKey: ['adminProducts'] })
                queryClient.invalidateQueries({ queryKey: ['analyticsLowStockAlerts'] })
              } else if (notifType.includes('review')) {
                queryClient.invalidateQueries({ queryKey: ['adminReviews'] })
                queryClient.invalidateQueries({ queryKey: ['adminReviewReports'] })
              } else if (notifType.includes('return')) {
                queryClient.invalidateQueries({ queryKey: ['adminReturns'] })
                queryClient.invalidateQueries({ queryKey: ['analyticsReturns'] })
              }
            } else if (payload?.type === 'chat.message') {
              // Real-time chat message invalidation
              queryClient.invalidateQueries({ queryKey: ['admin-chats'] })
            } else if (payload?.type === 'chat.read') {
              queryClient.invalidateQueries({ queryKey: ['admin-chats'] })
            }
          } catch {
            // Non-JSON or heartbeat frame
          }
        }

        ws.onerror = () => {
          // Handled via onclose
        }

        ws.onclose = () => {
          if (isCleanedUp) return

          // Exponential backoff reconnect: 1s, 2s, 4s, 8s, up to 30s
          const delay = Math.min(1000 * Math.pow(2, retryCountRef.current), 30000)
          retryCountRef.current += 1

          reconnectTimeoutRef.current = setTimeout(() => {
            connect()
          }, delay)
        }
      } catch {
        // Safe fallback on construction failure
      }
    }

    connect()

    return () => {
      isCleanedUp = true
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
        reconnectTimeoutRef.current = null
      }
      if (wsRef.current) {
        const socket = wsRef.current
        wsRef.current = null

        // Remove handlers so closing does not trigger errors or reconnection loops
        socket.onopen = null
        socket.onmessage = null
        socket.onerror = null
        socket.onclose = null

        if (socket.readyState === WebSocket.OPEN) {
          socket.close(1000, 'Normal closure')
        } else if (socket.readyState === WebSocket.CONNECTING) {
          // Avoid browser warning "WebSocket is closed before connection is established"
          socket.addEventListener(
            'open',
            () => {
              socket.close(1000, 'Normal closure')
            },
            { once: true }
          )
        }
      }
    }
  }, [isAuthenticated, accessToken, queryClient])
}
