"use client";

import React from "react";
import {
  Laptop,
  Smartphone,
  Tablet,
  Monitor,
  Globe,
  LogOut,
  CheckCircle2,
  Clock,
  MapPin,
  Loader2,
} from "lucide-react";
import { UserSession } from "@/types/session";

interface SessionDeviceItemProps {
  session: UserSession;
  isRevoking: boolean;
  onRevoke: (id: string) => void;
}

function DeviceIcon({ deviceName }: { deviceName: string }) {
  const name = deviceName.toLowerCase();
  if (
    name.includes("iphone") ||
    name.includes("android") ||
    name.includes("mobile")
  ) {
    return <Smartphone className="w-5 h-5" />;
  }
  if (name.includes("ipad") || name.includes("tablet")) {
    return <Tablet className="w-5 h-5" />;
  }
  if (
    name.includes("mac") ||
    name.includes("windows") ||
    name.includes("linux")
  ) {
    return <Laptop className="w-5 h-5" />;
  }
  if (name.includes("desktop") || name.includes("pc")) {
    return <Monitor className="w-5 h-5" />;
  }
  return <Globe className="w-5 h-5" />;
}

function formatRelativeTime(dateString: string): string {
  try {
    const date = new Date(dateString);
    const now = new Date();
    const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

    if (diffInSeconds < 60) return "Just now";
    const diffInMinutes = Math.floor(diffInSeconds / 60);
    if (diffInMinutes < 60) return `${diffInMinutes}m ago`;
    const diffInHours = Math.floor(diffInMinutes / 60);
    if (diffInHours < 24) return `${diffInHours}h ago`;
    const diffInDays = Math.floor(diffInHours / 24);
    if (diffInDays < 30) return `${diffInDays}d ago`;

    return date.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  } catch {
    return dateString;
  }
}

export function SessionDeviceItem({
  session,
  isRevoking,
  onRevoke,
}: SessionDeviceItemProps) {
  return (
    <div
      className={`p-4 rounded-md border transition-all ${
        session.is_current
          ? "bg-[#E0F7FC]/40 border-primary shadow-2xs"
          : "bg-white border-border-subtle hover:border-[#CBD5E1]"
      } flex flex-col sm:flex-row sm:items-center justify-between gap-4`}
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      <div className="flex items-start sm:items-center gap-3.5 min-w-0">
        <div
          className={`w-11 h-11 rounded-md flex items-center justify-center shrink-0 ${
            session.is_current
              ? "bg-primary text-white shadow-xs"
              : "bg-[#F1F5F9] text-text-secondary"
          }`}
        >
          <DeviceIcon deviceName={session.device_name} />
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <h4
              className="text-xs font-bold text-text-primary truncate"
              style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
            >
              {session.device_name || "Unknown Device"}
            </h4>
            {session.is_current && (
              <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-xs text-[10px] font-bold bg-emerald-50 text-emerald-700 border border-emerald-200">
                Current Device
              </span>
            )}
          </div>

          <div className="flex items-center gap-3 mt-1 text-[11px] text-text-secondary flex-wrap">
            <span className="inline-flex items-center gap-1 font-mono">
              <MapPin className="w-3 h-3 text-[#94A3B8]" />
              {session.ip_address || "Unknown IP"}
            </span>
            <span>•</span>
            <span className="inline-flex items-center gap-1">
              <Clock className="w-3 h-3 text-[#94A3B8]" />
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
          className="self-end sm:self-center inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-md border border-rose-200 text-rose-600 hover:bg-rose-50 hover:border-rose-300 text-xs font-bold transition-all disabled:opacity-50 cursor-pointer"
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
  );
}
