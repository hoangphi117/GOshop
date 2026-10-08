"use client";

import React, { useState } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { authFetch } from "@/lib/api";
import {
  Shield,
  Lock,
  CheckCircle2,
  KeyRound,
  ShieldCheck,
  Key,
  Eye,
  EyeOff,
} from "lucide-react";
import { toast } from "sonner";
import { SessionDeviceManager } from "@/components/account/session-device-manager";

export default function SecurityPage() {
  const { accessToken } = useAuthStore();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);

  // Toggle visibility states
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();

    if (newPassword.length < 6) {
      toast.error("New password must be at least 6 characters long.");
      return;
    }

    if (newPassword !== confirmPassword) {
      toast.error("New passwords do not match.");
      return;
    }

    setLoading(true);

    try {
      const res = await authFetch("/api/v1/users/me/change-password", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      });

      const data = await res.json().catch(() => null);

      if (!res.ok) {
        throw new Error(
          data?.message ||
            "Failed to update password. Verify your current password.",
        );
      }

      toast.success("Your password has been changed successfully!");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "Error changing password";
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="space-y-6"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      {/* Page Header & Security Status Banner */}
      <div className="bg-white border border-[#E2E8F0] rounded-md p-6 flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1
            className="text-2xl font-bold text-[#0F172A] tracking-tight"
            style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
          >
            Password & Security
          </h1>
          <p className="text-sm text-[#64748B] mt-1">
            Manage your authentication credentials, multi-factor authorization,
            and active browser sessions.
          </p>
        </div>
      </div>

      {/* Card 1: Change Password Section */}
      <div className="bg-white border border-[#E2E8F0] rounded-md p-6">
        <div className="border-b border-[#E2E8F0] pb-4 mb-6">
          <h2
            className="text-lg font-bold text-[#0F172A] flex items-center gap-2"
            style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
          >
            <Key className="w-5 h-5 text-[#007D9C]" />
            <span>Change Password</span>
          </h2>
          <p className="text-xs text-[#64748B] mt-1">
            Ensure your account is using a long, random password to stay secure
            against brute-force attacks.
          </p>
        </div>

        <form onSubmit={handleChangePassword} className="space-y-5 max-w-lg">
          {/* Current Password */}
          <div className="space-y-1.5">
            <label className="block text-xs font-bold text-[#0F172A] uppercase tracking-wide">
              Current Password <span className="text-[#EF4444]">*</span>
            </label>
            <div className="relative">
              <input
                type={showCurrent ? "text" : "password"}
                required
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full bg-[#FFFFFF] border border-[#E2E8F0] rounded-md px-3.5 py-2.5 text-sm text-[#0F172A] pr-10 focus:outline-none focus:border-[#007D9C]"
              />
              <button
                type="button"
                onClick={() => setShowCurrent(!showCurrent)}
                className="absolute inset-y-0 right-0 pr-3 flex items-center text-[#64748B] hover:text-[#0F172A] cursor-pointer"
              >
                {showCurrent ? (
                  <EyeOff className="w-4 h-4" />
                ) : (
                  <Eye className="w-4 h-4" />
                )}
              </button>
            </div>
          </div>

          {/* New Password */}
          <div className="space-y-1.5">
            <label className="block text-xs font-bold text-[#0F172A] uppercase tracking-wide">
              New Password <span className="text-[#EF4444]">*</span>
            </label>
            <div className="relative">
              <input
                type={showNew ? "text" : "password"}
                required
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="At least 6 characters"
                className="w-full bg-[#FFFFFF] border border-[#E2E8F0] rounded-md px-3.5 py-2.5 text-sm text-[#0F172A] pr-10 focus:outline-none focus:border-[#007D9C]"
              />
              <button
                type="button"
                onClick={() => setShowNew(!showNew)}
                className="absolute inset-y-0 right-0 pr-3 flex items-center text-[#64748B] hover:text-[#0F172A] cursor-pointer"
              >
                {showNew ? (
                  <EyeOff className="w-4 h-4" />
                ) : (
                  <Eye className="w-4 h-4" />
                )}
              </button>
            </div>
          </div>

          {/* Confirm New Password */}
          <div className="space-y-1.5">
            <label className="block text-xs font-bold text-[#0F172A] uppercase tracking-wide">
              Confirm New Password <span className="text-[#EF4444]">*</span>
            </label>
            <div className="relative">
              <input
                type="password"
                required
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Re-type new password"
                className="w-full bg-[#FFFFFF] border border-[#E2E8F0] rounded-md px-3.5 py-2.5 text-sm text-[#0F172A] pr-10 focus:outline-none focus:border-[#007D9C]"
              />
              <Lock className="w-4 h-4 text-[#64748B] absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none" />
            </div>
          </div>

          {/* Password Requirements Checklist */}
          <div className="bg-[#F8FAFC] border border-[#E2E8F0] rounded-md p-3.5">
            <div
              className="text-xs font-bold text-[#0F172A] mb-2"
              style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
            >
              Requirements checklist:
            </div>
            <div className="flex flex-col gap-2 text-xs">
              <div className="flex items-center gap-1.5 text-[#059669] font-medium">
                <span>Minimum 6 characters</span>
              </div>
              <div className="flex items-center gap-1.5 text-[#059669] font-medium">
                <span>
                  Must include at least one uppercase letter, one lowercase
                  letter, one number, and one special character
                </span>
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="pt-2 flex items-center gap-4">
            <button
              type="submit"
              disabled={loading}
              className="bg-[#007D9C] hover:bg-[#00657E] text-white font-semibold px-5 py-2.5 rounded-md text-sm transition-colors cursor-pointer disabled:opacity-50 inline-flex items-center gap-2"
              style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
            >
              {loading ? (
                <>
                  <div className="w-4 h-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
                  <span>Updating...</span>
                </>
              ) : (
                <>
                  <Shield className="w-4 h-4" />
                  <span>Update Password</span>
                </>
              )}
            </button>
          </div>
        </form>

        <div className="pt-6 border-t border-[#E2E8F0] mt-3 max-w-lg">
          <h3
            className="text-xs font-bold text-[#0F172A] mb-2 flex items-center gap-1.5"
            style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
          >
            <span>Security Recommendations</span>
          </h3>
          <ul className="space-y-1.5 text-xs text-[#64748B] list-disc list-inside leading-relaxed">
            <li>
              Use a combination of uppercase, lowercase, numbers, and symbols.
            </li>
            <li>
              Do not share your GOshop password across other shopping platforms.
            </li>
            <li>
              If you suspect unauthorized activity, change your password
              immediately.
            </li>
          </ul>
        </div>
      </div>

      {/* Session and Active Device Manager Component */}
      <div className="bg-white border border-[#E2E8F0] rounded-md p-6">
        <SessionDeviceManager />
      </div>
    </div>
  );
}
