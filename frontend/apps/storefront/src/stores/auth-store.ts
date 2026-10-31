import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

export interface AuthUser {
  uuid?: string
  accountNo?: string
  email: string
  fullName?: string
  role?: string[]
  level?: string
  status?: string
}

interface AuthState {
  user: AuthUser | null
  accessToken: string
  refreshToken: string
  setUser: (user: AuthUser | null) => void
  setTokens: (accessToken: string, refreshToken?: string) => void
  reset: () => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: '',
      refreshToken: '',
      setUser: (user) => set({ user }),
      setTokens: (accessToken, refreshToken) =>
        set((state) => ({
          accessToken,
          refreshToken: refreshToken || state.refreshToken,
        })),
      reset: () =>
        set({
          user: null,
          accessToken: '',
          refreshToken: '',
        }),
    }),
    {
      name: 'goshop_storefront_auth',
      storage: createJSONStorage(() => (typeof window !== 'undefined' ? localStorage : {
        getItem: () => null,
        setItem: () => {},
        removeItem: () => {},
      })),
    }
  )
)
