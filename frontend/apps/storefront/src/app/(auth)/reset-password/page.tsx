"use client";

import React, { useState, useEffect, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { extractErrorMessage } from "@/lib/utils";
import { API_BASE_URL, API_KEY } from "@/lib/api";
import {
  Lock,
  KeyRound,
  ArrowRight,
  ArrowLeft,
  ShieldCheck,
  CheckCircle2,
  AlertCircle,
  Eye,
  EyeOff,
  Terminal,
  Zap,
  Lock as LockIcon,
  Truck,
  Award,
  Loader2,
} from "lucide-react";

function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const [token, setToken] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{
    token?: string;
    newPassword?: string;
    confirmPassword?: string;
  }>({});

  useEffect(() => {
    const tokenFromUrl = searchParams.get("token");
    if (tokenFromUrl) {
      setToken(tokenFromUrl);
    }
  }, [searchParams]);

  const validate = () => {
    const errors: {
      token?: string;
      newPassword?: string;
      confirmPassword?: string;
    } = {};

    if (!token.trim()) {
      errors.token = "Please enter the reset token received via email";
    }

    if (!newPassword) {
      errors.newPassword = "Please enter a new password";
    } else if (newPassword.length < 6) {
      errors.newPassword = "Password must be at least 6 characters long";
    }

    if (!confirmPassword) {
      errors.confirmPassword = "Please confirm your new password";
    } else if (confirmPassword !== newPassword) {
      errors.confirmPassword = "Passwords do not match";
    }

    setFieldErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setGeneralError(null);

    if (!validate()) return;

    setLoading(true);

    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/reset-password`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-API-KEY": API_KEY,
        },
        body: JSON.stringify({
          token: token.trim(),
          new_password: newPassword,
        }),
      });

      const data = await res.json().catch(() => null);

      if (!res.ok) {
        throw new Error(
          extractErrorMessage(
            data,
            "Failed to reset password. The token may be expired or invalid.",
          ),
        );
      }

      setIsSuccess(true);
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Failed to reset password.";
      setGeneralError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="w-full max-w-7xl mx-auto px-4 sm:px-8 min-h-[80vh]"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      <div className="flex flex-col w-full items-center justify-center relative py-4 sm:py-8">
        {/* Central Authentication Container */}
        <div className="w-full max-w-[440px] bg-white rounded-2xl shadow-xl shadow-indigo-100/40 overflow-hidden flex flex-col relative transition-all duration-300 border border-[#E2E8F0]">
          {/* Top Decorative Line Stripe */}
          <div className="h-1.5 w-full bg-gradient-to-r from-[#00637C] via-[#4FD2FE] to-[#9F3800]" />

          <div className="p-6 sm:p-8 flex flex-col">
            {/* Mascot Avatar & Header */}
            <div className="flex flex-col items-center mb-6 text-center">
              <div className="mb-2 h-7 flex items-center justify-center">
                <span
                  className="text-2xl font-bold text-[#007D9C] tracking-tight"
                  style={{
                    fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                  }}
                >
                  GO<span className="text-[#F26522]">shop</span>
                </span>
              </div>

              <h1
                className="text-2xl font-bold text-[#131B2A] mt-3"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                Create New Password
              </h1>
              <p className="text-xs text-[#3E484D] mt-1 max-w-xs">
                Enter your verification token and choose a strong new password
                for your account.
              </p>
            </div>

            {/* General Error Banner */}
            {generalError && (
              <div className="mb-4 p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-xs text-rose-600 flex items-start gap-2.5">
                <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
                <span className="font-medium leading-relaxed">
                  {generalError}
                </span>
              </div>
            )}

            {/* SUCCESS VIEW */}
            {isSuccess ? (
              <div className="text-center space-y-4 py-2">
                <div className="w-14 h-14 rounded-full bg-emerald-50 text-emerald-500 flex items-center justify-center mx-auto shadow-2xs border border-emerald-200">
                  <CheckCircle2 className="w-8 h-8" />
                </div>
                <div className="space-y-1">
                  <h3
                    className="text-base font-bold text-[#131B2A]"
                    style={{
                      fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                    }}
                  >
                    Password Reset Complete!
                  </h3>
                  <p className="text-xs text-[#3E484D] leading-relaxed">
                    Your password has been changed securely. You can now sign in
                    with your new credentials.
                  </p>
                </div>

                <div className="pt-2">
                  <Link
                    href="/login"
                    className="w-full py-3 px-4 rounded-lg bg-[#00637C] hover:bg-[#004D62] text-white text-xs font-bold flex items-center justify-center gap-2 shadow-md shadow-[#00637C]/25 transition-all group"
                    style={{
                      fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                    }}
                  >
                    <span>Sign In Now</span>
                    <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                  </Link>
                </div>
              </div>
            ) : (
              /* FORM VIEW */
              <form
                onSubmit={handleSubmit}
                noValidate
                className="flex flex-col gap-4"
              >
                {/* Reset Token Field */}
                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-bold text-[#131B2A] flex items-center gap-1">
                    <span>Reset Token / Code</span>
                    <span className="text-rose-600">*</span>
                  </label>
                  <div className="relative flex items-center">
                    <KeyRound className="w-5 h-5 absolute left-3 text-[#6F797E] pointer-events-none" />
                    <input
                      type="text"
                      required
                      value={token}
                      onChange={(e) => {
                        setToken(e.target.value);
                        if (fieldErrors.token)
                          setFieldErrors((prev) => ({
                            ...prev,
                            token: undefined,
                          }));
                        if (generalError) setGeneralError(null);
                      }}
                      placeholder="Enter token received via email"
                      className={`w-full bg-white text-[#131B2A] text-sm font-mono pl-10 pr-4 py-2.5 rounded-lg border shadow-2xs focus:outline-none focus:ring-2 focus:ring-[#007D9C] placeholder-[#6F797E]/70 transition-all ${
                        fieldErrors.token
                          ? "border-rose-500 bg-rose-50/20"
                          : "border-[#BEC8CD]"
                      }`}
                    />
                  </div>
                  {fieldErrors.token && (
                    <p className="text-[11px] font-medium text-rose-600 flex items-center gap-1 mt-0.5">
                      <AlertCircle className="w-3 h-3 shrink-0" />
                      <span>{fieldErrors.token}</span>
                    </p>
                  )}
                </div>

                {/* New Password Field */}
                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-bold text-[#131B2A] flex items-center gap-1">
                    <span>New Password</span>
                    <span className="text-rose-600">*</span>
                  </label>
                  <div className="relative flex items-center">
                    <LockIcon className="w-5 h-5 absolute left-3 text-[#6F797E] pointer-events-none" />
                    <input
                      type={showPassword ? "text" : "password"}
                      required
                      value={newPassword}
                      onChange={(e) => {
                        setNewPassword(e.target.value);
                        if (fieldErrors.newPassword)
                          setFieldErrors((prev) => ({
                            ...prev,
                            newPassword: undefined,
                          }));
                        if (generalError) setGeneralError(null);
                      }}
                      placeholder="At least 6 characters"
                      className={`w-full bg-white text-[#131B2A] text-sm pl-10 pr-11 py-2.5 rounded-lg border shadow-2xs focus:outline-none focus:ring-2 focus:ring-[#007D9C] placeholder-[#6F797E]/70 transition-all ${
                        fieldErrors.newPassword
                          ? "border-rose-500 bg-rose-50/20"
                          : "border-[#BEC8CD]"
                      }`}
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword(!showPassword)}
                      className="absolute right-3 p-1 text-[#6F797E] hover:text-[#131B2A] transition-colors rounded focus:outline-none cursor-pointer"
                    >
                      {showPassword ? (
                        <EyeOff className="w-5 h-5" />
                      ) : (
                        <Eye className="w-5 h-5" />
                      )}
                    </button>
                  </div>
                  {fieldErrors.newPassword && (
                    <p className="text-[11px] font-medium text-rose-600 flex items-center gap-1 mt-0.5">
                      <AlertCircle className="w-3 h-3 shrink-0" />
                      <span>{fieldErrors.newPassword}</span>
                    </p>
                  )}
                </div>

                {/* Confirm Password Field */}
                <div className="flex flex-col gap-1.5">
                  <label className="text-xs font-bold text-[#131B2A] flex items-center gap-1">
                    <span>Confirm New Password</span>
                    <span className="text-rose-600">*</span>
                  </label>
                  <div className="relative flex items-center">
                    <LockIcon className="w-5 h-5 absolute left-3 text-[#6F797E] pointer-events-none" />
                    <input
                      type={showPassword ? "text" : "password"}
                      required
                      value={confirmPassword}
                      onChange={(e) => {
                        setConfirmPassword(e.target.value);
                        if (fieldErrors.confirmPassword)
                          setFieldErrors((prev) => ({
                            ...prev,
                            confirmPassword: undefined,
                          }));
                        if (generalError) setGeneralError(null);
                      }}
                      placeholder="Repeat your new password"
                      className={`w-full bg-white text-[#131B2A] text-sm pl-10 pr-11 py-2.5 rounded-lg border shadow-2xs focus:outline-none focus:ring-2 focus:ring-[#007D9C] placeholder-[#6F797E]/70 transition-all ${
                        fieldErrors.confirmPassword
                          ? "border-rose-500 bg-rose-50/20"
                          : "border-[#BEC8CD]"
                      }`}
                    />
                  </div>
                  {fieldErrors.confirmPassword && (
                    <p className="text-[11px] font-medium text-rose-600 flex items-center gap-1 mt-0.5">
                      <AlertCircle className="w-3 h-3 shrink-0" />
                      <span>{fieldErrors.confirmPassword}</span>
                    </p>
                  )}
                </div>

                {/* Submit Button */}
                <button
                  type="submit"
                  disabled={loading}
                  className="w-full mt-1 py-3 px-4 rounded-lg bg-[#00637C] hover:bg-[#004D62] text-white text-xs font-bold flex items-center justify-center gap-2 shadow-md shadow-[#00637C]/25 transition-all group cursor-pointer disabled:opacity-50"
                  style={{
                    fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                  }}
                >
                  {loading ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>Resetting Password...</span>
                    </>
                  ) : (
                    <>
                      <span>Reset Password</span>
                      <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                    </>
                  )}
                </button>
              </form>
            )}

            {/* CARD FOOTER LINKS */}
            <div className="mt-6 pt-4 border-t border-[#E2E8F0] flex items-center justify-between text-xs text-[#3E484D]">
              <Link
                href="/login"
                className="inline-flex items-center gap-1 font-bold text-[#3E484D] hover:text-[#00637C] transition-colors"
              >
                <ArrowLeft className="w-3.5 h-3.5" />
                <span>Back to Sign In</span>
              </Link>

              <Link
                href="/forgot-password"
                className="font-bold text-[#00637C] hover:underline"
              >
                Request new token
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-[60vh] flex items-center justify-center">
          <Loader2 className="w-6 h-6 animate-spin text-[#00637C]" />
        </div>
      }
    >
      <ResetPasswordForm />
    </Suspense>
  );
}
