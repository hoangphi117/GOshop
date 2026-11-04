import { create } from 'zustand'
import { getCookie, setCookie, removeCookie } from '@/lib/cookies'
const USER_KEY = 'goshop_user'

export interface AuthUser {
  uuid?: string
  accountNo?: string
  email: string
  fullName?: string
  role?: string[]
  level?: string
  status?: string
  exp?: number
}

interface AuthState {
  auth: {
    user: AuthUser | null
    setUser: (user: AuthUser | null) => void
    accessToken: string
    setAccessToken: (accessToken: string) => void
    setTokens: (accessToken: string) => void
    resetAccessToken: () => void
    reset: () => void
  }
}

export const useAuthStore = create<AuthState>()((set) => {
  // Access token lives in memory only: an XSS can at most steal a token that
  // expires in minutes. It is rebuilt on load by tryRefreshSession() from the
  // HttpOnly refresh cookie. Only the display profile is persisted.
  const cookieUser = getCookie(USER_KEY)

  let initUser: AuthUser | null
  try {
    initUser = cookieUser ? JSON.parse(cookieUser) : null
  } catch {
    initUser = null
  }

  return {
    auth: {
      user: initUser,
      setUser: (user) => {
        if (user) {
          setCookie(USER_KEY, JSON.stringify(user))
        } else {
          removeCookie(USER_KEY)
        }
        set((state) => ({ ...state, auth: { ...state.auth, user } }))
      },
      accessToken: '',
      setAccessToken: (accessToken) => {
        set((state) => ({ ...state, auth: { ...state.auth, accessToken } }))
      },
      setTokens: (accessToken) => {
        set((state) => ({ ...state, auth: { ...state.auth, accessToken } }))
      },
      resetAccessToken: () => {
        set((state) => ({ ...state, auth: { ...state.auth, accessToken: '' } }))
      },
      reset: () => {
        removeCookie(USER_KEY)
        set((state) => ({
          ...state,
          auth: {
            ...state.auth,
            user: null,
            accessToken: '',
          },
        }))
      },
    },
  }
})
