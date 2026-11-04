"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  AlertCircle,
  ArrowRight,
  Eye,
  EyeOff,
  Loader2,
  Lock as LockIcon,
  Mail,
} from "lucide-react";
import { GoogleLoginButton } from "@/components/auth/google-login-button";
import { API_BASE_URL, API_KEY } from "@/lib/api";
import { useAuthStore } from "@/stores/auth-store";

export default function LoginPage() {
  const router = useRouter();
  const { setTokens, setUser } = useAuthStore();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{
    email?: string;
    password?: string;
  }>({});

  const validate = () => {
    const errors: { email?: string; password?: string } = {};
    const trimmedEmail = email.trim();

    if (!trimmedEmail) {
      errors.email = "Please enter your email address";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
      errors.email = "Please enter a valid email address";
    }

    if (!password) {
      errors.password = "Please enter your password";
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
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-API-KEY": API_KEY,
          "X-Client-App": "storefront",
        },
        credentials: "include",
        body: JSON.stringify({ email: email.trim(), password }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          data?.message ||
            "Login failed. Please verify your email and password.",
        );
      }

      const accessToken = data.data.accessToken || data.data.access_token || "";
      // Refresh token arrives as an HttpOnly cookie; never stored client-side.
      setTokens(accessToken);

      setUser({
        uuid: data.data.user?.uuid,
        email: data.data.user?.email || email,
        fullName: data.data.user?.full_name || data.data.user?.fullName || "",
        role: data.data.user?.roles || [],
      });

      const redirectUrl =
        typeof window !== "undefined"
          ? new URLSearchParams(window.location.search).get("redirect") || "/"
          : "/";
      router.push(redirectUrl);
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "An error occurred during sign in.";
      setGeneralError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col w-full items-center justify-center relative py-4 sm:py-8">
      <div className="w-full max-w-[440px] bg-white rounded-2xl shadow-xl shadow-indigo-100/40 overflow-hidden flex flex-col relative transition-all duration-300 border border-[#E2E8F0]">
        <div className="h-1.5 w-full bg-gradient-to-r from-[#00637C] via-[#4FD2FE] to-[#9F3800]" />

        <div className="p-6 sm:p-8 flex flex-col">
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
              Welcome Back
            </h1>
          </div>

          {generalError && (
            <div className="mb-4 p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-xs text-rose-600 flex items-start gap-2.5">
              <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
              <span className="font-medium leading-relaxed">
                {generalError}
              </span>
            </div>
          )}

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
                    if (fieldErrors.email) {
                      setFieldErrors((prev) => ({
                        ...prev,
                        email: undefined,
                      }));
                    }
                    if (generalError) setGeneralError(null);
                  }}
                  placeholder="gopher@developer.dev"
                  className={`w-full bg-white text-[#131B2A] text-sm pl-10 pr-4 py-2.5 rounded-lg border shadow-2xs focus:outline-none focus:ring-2 focus:ring-[#007D9C] placeholder-[#6F797E]/70 transition-all ${
                    fieldErrors.email
                      ? "border-rose-500 bg-rose-50/20"
                      : "border-[#BEC8CD]"
                  }`}
                />
              </div>
              {fieldErrors.email && (
                <p className="text-[11px] font-medium text-rose-600 flex items-center gap-1 mt-0.5">
                  <AlertCircle className="w-3 h-3 shrink-0" />
                  <span>{fieldErrors.email}</span>
                </p>
              )}
            </div>

            <div className="flex flex-col gap-1.5">
              <div className="flex items-center justify-between">
                <label className="text-xs font-bold text-[#131B2A] flex items-center gap-1">
                  <span>Password</span>
                  <span className="text-rose-600">*</span>
                </label>
                <Link
                  href="/forgot-password"
                  tabIndex={-1}
                  className="text-xs font-bold text-[#00637C] hover:underline transition-colors"
                >
                  Forgot password?
                </Link>
              </div>
              <div className="relative flex items-center">
                <LockIcon className="w-5 h-5 absolute left-3 text-[#6F797E] pointer-events-none" />
                <input
                  type={showPassword ? "text" : "password"}
                  required
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    if (fieldErrors.password) {
                      setFieldErrors((prev) => ({
                        ...prev,
                        password: undefined,
                      }));
                    }
                    if (generalError) setGeneralError(null);
                  }}
                  placeholder="************"
                  className={`w-full bg-white text-[#131B2A] text-sm pl-10 pr-11 py-2.5 rounded-lg border shadow-2xs focus:outline-none focus:ring-2 focus:ring-[#007D9C] placeholder-[#6F797E]/70 transition-all ${
                    fieldErrors.password
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
              {fieldErrors.password && (
                <p className="text-[11px] font-medium text-rose-600 flex items-center gap-1 mt-0.5">
                  <AlertCircle className="w-3 h-3 shrink-0" />
                  <span>{fieldErrors.password}</span>
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
                  <span>Authenticating...</span>
                </>
              ) : (
                <>
                  <span>Sign In to GOshop</span>
                  <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                </>
              )}
            </button>
          </form>

          <div className="relative my-6 flex items-center justify-center">
            <div className="w-full h-px bg-[#E2E7FF]" />
            <span className="absolute px-3 bg-white font-bold text-[11px] text-[#6F797E] tracking-wider uppercase">
              OR CONTINUE WITH
            </span>
          </div>

          <GoogleLoginButton
            text="signin_with"
            onError={(msg) => setGeneralError(msg)}
          />

          <div className="mt-6 text-center text-xs text-[#3E484D]">
            <span>Don&apos;t have an account yet? </span>
            <Link
              href="/register"
              className="font-bold text-[#00637C] hover:underline transition-colors"
            >
              Register here
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
