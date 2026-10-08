"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { z } from "zod";
import {
  ArrowRight,
  Check,
  Eye,
  EyeOff,
  Loader2,
  Lock,
  Mail,
  User,
  X,
} from "lucide-react";
import {
  AuthDivider,
  AuthError,
  AuthField,
  AuthSubmitButton,
} from "@/components/auth/auth-card";
import { GoogleLoginButton } from "@/components/auth/google-login-button";
import { API_BASE_URL, API_KEY } from "@/lib/api";
import { toast } from "sonner";

// Schema validation bằng Zod
const registerSchema = z
  .object({
    fullName: z
      .string()
      .trim()
      .min(1, "Please enter your full name")
      .min(2, "Full name must be at least 2 characters long"),
    email: z
      .string()
      .trim()
      .min(1, "Please enter your email address")
      .email("Please enter a valid email address"),
    password: z
      .string()
      .min(1, "Please enter a password")
      .min(6, "Password must be at least 6 characters long")
      .regex(/[A-Z]/, "Password must contain at least 1 uppercase letter")
      .regex(/[a-z]/, "Password must contain at least 1 lowercase letter")
      .regex(/[0-9]/, "Password must contain at least 1 number")
      .regex(
        /[!@#$%^&*(),.?":{}|<>]/,
        "Password must contain at least 1 special character",
      ),
    confirmPassword: z.string().min(1, "Please confirm your password"),
  })
  .refine((data) => data.password === data.confirmPassword, {
    message: "Passwords do not match",
    path: ["confirmPassword"],
  });

type RegisterFormData = z.infer<typeof registerSchema>;

export default function RegisterPage() {
  const router = useRouter();

  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [generalError, setGeneralError] = useState<string | null>(null);

  const [fieldErrors, setFieldErrors] = useState<
    Partial<Record<keyof RegisterFormData, string>>
  >({});

  // Real-time checks cho bảng yêu cầu phía bên phải
  const isNameValid = fullName.trim().length >= 2;
  const isMinLength = password.length >= 6;
  const hasUppercase = /[A-Z]/.test(password);
  const hasLowercase = /[a-z]/.test(password);
  const hasNumber = /[0-9]/.test(password);
  const hasSpecialChar = /[!@#$%^&*(),.?":{}|<>]/.test(password);
  const isConfirmMatch =
    confirmPassword.length > 0 && confirmPassword === password;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setGeneralError(null);
    setFieldErrors({});

    const result = registerSchema.safeParse({
      fullName,
      email,
      password,
      confirmPassword,
    });

    if (!result.success) {
      const formattedErrors: Partial<Record<keyof RegisterFormData, string>> =
        {};
      result.error.issues.forEach((issue) => {
        const fieldName = issue.path[0] as keyof RegisterFormData;
        if (!formattedErrors[fieldName]) {
          formattedErrors[fieldName] = issue.message;
        }
      });
      setFieldErrors(formattedErrors);
      return;
    }

    setLoading(true);

    try {
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/register`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-API-KEY": API_KEY,
        },
        body: JSON.stringify({
          email: email.trim(),
          password,
          name: fullName.trim(),
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          data?.message || "Registration failed. Please try again.",
        );
      }
      toast.success("Registration successful. Please log in.");
      router.push("/login");
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "An error occurred during registration.";
      setGeneralError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col w-full items-center justify-center relative py-4 sm:py-8 px-4">
      <div className="w-full max-w-4xl bg-white rounded-2xl shadow-xl shadow-indigo-100/40 overflow-hidden flex flex-col relative transition-all duration-300 border border-[#E2E8F0]">
        {/* Top Decorative Line Stripe */}
        <div className="h-1.5 w-full bg-gradient-to-r from-[#00637C] via-[#4FD2FE] to-[#9F3800]" />

        <div className="grid grid-cols-1 lg:grid-cols-12 items-stretch">
          {/* SECTION 1: MAIN REGISTRATION FORM (LEFT - 7 COLS) */}
          <div className="lg:col-span-7 p-6 sm:p-8 flex flex-col bg-white">
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
                className="text-2xl font-bold text-[#131B2A] mt-2"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                Create Your Account
              </h1>
            </div>

            {generalError && <AuthError message={generalError} />}

            <form
              onSubmit={handleSubmit}
              noValidate
              className="flex flex-col gap-4"
            >
              <AuthField
                id="fullName"
                label="Full Name"
                icon={User}
                required
                type="text"
                value={fullName}
                onChange={(e) => {
                  setFullName(e.target.value);
                  if (fieldErrors.fullName) {
                    setFieldErrors((prev) => ({
                      ...prev,
                      fullName: undefined,
                    }));
                  }
                  if (generalError) setGeneralError(null);
                }}
                placeholder="Alex Johnson"
                error={fieldErrors.fullName}
              />

              <AuthField
                id="email"
                label="Email Address"
                icon={Mail}
                required
                type="email"
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
                error={fieldErrors.email}
              />

              <AuthField
                id="password"
                label="Password"
                icon={Lock}
                required
                type={showPassword ? "text" : "password"}
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
                placeholder="Enter strong password"
                error={fieldErrors.password}
                rightControl={
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
                }
              />

              <AuthField
                id="confirmPassword"
                label="Confirm Password"
                icon={Lock}
                required
                type={showConfirmPassword ? "text" : "password"}
                value={confirmPassword}
                onChange={(e) => {
                  setConfirmPassword(e.target.value);
                  if (fieldErrors.confirmPassword) {
                    setFieldErrors((prev) => ({
                      ...prev,
                      confirmPassword: undefined,
                    }));
                  }
                  if (generalError) setGeneralError(null);
                }}
                placeholder="Re-enter your password"
                error={fieldErrors.confirmPassword}
                rightControl={
                  <button
                    type="button"
                    onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                    className="absolute right-3 p-1 text-[#6F797E] hover:text-[#131B2A] transition-colors rounded focus:outline-none cursor-pointer"
                  >
                    {showConfirmPassword ? (
                      <EyeOff className="w-5 h-5" />
                    ) : (
                      <Eye className="w-5 h-5" />
                    )}
                  </button>
                }
              />

              <AuthSubmitButton
                loading={loading}
                loadingContent={
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    <span>Creating account...</span>
                  </>
                }
              >
                <span>Create Account</span>
                <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
              </AuthSubmitButton>
            </form>

            <AuthDivider>OR SIGN UP WITH</AuthDivider>

            <GoogleLoginButton
              text="signup_with"
              onError={(msg) => setGeneralError(msg)}
            />

            <div className="mt-6 text-center text-xs text-[#3E484D]">
              <span>Already have an account? </span>
              <Link
                href="/login"
                className="font-bold text-[#00637C] hover:underline transition-colors"
              >
                Sign in
              </Link>
            </div>
          </div>

          {/* SECTION 2: INPUT REQUIREMENTS PANEL (RIGHT - 5 COLS) */}
          <div className="lg:col-span-5 p-6 sm:p-8 flex flex-col justify-between bg-[#F8FAFC] border-t lg:border-t-0 lg:border-l border-[#E2E8F0]">
            <div>
              <div className="flex items-center gap-2 pb-3 border-b border-[#E2E8F0]">
                <h3
                  className="text-xs font-bold text-[#131B2A] uppercase tracking-wide"
                  style={{
                    fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                  }}
                >
                  Input Requirements
                </h3>
              </div>

              <div className="space-y-4 pt-4">
                {/* Full Name Rules */}
                <div className="space-y-2">
                  <span className="text-[11px] font-bold text-[#3E484D] uppercase tracking-wider block">
                    Full Name
                  </span>
                  <RequirementItem
                    fulfilled={isNameValid}
                    isEmpty={fullName.length === 0}
                    label="At least 2 characters long"
                  />
                </div>

                {/* Password Rules */}
                <div className="space-y-2.5 pt-4 border-t border-[#E2E8F0]">
                  <span className="text-[11px] font-bold text-[#3E484D] uppercase tracking-wider block">
                    Password Requirements
                  </span>

                  <ul className="space-y-2.5 text-xs">
                    <RequirementItem
                      fulfilled={isMinLength}
                      isEmpty={password.length === 0}
                      label="At least 6 characters long"
                    />
                    <RequirementItem
                      fulfilled={hasUppercase}
                      isEmpty={password.length === 0}
                      label="At least 1 uppercase letter (A-Z)"
                    />
                    <RequirementItem
                      fulfilled={hasLowercase}
                      isEmpty={password.length === 0}
                      label="At least 1 lowercase letter (a-z)"
                    />
                    <RequirementItem
                      fulfilled={hasNumber}
                      isEmpty={password.length === 0}
                      label="At least 1 number (0-9)"
                    />
                    <RequirementItem
                      fulfilled={hasSpecialChar}
                      isEmpty={password.length === 0}
                      label="At least 1 special character (!@#$%...)"
                    />
                  </ul>
                </div>

                {/* Confirm Password Rule */}
                <div className="space-y-2 pt-4 border-t border-[#E2E8F0]">
                  <span className="text-[11px] font-bold text-[#3E484D] uppercase tracking-wider block">
                    Password Confirmation
                  </span>
                  <RequirementItem
                    fulfilled={isConfirmMatch}
                    isEmpty={confirmPassword.length === 0}
                    label="Passwords match"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function RequirementItem({
  fulfilled,
  isEmpty,
  label,
}: {
  fulfilled: boolean;
  isEmpty: boolean;
  label: string;
}) {
  return (
    <div className="flex items-center gap-2 text-xs">
      {isEmpty ? (
        <span className="w-4 h-4 rounded-full border border-[#BEC8CD] shrink-0" />
      ) : fulfilled ? (
        <Check className="w-4 h-4 text-emerald-600 shrink-0" />
      ) : (
        <X className="w-4 h-4 text-rose-500 shrink-0" />
      )}
      <span
        className={
          isEmpty
            ? "text-[#6F797E]"
            : fulfilled
              ? "text-emerald-700 font-medium"
              : "text-rose-600 font-medium"
        }
      >
        {label}
      </span>
    </div>
  );
}
