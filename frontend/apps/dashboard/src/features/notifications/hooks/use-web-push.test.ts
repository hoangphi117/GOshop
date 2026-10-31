import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  isNotificationSupported,
  createDesktopNotification,
} from './use-web-push'
import { useNotificationSettingsStore } from '../stores/notification-settings-store'

describe('useWebPush and Notification utilities', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useNotificationSettingsStore.setState({ desktopPushEnabled: true })
    if (typeof window === 'undefined') {
      // @ts-expect-error Mocking window in node
      globalThis.window = globalThis
    }
  })

  it('isNotificationSupported returns false when Notification is undefined', () => {
    const originalNotification = globalThis.Notification
    // @ts-expect-error Mocking missing global
    delete globalThis.Notification

    expect(isNotificationSupported()).toBe(false)

    globalThis.Notification = originalNotification
  })

  it('createDesktopNotification returns null when permission is not granted', () => {
    const mockNotification = vi.fn()
    // @ts-expect-error Mocking Notification constructor
    mockNotification.permission = 'denied'
    globalThis.Notification = mockNotification as unknown as typeof Notification

    const notif = createDesktopNotification('Test title', { body: 'Test body' })
    expect(notif).toBeNull()
  })

  it('createDesktopNotification instantiates Notification when permission is granted', () => {
    const mockInstance = {
      title: '',
      options: undefined as unknown,
      onclick: null as (() => void) | null,
      close: vi.fn(),
    }

    class MockNotification {
      static permission: NotificationPermission = 'granted'
      constructor(title: string, options: unknown) {
        mockInstance.title = title
        mockInstance.options = options
        return mockInstance
      }
    }

    globalThis.Notification = MockNotification as unknown as typeof Notification

    const onClickSpy = vi.fn()
    const notif = createDesktopNotification('New Order', {
      body: 'Order #1001 placed',
      onClick: onClickSpy,
    })

    expect(notif).toBe(mockInstance)
    expect(mockInstance.title).toBe('New Order')
    expect(mockInstance.options).toEqual({
      body: 'Order #1001 placed',
      icon: '/favicon.ico',
      badge: undefined,
      tag: undefined,
    })

    // Test onclick triggers window focus, callback, and closes notification
    window.focus = vi.fn()
    mockInstance.onclick?.()
    expect(window.focus).toHaveBeenCalled()
    expect(onClickSpy).toHaveBeenCalled()
    expect(mockInstance.close).toHaveBeenCalled()
  })
})
