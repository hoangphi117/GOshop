"use client";

import React, { useState, useEffect, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { extractErrorMessage } from "@/lib/utils";
import { API_BASE_URL, API_KEY } from "@/lib/api";
import {
  Lock as LockIcon,
  ArrowRight,
  ArrowLeft,
  CheckCircle2,
  AlertCircle,
  Eye,
  EyeOff,
  Loader2,
  Link2Off,
} from "lucide-react";

/**
 * Moves the one-time reset token out of the page and into memory.
 *
 * The token arrives in the email link, and it must not stay in the address bar:
 * a credential in a URL leaks through browser history and through the Referer
 * header of any third-party resource the page loads. Reading it once into a ref
 * and rewriting the URL immediately keeps it out of both, without rendering it
 * (unlike a form field, which shows the same secret to anyone watching the
 * screen or a recording).
 */
const capturedToken = { value: "" };

function captureTokenFromUrl(search: string): string {
  const token = new URLSearchParams(search).get("token") ?? "";
  if (token) capturedToken.value = token;
  return token;
}

function ResetPasswordForm() {
  const router = useRouter();
  const searchParams = useSearchParams();

  const tokenFromUrl = captureTokenFromUrl(searchParams.toString());
  const [token] = useState(tokenFromUrl || capturedToken.value);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{
    newPassword?: string;
    confirmPassword?: string;
  }>({});

  // Drop ?token= from the address bar and replace the history entry, so a
  // reload or a copied link does not carry a usable credential.
  useEffect(() => {
    if (token && searchParams.has("token")) {
      router.replace("/reset-password", { scroll: false });
    }
  }, [token, searchParams, router]);

  const validate = () => {
    const errors: {
      newPassword?: string;
      confirmPassword?: string;
    } = {};

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
          "X-Client-App": "storefront",
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
            "Failed to reset password. The link may be expired or invalid.",
          ),
        );
      }

      // The token is spent; drop it so nothing in this tab can replay it.
      capturedToken.value = "";
      setIsSuccess(true);
    } catch (err: unknown) {
      const message =
        err instanceof Error ? err.message : "Failed to reset password.";
      setGeneralError(message);
    } finally {
      setLoading(false);
    }
  };

  // Without a link token there is nothing to submit, so the form is not shown.
  const invalidLink = !token && !isSuccess;

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
                Choose a strong new password for your account.
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
            ) : invalidLink ? (
              /* INVALID LINK VIEW: a missing, used or expired link must not
                 render as a form the visitor could fill in. */
              <div className="text-center space-y-4 py-2">
                <div className="w-14 h-14 rounded-full bg-amber-50 text-amber-600 flex items-center justify-center mx-auto shadow-2xs border border-amber-200">
                  <Link2Off className="w-8 h-8" />
                </div>
                <div className="space-y-1">
                  <h3
                    className="text-base font-bold text-[#131B2A]"
                    style={{
                      fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                    }}
                  >
                    This reset link is no longer valid
                  </h3>
                  <p className="text-xs text-[#3E484D] leading-relaxed">
                    Reset links can be used once and expire quickly. Request a
                    new one and open it from your inbox.
                  </p>
                </div>
                <div className="pt-2">
                  <Link
                    href="/forgot-password"
                    className="w-full py-3 px-4 rounded-lg bg-[#00637C] hover:bg-[#004D62] text-white text-xs font-bold flex items-center justify-center gap-2 shadow-md shadow-[#00637C]/25 transition-all group"
                    style={{
                      fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                    }}
                  >
                    <span>Request a new link</span>
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
                      autoComplete="new-password"
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
                      autoComplete="new-password"
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
                Request new link
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
