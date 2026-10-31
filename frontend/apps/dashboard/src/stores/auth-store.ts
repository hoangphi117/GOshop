import { create } from 'zustand'
import { getCookie, setCookie, removeCookie } from '@/lib/cookies'

const ACCESS_TOKEN_KEY = 'goshop_access_token'
const REFRESH_TOKEN_KEY = 'goshop_refresh_token'
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
    refreshToken: string
    setAccessToken: (accessToken: string) => void
    setTokens: (accessToken: string, refreshToken?: string) => void
    resetAccessToken: () => void
    reset: () => void
  }
}

export const useAuthStore = create<AuthState>()((set) => {
  const cookieAccessToken = getCookie(ACCESS_TOKEN_KEY)
  const cookieRefreshToken = getCookie(REFRESH_TOKEN_KEY)
  const cookieUser = getCookie(USER_KEY)

  let initToken: string
  try {
    initToken = cookieAccessToken ? JSON.parse(cookieAccessToken) : ''
  } catch {
    initToken = cookieAccessToken || ''
  }

  let initRefreshToken: string
  try {
    initRefreshToken = cookieRefreshToken ? JSON.parse(cookieRefreshToken) : ''
  } catch {
    initRefreshToken = cookieRefreshToken || ''
  }

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
      accessToken: initToken,
      refreshToken: initRefreshToken,
      setAccessToken: (accessToken) => {
        setCookie(ACCESS_TOKEN_KEY, JSON.stringify(accessToken))
        set((state) => ({ ...state, auth: { ...state.auth, accessToken } }))
      },
      setTokens: (accessToken, refreshToken) => {
        setCookie(ACCESS_TOKEN_KEY, JSON.stringify(accessToken))
        if (refreshToken) {
          setCookie(REFRESH_TOKEN_KEY, JSON.stringify(refreshToken))
        }
        set((state) => ({
          ...state,
          auth: {
            ...state.auth,
            accessToken,
            refreshToken: refreshToken || state.auth.refreshToken,
          },
        }))
      },
      resetAccessToken: () => {
        removeCookie(ACCESS_TOKEN_KEY)
        set((state) => ({ ...state, auth: { ...state.auth, accessToken: '' } }))
      },
      reset: () => {
        removeCookie(ACCESS_TOKEN_KEY)
        removeCookie(REFRESH_TOKEN_KEY)
        removeCookie(USER_KEY)
        set((state) => ({
          ...state,
          auth: {
            ...state.auth,
            user: null,
            accessToken: '',
            refreshToken: '',
          },
        }))
      },
    },
  }
})
