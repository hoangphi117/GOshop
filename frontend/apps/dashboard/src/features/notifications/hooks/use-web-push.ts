import { useCallback, useEffect, useState } from 'react'
import { useNotificationSettingsStore } from '../stores/notification-settings-store'

export interface WebPushOptions {
  body?: string
  icon?: string
  badge?: string
  tag?: string
  data?: unknown
  onClick?: () => void
}

export function isNotificationSupported(): boolean {
  return typeof window !== 'undefined' && typeof Notification !== 'undefined'
}

export function createDesktopNotification(
  title: string,
  options?: WebPushOptions
): Notification | null {
  if (!isNotificationSupported() || Notification.permission !== 'granted') {
    return null
  }
  try {
    const notif = new Notification(title, {
      body: options?.body,
      icon: options?.icon || '/favicon.ico',
      badge: options?.badge,
      tag: options?.tag,
    })
    notif.onclick = () => {
      window.focus()
      options?.onClick?.()
      notif.close()
    }
    return notif
  } catch {
    return null
  }
}

export function useWebPush() {
  const isSupported = isNotificationSupported()
  const [permission, setPermission] = useState<NotificationPermission>(() => {
    if (typeof window !== 'undefined' && 'Notification' in window) {
      return Notification.permission
    }
    return 'default'
  })

  const desktopPushEnabled = useNotificationSettingsStore((s) => s.desktopPushEnabled)
  const setDesktopPushEnabled = useNotificationSettingsStore((s) => s.setDesktopPushEnabled)

  useEffect(() => {
    if (isSupported) {
      setPermission(Notification.permission)
    }
  }, [isSupported])

  const requestPermission = useCallback(async (): Promise<boolean> => {
    if (!isSupported) return false

    try {
      const result = await Notification.requestPermission()
      setPermission(result)
      const granted = result === 'granted'
      setDesktopPushEnabled(granted)
      return granted
    } catch {
      return false
    }
  }, [isSupported, setDesktopPushEnabled])

  const sendNotification = useCallback(
    (title: string, options?: WebPushOptions): Notification | null => {
      if (!isSupported || !desktopPushEnabled || Notification.permission !== 'granted') {
        return null
      }

      try {
        const notif = new Notification(title, {
          body: options?.body,
          icon: options?.icon || '/favicon.ico',
          badge: options?.badge,
          tag: options?.tag,
        })

        notif.onclick = () => {
          window.focus()
          options?.onClick?.()
          notif.close()
        }

        return notif
      } catch {
        return null
      }
    },
    [isSupported, desktopPushEnabled]
  )

  return {
    isSupported,
    permission,
    desktopPushEnabled,
    requestPermission,
    sendNotification,
  }
}
