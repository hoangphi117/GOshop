import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export interface NotificationSettingsState {
  soundEnabled: boolean
  desktopPushEnabled: boolean
  mutedTypes: string[]

  setSoundEnabled: (enabled: boolean) => void
  setDesktopPushEnabled: (enabled: boolean) => void
  toggleMutedType: (type: string) => void
  resetSettings: () => void
}

const DEFAULT_SETTINGS = {
  soundEnabled: true,
  desktopPushEnabled: false,
  mutedTypes: [] as string[],
}

export const useNotificationSettingsStore = create<NotificationSettingsState>()(
  persist(
    (set) => ({
      ...DEFAULT_SETTINGS,
      setSoundEnabled: (enabled) => set({ soundEnabled: enabled }),
      setDesktopPushEnabled: (enabled) => set({ desktopPushEnabled: enabled }),
      toggleMutedType: (type) =>
        set((state) => {
          const lower = type.toLowerCase()
          const exists = state.mutedTypes.includes(lower)
          return {
            mutedTypes: exists
              ? state.mutedTypes.filter((t) => t !== lower)
              : [...state.mutedTypes, lower],
          }
        }),
      resetSettings: () => set({ ...DEFAULT_SETTINGS }),
    }),
    {
      name: 'goshop_admin_notification_settings',
    }
  )
)
