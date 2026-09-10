'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/auth-store'
import { Store, Lock, Mail, ArrowRight, ShieldCheck } from 'lucide-react'
import { toast } from 'sonner'

export default function LoginPage() {
  const router = useRouter()
  const { setTokens, setUser } = useAuthStore()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)

    try {
      const res = await fetch('http://localhost:8080/api/v1/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-KEY': 'abcd-1234-1111',
        },
        body: JSON.stringify({ email, password }),
      })

      const data = await res.json()

      if (!res.ok) {
        throw new Error(data?.message || 'Login failed. Please verify your credentials.')
      }

      // Save tokens & profile
      const accessToken = data.data.accessToken || data.data.access_token || ''
      const refreshToken = data.data.refreshToken || data.data.refresh_token || ''
      setTokens(accessToken, refreshToken)

      setUser({
        uuid: data.data.user?.uuid,
        email: data.data.user?.email || email,
        fullName: data.data.user?.full_name || data.data.user?.fullName || '',
        role: data.data.user?.roles || [],
      })

      toast.success('Successfully logged in!')
      const redirectUrl =
        typeof window !== 'undefined'
          ? new URLSearchParams(window.location.search).get('redirect') || '/'
          : '/'
      router.push(redirectUrl)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred during sign in.'
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-[80vh] flex items-center justify-center py-16 px-4 sm:px-6 bg-[#FAFBFC]">
      <div className="max-w-md w-full space-y-6">
        {/* Brand Header */}
        <div className="text-center space-y-2">
          <Link href="/" className="inline-flex items-center gap-2 mb-2">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-[#0D47A1] to-[#2196F3] flex items-center justify-center text-white shadow-md">
              <Store className="w-6 h-6" />
            </div>
            <span className="text-2xl font-black tracking-tight text-[#0D1B2A]">
              GO<span className="text-[#2196F3]">shop</span>
            </span>
          </Link>
          <h1 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
            Welcome Back
          </h1>
          <p className="text-xs text-slate-500">
            Sign in to track orders, manage addresses, and enjoy faster checkout.
          </p>
        </div>

        {/* Form Card */}
        <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-lg space-y-5">
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                Email Address
              </label>
              <div className="relative">
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="name@example.com"
                  className="w-full pl-9 pr-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                />
                <Mail className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              </div>
            </div>

            <div className="space-y-1.5">
              <div className="flex justify-between items-center">
                <label className="text-xs font-semibold text-slate-700">
                  Password
                </label>
                <Link
                  href="/forgot-password"
                  className="text-[11px] font-semibold text-[#2196F3] hover:underline"
                >
                  Forgot?
                </Link>
              </div>
              <div className="relative">
                <input
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  className="w-full pl-9 pr-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                />
                <Lock className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full py-3 px-4 rounded-xl bg-[#2196F3] text-white text-xs font-bold flex items-center justify-center gap-2 hover:bg-[#0D47A1] transition-all shadow-md disabled:bg-slate-300"
            >
              {loading ? (
                <div className="w-4 h-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
              ) : (
                <>
                  <span>Sign In</span>
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>
          </form>

          <div className="pt-2 text-center text-xs text-slate-500">
            Don&apos;t have an account?{' '}
            <Link
              href="/register"
              className="font-bold text-[#2196F3] hover:underline"
            >
              Register here
            </Link>
          </div>
        </div>

        {/* Security badge */}
        <div className="flex items-center justify-center gap-2 text-xs text-slate-400">
          <ShieldCheck className="w-4 h-4 text-emerald-500" />
          <span>Secure authentication protected by JWT & SSL</span>
        </div>
      </div>
    </div>
  )
}
