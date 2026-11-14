"use client";

import React, { useState, useEffect } from "react";
import { useAuthStore } from "@/stores/auth-store";
import { authFetch } from "@/lib/api";
import { User, Mail, Phone, Save, CheckCircle2 } from "lucide-react";
import { toast } from "sonner";
import { AvatarUploader } from "@/components/account/avatar-uploader";

export default function ProfilePage() {
  const { user, accessToken, setUser } = useAuthStore();

  const [fullName, setFullName] = useState(user?.fullName || "");
  const [phone, setPhone] = useState("");
  const [avatarUrl, setAvatarUrl] = useState(user?.avatarUrl || "");
  const [loading, setLoading] = useState(false);
  const [fetching, setFetching] = useState(true);

  // Fetch current user details from /api/v1/users/me
  useEffect(() => {
    if (!accessToken) return;

    authFetch("/api/v1/users/me")
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.data) {
          const profile = data.data;
          const currentUser = useAuthStore.getState().user;
          setFullName(
            profile.full_name ||
              profile.fullName ||
              currentUser?.fullName ||
              "",
          );
          setPhone(profile.phone_number || profile.phone || "");
          setAvatarUrl(profile.avatar_url || currentUser?.avatarUrl || "");
          if (currentUser) {
            setUser({
              ...currentUser,
              fullName:
                profile.full_name || profile.fullName || currentUser.fullName,
              avatarUrl: profile.avatar_url || currentUser.avatarUrl,
            });
          }
        }
      })
      .catch(() => {
        // Fallback to local store data
      })
      .finally(() => {
        setFetching(false);
      });
  }, [accessToken, setUser]);

  const handleUpdateProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      const res = await authFetch("/api/v1/users/me", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          full_name: fullName,
          phone_number: phone,
        }),
      });

      if (!res.ok) {
        throw new Error("Failed to update profile. Please try again.");
      }

      if (user) {
        setUser({
          ...user,
          fullName,
        });
      }

      toast.success("Your profile has been updated successfully!");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Error updating profile";
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  };

  if (fetching) {
    return (
      <div className="p-8 rounded-md bg-white border border-[#E2E8F0] shadow-xs flex items-center justify-center min-h-[300px]">
        <div className="w-8 h-8 rounded-md border-2 border-[#007D9C] border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="space-y-6 font-body">
      {/* Main Profile Form Card */}
      <div className="p-6 sm:p-8 rounded-md bg-white border border-[#E2E8F0] shadow-xs space-y-6">
        <div>
          <h1 className="text-xl font-bold text-[#0F172A] tracking-tight font-heading">
            Personal Information
          </h1>
          <p className="text-xs text-slate-500 mt-1">
            Manage your personal identity, contact details, and account
            preferences.
          </p>
        </div>

        <form onSubmit={handleUpdateProfile} className="space-y-5">
          {/* Avatar */}
          <div className="space-y-2">
            <label className="text-xs font-semibold text-slate-700">
              Profile Photo
            </label>
            <AvatarUploader
              value={avatarUrl}
              onUploaded={(url) => {
                setAvatarUrl(url);
                const current = useAuthStore.getState().user;
                if (current) setUser({ ...current, avatarUrl: url });
              }}
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
            {/* Full Name */}
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                Full Name
              </label>
              <div className="relative">
                <input
                  type="text"
                  required
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  placeholder="Enter your full name"
                  className="w-full pl-9 pr-3.5 py-2.5 rounded-md bg-slate-50 border border-[#E2E8F0] text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C]"
                />
                <User className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              </div>
            </div>

            {/* Phone Number */}
            <div className="space-y-1.5">
              <label className="text-xs font-semibold text-slate-700">
                Phone Number
              </label>
              <div className="relative">
                <input
                  type="tel"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  placeholder="e.g. 0901234567"
                  className="w-full pl-9 pr-3.5 py-2.5 rounded-md bg-slate-50 border border-[#E2E8F0] text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C]"
                />
                <Phone className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              </div>
            </div>
          </div>

          {/* Email Address (Read-only) */}
          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-slate-700">
              Email Address (Primary Account Identity)
            </label>
            <div className="relative">
              <input
                type="email"
                disabled
                value={user?.email || ""}
                className="w-full pl-9 pr-24 py-2.5 rounded-md bg-slate-100 border border-[#E2E8F0] text-xs text-slate-500 cursor-not-allowed"
              />
              <Mail className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
              <span className="absolute right-3 top-1/2 -translate-y-1/2 text-[10px] font-bold text-[#007D9C]  px-2 py-0.5 rounded-md  flex items-center gap-1">
                <CheckCircle2 className="w-3.5 h-3.5" /> Verified
              </span>
            </div>
            <p className="text-[11px] text-slate-400">
              Email address is linked to your authentication provider and cannot
              be changed directly.
            </p>
          </div>

          {/* Submit CTA */}
          <div className="pt-2 flex justify-end">
            <button
              type="submit"
              disabled={loading}
              className="inline-flex items-center gap-2 px-6 py-2.5 rounded-md bg-[#007D9C] text-white text-xs font-bold hover:bg-[#006782] transition-colors shadow-xs disabled:bg-slate-300 cursor-pointer font-heading"
            >
              {loading ? (
                <div className="w-4 h-4 rounded-md border-2 border-white border-t-transparent" />
              ) : (
                <>
                  <Save className="w-4 h-4" />
                  <span>Save Changes</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
