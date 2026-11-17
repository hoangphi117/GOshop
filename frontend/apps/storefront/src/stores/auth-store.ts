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
  /** Edge-relative media path or an external provider URL; empty when unset. */
  avatarUrl?: string
}

interface AuthState {
  user: AuthUser | null
  // Access token lives in memory only: it dies on reload and is restored by
  // tryRefreshSession() from the HttpOnly refresh cookie. An XSS can at most
  // steal a token that expires in minutes, never one persisted on disk.
  accessToken: string
  setUser: (user: AuthUser | null) => void
  setTokens: (accessToken: string) => void
  reset: () => void
}

// Only the display profile is persisted; tokens never touch storage.
export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      accessToken: '',
      setUser: (user) => set({ user }),
      setTokens: (accessToken) => set({ accessToken }),
      reset: () =>
        set({
          user: null,
          accessToken: '',
        }),
    }),
    {
      name: 'goshop_storefront_auth',
      storage: createJSONStorage(() => (typeof window !== 'undefined' ? localStorage : {
        getItem: () => null,
        setItem: () => {},
        removeItem: () => {},
      })),
      partialize: (state) => ({ user: state.user }) as AuthState,
    }
  )
)
