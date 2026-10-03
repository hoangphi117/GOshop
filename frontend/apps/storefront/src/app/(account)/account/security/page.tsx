'use client'

import React, { useState } from 'react'
import { useAuthStore } from '@/stores/auth-store'
import { Shield, Lock, CheckCircle2, KeyRound } from 'lucide-react'
import { toast } from 'sonner'
import { SessionDeviceManager } from '@/components/account/session-device-manager'

export default function SecurityPage() {
  const { accessToken } = useAuthStore()

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()

    if (newPassword.length < 6) {
      toast.error('New password must be at least 6 characters long.')
      return
    }

    if (newPassword !== confirmPassword) {
      toast.error('New passwords do not match.')
      return
    }

    setLoading(true)

    try {
      const res = await fetch('http://localhost:8080/api/v1/users/me/change-password', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${accessToken}`,
          'X-API-KEY': 'abcd-1234-1111',
        },
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      })

      const data = await res.json().catch(() => null)

      if (!res.ok) {
        throw new Error(data?.message || 'Failed to update password. Verify your current password.')
      }

      toast.success('Your password has been changed successfully!')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error changing password'
      toast.error(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
        <div>
          <h1 className="text-xl font-extrabold text-[#0D1B2A] tracking-tight">
            Password & Security
          </h1>
          <p className="text-xs text-slate-500 mt-1">
            Update your account password and review authentication safeguards.
          </p>
        </div>

      <form onSubmit={handleChangePassword} className="space-y-4 max-w-md">
        <div className="space-y-1.5">
          <label className="text-xs font-semibold text-slate-700">
            Current Password <span className="text-rose-500">*</span>
          </label>
          <div className="relative">
            <input
              type="password"
              required
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              placeholder="••••••••"
              className="w-full pl-9 pr-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
            />
            <Lock className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="text-xs font-semibold text-slate-700">
            New Password <span className="text-rose-500">*</span>
          </label>
          <div className="relative">
            <input
              type="password"
              required
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="At least 6 characters"
              className="w-full pl-9 pr-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
            />
            <KeyRound className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          </div>
        </div>

        <div className="space-y-1.5">
          <label className="text-xs font-semibold text-slate-700">
            Confirm New Password <span className="text-rose-500">*</span>
          </label>
          <div className="relative">
            <input
              type="password"
              required
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Re-type new password"
              className="w-full pl-9 pr-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
            />
            <Lock className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          </div>
        </div>

        <div className="pt-2">
          <button
            type="submit"
            disabled={loading}
            className="inline-flex items-center gap-2 px-6 py-3 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-md disabled:bg-slate-300"
          >
            {loading ? (
              <div className="w-4 h-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
            ) : (
              <>
                <Shield className="w-4 h-4" />
                <span>Update Password</span>
              </>
            )}
          </button>
        </div>
      </form>

      <div className="pt-6 border-t border-slate-100 max-w-md">
        <h3 className="text-xs font-bold text-[#0D1B2A] mb-2 flex items-center gap-1.5">
          <CheckCircle2 className="w-4 h-4 text-emerald-500" />
          <span>Security Recommendations</span>
        </h3>
        <ul className="space-y-1.5 text-[11px] text-slate-500 list-disc list-inside leading-relaxed">
          <li>Use a combination of uppercase, lowercase, numbers, and symbols.</li>
          <li>Do not share your GOshop password across other shopping platforms.</li>
          <li>If you suspect unauthorized activity, change your password immediately.</li>
        </ul>
      </div>
    </div>

    <SessionDeviceManager />
  </div>
)
}
