"use client";

import React, { useState } from "react";
import {
  MonitorSmartphone,
  ShieldAlert,
  LogOut,
  RefreshCw,
  Loader2,
  AlertTriangle,
} from "lucide-react";
import { useUserSessions } from "@/hooks/use-user-sessions";
import { SessionDeviceItem } from "./session-device-item";

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
  } = useUserSessions();

  const [confirmRevokeAll, setConfirmRevokeAll] = useState(false);

  const handleRevokeAllClick = () => {
    if (!confirmRevokeAll) {
      setConfirmRevokeAll(true);
      return;
    }
    revokeAllOtherSessions().then(() => {
      setConfirmRevokeAll(false);
    });
  };

  return (
    <div
      className="p-6 sm:p-8 rounded-md bg-white border border-[#E2E8F0] shadow-sm space-y-6"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-[#E2E8F0]">
        <div>
          <div className="flex items-center">
            <h2
              className="text-lg font-bold text-[#0F172A]"
              style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
            >
              Active Devices & Sessions
            </h2>
            <span className="px-2.5 py-0.5 rounded-md text-md font-bold text-[#007D9C] ">
              ({sessions.length})
            </span>
          </div>
          <p className="text-xs text-[#64748B] mt-1">
            Manage devices currently logged into your account. If you spot an
            unrecognized device, revoke it immediately.
          </p>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-center">
          <button
            type="button"
            onClick={reload}
            disabled={isLoading}
            className="p-2 rounded-md border border-[#E2E8F0] text-[#64748B] hover:bg-[#F8FAFC] transition-colors disabled:opacity-50 cursor-pointer"
            title="Refresh sessions"
          >
            <RefreshCw
              className={`w-4 h-4 ${isLoading ? "animate-spin" : ""}`}
            />
          </button>

          {otherSessionsCount > 0 && (
            <button
              type="button"
              onClick={handleRevokeAllClick}
              disabled={isRevokingAll}
              className={`inline-flex items-center gap-1.5 px-3.5 py-2 rounded-md text-xs font-bold transition-all cursor-pointer ${
                confirmRevokeAll
                  ? "bg-rose-600 hover:bg-rose-700 text-white shadow-md"
                  : "bg-rose-50 hover:bg-rose-100 text-rose-700 border border-rose-200"
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
                  ? "Confirm log out all others?"
                  : "Log out other devices"}
              </span>
            </button>
          )}
        </div>
      </div>

      {confirmRevokeAll && (
        <div className="p-3.5 rounded-md bg-amber-50 border border-amber-200 flex items-center justify-between text-xs text-amber-800">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-600 shrink-0" />
            <span>
              This will immediately disconnect {otherSessionsCount} other active
              session(s).
            </span>
          </div>
          <button
            type="button"
            onClick={() => setConfirmRevokeAll(false)}
            className="text-xs font-semibold underline hover:text-amber-950 ml-2 cursor-pointer"
          >
            Cancel
          </button>
        </div>
      )}

      {/* Sessions List */}
      <div className="space-y-3">
        {isLoading && sessions.length === 0 ? (
          <div className="py-8 flex flex-col items-center justify-center text-[#64748B] space-y-2">
            <Loader2 className="w-6 h-6 animate-spin text-[#007D9C]" />
            <p className="text-xs">Loading active devices...</p>
          </div>
        ) : sessions.length === 0 ? (
          <div className="py-8 text-center text-[#64748B] space-y-2">
            <MonitorSmartphone className="w-8 h-8 mx-auto text-[#CBD5E1]" />
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
    </div>
  );
}
