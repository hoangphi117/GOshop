"use client";

import React, { useState } from "react";
import Link from "next/link";
import { API_BASE_URL, API_KEY } from "@/lib/api";
import { extractErrorMessage } from "@/lib/utils";
import {
  Mail,
  ArrowRight,
  ArrowLeft,
  AlertCircle,
  Terminal,
  Zap,
  Lock as LockIcon,
  Truck,
  ShieldCheck,
  Award,
  CheckCircle2,
  Loader2,
} from "lucide-react";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [loading, setLoading] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [emailError, setEmailError] = useState<string | null>(null);

  const validate = () => {
    const trimmed = email.trim();
    if (!trimmed) {
      setEmailError("Please enter your email address");
      return false;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmed)) {
      setEmailError("Please enter a valid email address");
      return false;
    }
    setEmailError(null);
    return true;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setGeneralError(null);

    if (!validate()) return;

    setLoading(true);

    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/forgot-password`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-API-KEY": API_KEY,
        },
        body: JSON.stringify({ email: email.trim().toLowerCase() }),
      });

      const data = await res.json().catch(() => null);

      if (!res.ok) {
        throw new Error(
          extractErrorMessage(
            data,
            "Failed to request password reset. Please verify your email.",
          ),
        );
      }

      setIsSubmitted(true);
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "An error occurred. Please try again.";
      setGeneralError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="min-h-screen bg-[#FAF8FF] text-[#131B2A] antialiased"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      {/* MAIN CONTAINER */}
      <main className="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-8 py-2 flex flex-col items-center justify-center">
        <div className="flex flex-col w-full items-center justify-center relative py-2 sm:py-4">
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
                  Reset Password
                </h1>
                <p className="text-xs text-[#3E484D] mt-1 max-w-xs">
                  Enter your registered email address and we&apos;ll send you
                  instructions to reset your password.
                </p>
              </div>

              {/* General Error Banner */}
              {generalError && !isSubmitted && (
                <div className="mb-4 p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-xs text-rose-600 flex items-start gap-2.5">
                  <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
                  <span className="font-medium leading-relaxed">
                    {generalError}
                  </span>
                </div>
              )}

              {/* SUBMITTED CONFIRMATION VIEW */}
              {isSubmitted ? (
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
                      Check Your Email
                    </h3>
                    <p className="text-xs text-[#3E484D] leading-relaxed">
                      We have sent password reset instructions to{" "}
                      <strong className="text-[#131B2A] font-bold">
                        {email}
                      </strong>
                      .
                    </p>
                  </div>

                  <div className="p-3.5 rounded-xl bg-[#F8FAFC] border border-[#E2E8F0] text-[18px] text-[#6F797E] text-left space-y-1">
                    <p>
                      • If you don&apos;t see the email, please check your Spam
                      or Junk folder.
                    </p>
                    <p>
                      • The reset link/token is valid for a limited period for
                      security reasons.
                    </p>
                  </div>

                  <div className="pt-2 space-y-2.5">
                    <Link
                      href="/reset-password"
                      className="w-full py-3 px-4 rounded-lg bg-[#00637C] hover:bg-[#004D62] text-white text-xs font-bold flex items-center justify-center gap-2 shadow-md shadow-[#00637C]/25 transition-all group"
                      style={{
                        fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                      }}
                    >
                      <span>Enter Reset Token</span>
                      <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                    </Link>

                    <button
                      type="button"
                      onClick={() => {
                        setIsSubmitted(false);
                        setGeneralError(null);
                        setEmailError(null);
                      }}
                      className="w-full py-2.5 px-4 rounded-lg bg-white border border-[#E2E8F0] text-[#131B2A] text-xs font-bold hover:bg-[#F2F3FF] transition-all cursor-pointer"
                    >
                      Resend Email
                    </button>
                  </div>
                </div>
              ) : (
                /* INITIAL REQUEST FORM VIEW */
                <form
                  onSubmit={handleSubmit}
                  noValidate
                  className="flex flex-col gap-4"
                >
                  <div className="flex flex-col gap-1.5">
                    <label className="text-xs font-bold text-[#131B2A] flex items-center gap-1">
                      <span>Email Address</span>
                      <span className="text-rose-600">*</span>
                    </label>
                    <div className="relative flex items-center">
                      <Mail className="w-5 h-5 absolute left-3 text-[#6F797E] pointer-events-none" />
                      <input
                        type="email"
                        required
                        value={email}
                        onChange={(e) => {
                          setEmail(e.target.value);
                          if (emailError) setEmailError(null);
                          if (generalError) setGeneralError(null);
                        }}
                        placeholder="gopher@developer.dev"
                        className={`w-full bg-white text-[#131B2A] text-sm pl-10 pr-4 py-2.5 rounded-lg border shadow-2xs focus:outline-none focus:ring-2 focus:ring-[#007D9C] placeholder-[#6F797E]/70 transition-all ${
                          emailError
                            ? "border-rose-500 bg-rose-50/20"
                            : "border-[#BEC8CD]"
                        }`}
                      />
                    </div>
                    {emailError && (
                      <p className="text-[11px] font-medium text-rose-600 flex items-center gap-1 mt-0.5">
                        <AlertCircle className="w-3 h-3 shrink-0" />
                        <span>{emailError}</span>
                      </p>
                    )}
                  </div>

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
                        <span>Sending Instructions...</span>
                      </>
                    ) : (
                      <>
                        <span>Send Reset Instructions</span>
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
                  href="/reset-password"
                  className="font-bold text-[#00637C] hover:underline"
                >
                  Have a token?
                </Link>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
