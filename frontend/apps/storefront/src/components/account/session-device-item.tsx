'use client'

import React from 'react'
import { Laptop, Smartphone, Tablet, Monitor, Globe, LogOut, CheckCircle2, Clock, MapPin, Loader2 } from 'lucide-react'
import { UserSession } from '@/types/session'

interface SessionDeviceItemProps {
  session: UserSession
  isRevoking: boolean
  onRevoke: (id: string) => void
}

function getDeviceIcon(deviceName: string) {
  const name = deviceName.toLowerCase()
  if (name.includes('iphone') || name.includes('android') || name.includes('mobile')) {
    return Smartphone
  }
  if (name.includes('ipad') || name.includes('tablet')) {
    return Tablet
  }
  if (name.includes('mac') || name.includes('windows') || name.includes('linux')) {
    return Laptop
  }
  if (name.includes('desktop') || name.includes('pc')) {
    return Monitor
  }
  return Globe
}

function formatRelativeTime(dateString: string): string {
  try {
    const date = new Date(dateString)
    const now = new Date()
    const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000)

    if (diffInSeconds < 60) return 'Just now'
    const diffInMinutes = Math.floor(diffInSeconds / 60)
    if (diffInMinutes < 60) return `${diffInMinutes}m ago`
    const diffInHours = Math.floor(diffInMinutes / 60)
    if (diffInHours < 24) return `${diffInHours}h ago`
    const diffInDays = Math.floor(diffInHours / 24)
    if (diffInDays < 30) return `${diffInDays}d ago`

    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  } catch {
    return dateString
  }
}

export function SessionDeviceItem({ session, isRevoking, onRevoke }: SessionDeviceItemProps) {
  const Icon = getDeviceIcon(session.device_name)

  return (
    <div
      className={`p-4 rounded-2xl border transition-all ${
        session.is_current
          ? 'bg-blue-50/40 border-blue-200/80 shadow-xs'
          : 'bg-white border-slate-200/70 hover:border-slate-300'
      } flex flex-col sm:flex-row sm:items-center justify-between gap-4`}
    >
      <div className="flex items-start sm:items-center gap-3.5 min-w-0">
        <div
          className={`w-11 h-11 rounded-xl flex items-center justify-center shrink-0 ${
            session.is_current
              ? 'bg-[#2196F3] text-white shadow-sm'
              : 'bg-slate-100 text-slate-600'
          }`}
        >
          <Icon className="w-5 h-5" />
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <h4 className="text-xs font-bold text-[#0D1B2A] truncate">
              {session.device_name || 'Unknown Device'}
            </h4>
            {session.is_current && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-100 text-emerald-700 border border-emerald-200">
                <CheckCircle2 className="w-3 h-3" />
                Current Device
              </span>
            )}
          </div>

          <div className="flex items-center gap-3 mt-1 text-[11px] text-slate-500 flex-wrap">
            <span className="inline-flex items-center gap-1 font-mono">
              <MapPin className="w-3 h-3 text-slate-400" />
              {session.ip_address || 'Unknown IP'}
            </span>
            <span>•</span>
            <span className="inline-flex items-center gap-1">
              <Clock className="w-3 h-3 text-slate-400" />
              Last active: {formatRelativeTime(session.last_active_at)}
            </span>
          </div>
        </div>
      </div>

      {!session.is_current && (
        <button
          type="button"
          onClick={() => onRevoke(session.id)}
          disabled={isRevoking}
          className="self-end sm:self-center inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-rose-200 text-rose-600 hover:bg-rose-50 hover:border-rose-300 text-xs font-semibold transition-all disabled:opacity-50 cursor-pointer"
        >
          {isRevoking ? (
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
          ) : (
            <LogOut className="w-3.5 h-3.5" />
          )}
          <span>Log out</span>
        </button>
      )}
    </div>
  )
}
