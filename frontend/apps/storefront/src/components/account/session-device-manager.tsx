'use client'

import React, { useState } from 'react'
import { MonitorSmartphone, ShieldAlert, LogOut, RefreshCw, Loader2, AlertTriangle } from 'lucide-react'
import { useUserSessions } from '@/hooks/use-user-sessions'
import { SessionDeviceItem } from './session-device-item'

export function SessionDeviceManager() {
  const {
    sessions,
    isLoading,
    revokingId,
    isRevokingAll,
    otherSessionsCount,
    reload,
    revokeSession,
    revokeAllOtherSessions,
  } = useUserSessions()

  const [confirmRevokeAll, setConfirmRevokeAll] = useState(false)

  const handleRevokeAllClick = () => {
    if (!confirmRevokeAll) {
      setConfirmRevokeAll(true)
      return
    }
    revokeAllOtherSessions().then(() => {
      setConfirmRevokeAll(false)
    })
  }

  return (
    <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-xl font-extrabold text-[#0D1B2A] tracking-tight">
              Active Devices & Sessions
            </h2>
            <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-blue-100 text-blue-700">
              {sessions.length}
            </span>
          </div>
          <p className="text-xs text-slate-500 mt-1">
            Manage devices currently logged into your account. If you spot an unrecognized device, revoke it immediately.
          </p>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-center">
          <button
            type="button"
            onClick={reload}
            disabled={isLoading}
            className="p-2 rounded-xl border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors disabled:opacity-50"
            title="Refresh sessions"
          >
            <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          </button>

          {otherSessionsCount > 0 && (
            <button
              type="button"
              onClick={handleRevokeAllClick}
              disabled={isRevokingAll}
              className={`inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-bold transition-all ${
                confirmRevokeAll
                  ? 'bg-rose-600 hover:bg-rose-700 text-white shadow-md animate-pulse'
                  : 'bg-rose-50 hover:bg-rose-100 text-rose-700 border border-rose-200'
              }`}
            >
              {isRevokingAll ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : confirmRevokeAll ? (
                <AlertTriangle className="w-3.5 h-3.5" />
              ) : (
                <LogOut className="w-3.5 h-3.5" />
              )}
              <span>
                {confirmRevokeAll
                  ? 'Confirm log out all others?'
                  : 'Log out other devices'}
              </span>
            </button>
          )}
        </div>
      </div>

      {confirmRevokeAll && (
        <div className="p-3.5 rounded-xl bg-amber-50 border border-amber-200/80 flex items-center justify-between text-xs text-amber-800">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-600 shrink-0" />
            <span>This will immediately disconnect {otherSessionsCount} other active session(s).</span>
          </div>
          <button
            type="button"
            onClick={() => setConfirmRevokeAll(false)}
            className="text-[11px] font-semibold underline hover:text-amber-950 ml-2"
          >
            Cancel
          </button>
        </div>
      )}

      {/* Sessions List */}
      <div className="space-y-3">
        {isLoading && sessions.length === 0 ? (
          <div className="py-8 flex flex-col items-center justify-center text-slate-400 space-y-2">
            <Loader2 className="w-6 h-6 animate-spin text-[#2196F3]" />
            <p className="text-xs">Loading active devices...</p>
          </div>
        ) : sessions.length === 0 ? (
          <div className="py-8 text-center text-slate-400 space-y-2">
            <MonitorSmartphone className="w-8 h-8 mx-auto text-slate-300" />
            <p className="text-xs">No active sessions found.</p>
          </div>
        ) : (
          sessions.map((session) => (
            <SessionDeviceItem
              key={session.id}
              session={session}
              isRevoking={revokingId === session.id}
              onRevoke={revokeSession}
            />
          ))
        )}
      </div>

      <div className="p-4 rounded-2xl bg-slate-50 border border-slate-100 flex items-start gap-3">
        <ShieldAlert className="w-4 h-4 text-slate-500 shrink-0 mt-0.5" />
        <p className="text-[11px] text-slate-500 leading-relaxed">
          Sessions are automatically refreshed as long as your device is active. Logging out a session will revoke its refresh token immediately.
        </p>
      </div>
    </div>
  )
}
