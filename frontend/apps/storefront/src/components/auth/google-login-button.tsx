'use client'

import React, { useEffect, useRef, useState, useCallback } from 'react'
import Script from 'next/script'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/auth-store'
import { API_BASE_URL, API_KEY } from '@/lib/api'

interface GoogleLoginButtonProps {
  text?: 'signin_with' | 'signup_with' | 'continue_with'
  onSuccess?: () => void
  onError?: (errorMsg: string) => void
}

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: {
            client_id: string
            callback: (response: { credential: string }) => void
            auto_select?: boolean
            cancel_on_tap_outside?: boolean
          }) => void
          renderButton: (
            parent: HTMLElement,
            options: {
              type?: 'standard' | 'icon'
              theme?: 'outline' | 'filled_blue' | 'filled_black'
              size?: 'large' | 'medium' | 'small'
              text?: 'signin_with' | 'signup_with' | 'continue_with'
              shape?: 'rectangular' | 'pill' | 'circle' | 'square'
              logo_alignment?: 'left' | 'center'
              width?: number | string
            }
          ) => void
          prompt: () => void
        }
      }
    }
  }
}

export function GoogleLoginButton({
  text = 'continue_with',
  onSuccess,
  onError,
}: GoogleLoginButtonProps) {
  const router = useRouter()
  const { setTokens, setUser } = useAuthStore()
  const buttonRef = useRef<HTMLDivElement>(null)
  const [loading, setLoading] = useState(false)

  const clientId = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID

  const handleCredentialResponse = useCallback(
    async (response: { credential: string }) => {
      const idToken = response.credential
      if (!idToken) {
        if (onError) onError('Did not receive Google credentials. Please try again.')
        return
      }

      setLoading(true)
      try {
        const res = await fetch(`${API_BASE_URL}/api/v1/auth/google-login`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-API-KEY': API_KEY,
          },
          credentials: 'include',
          body: JSON.stringify({ id_token: idToken }),
        })

        const data = await res.json()

        if (!res.ok) {
          throw new Error(data?.message || 'Google login failed. Please verify your credentials.')
        }

        const accessToken = data.data?.access_token || data.data?.accessToken || ''
        const refreshToken = data.data?.refresh_token || data.data?.refreshToken || ''
        setTokens(accessToken, refreshToken)

        // Fetch user profile
        try {
          const profileRes = await fetch(`${API_BASE_URL}/api/v1/users/me`, {
            headers: {
              Authorization: `Bearer ${accessToken}`,
              'X-API-KEY': API_KEY,
            },
          })
          if (profileRes.ok) {
            const profileData = await profileRes.json()
            const p = profileData.data
            setUser({
              uuid: p.uuid || p.user_uuid,
              email: p.email || p.user_email,
              fullName: p.full_name || p.user_fullname,
              role: p.roles || (p.user_level === 1 ? ['admin'] : ['member']),
            })
          }
        } catch (profileErr) {
          console.warn('Failed to fetch user profile after Google login:', profileErr)
        }

        if (onSuccess) {
          onSuccess()
        } else {
          const redirectUrl =
            typeof window !== 'undefined'
              ? new URLSearchParams(window.location.search).get('redirect') || '/'
              : '/'
          router.push(redirectUrl)
        }
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : 'Google authentication failed'
        if (onError) {
          onError(message)
        }
      } finally {
        setLoading(false)
      }
    },
    [router, setTokens, setUser, onSuccess, onError]
  )

  const renderGoogleButton = useCallback(() => {
    if (!window.google?.accounts?.id || !buttonRef.current || !clientId) return

    try {
      window.google.accounts.id.initialize({
        client_id: clientId,
        callback: handleCredentialResponse,
      })

      buttonRef.current.innerHTML = ''
      window.google.accounts.id.renderButton(buttonRef.current, {
        type: 'standard',
        theme: 'outline',
        size: 'large',
        text,
        shape: 'pill',
        width: 320,
      })
    } catch (err) {
      console.error('Google button render error:', err)
    }
  }, [clientId, handleCredentialResponse, text])

  useEffect(() => {
    if (window.google?.accounts?.id) {
      renderGoogleButton()
      return
    }

    const interval = setInterval(() => {
      if (window.google?.accounts?.id) {
        renderGoogleButton()
        clearInterval(interval)
      }
    }, 150)

    const timer = setTimeout(() => clearInterval(interval), 5000)

    return () => {
      clearInterval(interval)
      clearTimeout(timer)
    }
  }, [renderGoogleButton])

  return (
    <div className="w-full flex flex-col items-center">
      <Script
        src="https://accounts.google.com/gsi/client?hl=en"
        strategy="afterInteractive"
        onLoad={renderGoogleButton}
      />

      {loading ? (
        <div className="w-full py-2.5 px-4 rounded-full border border-slate-200 bg-slate-50 flex items-center justify-center gap-2 text-xs font-semibold text-slate-600">
          <div className="w-4 h-4 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
          <span>Signing in with Google...</span>
        </div>
      ) : (
        <div ref={buttonRef} className="w-full flex justify-center min-h-[44px]" />
      )}
    </div>
  )
}
