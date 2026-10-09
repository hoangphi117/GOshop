"use client";

import React, { useState, useEffect, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { z } from "zod";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  CheckCircle2,
  Eye,
  EyeOff,
  KeyRound,
  Loader2,
  Lock,
  Mail,
  RefreshCw,
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
import { useAuthStore } from "@/stores/auth-store";
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
  const { setTokens, setUser } = useAuthStore();

  const [step, setStep] = useState<"form" | "otp">("form");

  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [resending, setResending] = useState(false);
  const [generalError, setGeneralError] = useState<string | null>(null);

  // OTP 6-digit state
  const [otpDigits, setOtpDigits] = useState<string[]>([
    "",
    "",
    "",
    "",
    "",
    "",
  ]);
  const [resendTimer, setResendTimer] = useState(60);
  const otpInputRefs = useRef<(HTMLInputElement | null)[]>([]);

  const [fieldErrors, setFieldErrors] = useState<
    Partial<Record<keyof RegisterFormData, string>>
  >({});

  // 60-second countdown timer when in OTP step
  useEffect(() => {
    let timer: NodeJS.Timeout | null = null;
    if (step === "otp" && resendTimer > 0) {
      timer = setInterval(() => {
        setResendTimer((prev) => (prev > 0 ? prev - 1 : 0));
      }, 1000);
    }
    return () => {
      if (timer) clearInterval(timer);
    };
  }, [step, resendTimer]);

  // Focus first OTP input when step changes to 'otp'
  useEffect(() => {
    if (step === "otp") {
      setTimeout(() => {
        otpInputRefs.current[0]?.focus();
      }, 150);
    }
  }, [step]);

  // Real-time checks for requirements panel
  const isNameValid = fullName.trim().length >= 2;
  const isMinLength = password.length >= 6;
  const hasUppercase = /[A-Z]/.test(password);
  const hasLowercase = /[a-z]/.test(password);
  const hasNumber = /[0-9]/.test(password);
  const hasSpecialChar = /[!@#$%^&*(),.?":{}|<>]/.test(password);
  const isConfirmMatch =
    confirmPassword.length > 0 && confirmPassword === password;

  // Handle Step 1: Send OTP
  const handleSendOTP = async (e: React.FormEvent) => {
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
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/register-otp/send`, {
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
          data?.message ||
            "Failed to send verification code. Please try again.",
        );
      }

      toast.success("Verification code sent to your email!");
      setStep("otp");
      setResendTimer(60);
      setOtpDigits(["", "", "", "", "", ""]);
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "An error occurred while sending verification code.";
      setGeneralError(message);
    } finally {
      setLoading(false);
    }
  };

  // Handle OTP digit changes
  const handleOtpChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return;

    const newDigits = [...otpDigits];
    // In case user pastes or types a single digit
    const char = value.slice(-1);
    newDigits[index] = char;
    setOtpDigits(newDigits);
    if (generalError) setGeneralError(null);

    // Auto focus next input
    if (char && index < 5) {
      otpInputRefs.current[index + 1]?.focus();
    }
  };

  // Handle KeyDown on OTP inputs (Backspace & Arrows)
  const handleOtpKeyDown = (
    index: number,
    e: React.KeyboardEvent<HTMLInputElement>,
  ) => {
    if (e.key === "Backspace" && !otpDigits[index] && index > 0) {
      otpInputRefs.current[index - 1]?.focus();
    } else if (e.key === "ArrowLeft" && index > 0) {
      otpInputRefs.current[index - 1]?.focus();
    } else if (e.key === "ArrowRight" && index < 5) {
      otpInputRefs.current[index + 1]?.focus();
    }
  };

  // Handle Paste event on OTP boxes
  const handleOtpPaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    const pastedData = e.clipboardData.getData("text/plain").trim();
    if (!/^\d+$/.test(pastedData)) return;

    const digits = pastedData.slice(0, 6).split("");
    const newDigits = [...otpDigits];
    digits.forEach((d, i) => {
      if (i < 6) newDigits[i] = d;
    });
    setOtpDigits(newDigits);
    if (generalError) setGeneralError(null);

    const nextIndex = Math.min(digits.length, 5);
    otpInputRefs.current[nextIndex]?.focus();
  };

  // Handle Step 2: Verify OTP
  const handleVerifyOTP = async (e: React.FormEvent) => {
    e.preventDefault();
    setGeneralError(null);

    const otp = otpDigits.join("");
    if (otp.length !== 6) {
      setGeneralError("Please enter the complete 6-digit verification code.");
      return;
    }

    setLoading(true);

    try {
      const res = await fetch(
        `${API_BASE_URL}/api/v1/auth/register-otp/verify`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-API-KEY": API_KEY,
          },
          body: JSON.stringify({
            email: email.trim(),
            otp,
          }),
        },
      );

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          data?.message || "Invalid verification code. Please try again.",
        );
      }

      const accessToken =
        data.data?.accessToken || data.data?.access_token || "";
      const refreshToken =
        data.data?.refreshToken || data.data?.refresh_token || "";

      if (accessToken) {
        setTokens(accessToken, refreshToken);
        setUser({
          email: email.trim(),
          fullName: fullName.trim(),
        });
      }

      toast.success("Account verified successfully! Welcome to GOshop.");

      const redirectUrl =
        typeof window !== "undefined"
          ? new URLSearchParams(window.location.search).get("redirect") || "/"
          : "/";
      router.push(redirectUrl);
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "An error occurred during verification.";
      setGeneralError(message);
    } finally {
      setLoading(false);
    }
  };

  // Handle Resend OTP
  const handleResendOTP = async () => {
    if (resendTimer > 0 || resending) return;
    setGeneralError(null);
    setResending(true);

    try {
      const res = await fetch(
        `${API_BASE_URL}/api/v1/auth/register-otp/resend`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-API-KEY": API_KEY,
          },
          body: JSON.stringify({
            email: email.trim(),
          }),
        },
      );

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          data?.message || "Failed to resend code. Please try again later.",
        );
      }

      toast.success("New verification code sent to your email!");
      setResendTimer(60);
      setOtpDigits(["", "", "", "", "", ""]);
      otpInputRefs.current[0]?.focus();
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to resend verification code.";
      setGeneralError(message);
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="flex flex-col w-full items-center justify-center relative py-4 sm:py-8 px-4">
      <div className="w-full max-w-4xl bg-white rounded-2xl shadow-xl shadow-indigo-100/40 overflow-hidden flex flex-col relative transition-all duration-300 border border-[#E2E8F0]">
        {/* Top Decorative Line Stripe */}
        <div className="h-1.5 w-full bg-gradient-to-r from-[#00637C] via-[#4FD2FE] to-[#9F3800]" />

        <div className="grid grid-cols-1 lg:grid-cols-12 items-stretch">
          {/* SECTION 1: MAIN FORM (LEFT - 7 COLS) */}
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
                {step === "form" ? "Create Your Account" : "Verify Your Email"}
              </h1>

              {step === "otp" && (
                <p className="text-xs text-[#6F797E] mt-1.5 max-w-md">
                  We sent a 6-digit verification code to{" "}
                  <span className="font-semibold text-[#131B2A]">{email}</span>
                </p>
              )}
            </div>

            {generalError && <AuthError message={generalError} />}

            {step === "form" ? (
              /* STEP 1: REGISTRATION DETAILS FORM */
              <>
                <form
                  onSubmit={handleSendOTP}
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
                        onClick={() =>
                          setShowConfirmPassword(!showConfirmPassword)
                        }
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
                        <span>Sending verification code...</span>
                      </>
                    }
                  >
                    <span>Continue to Verification</span>
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
              </>
            ) : (
              /* STEP 2: OTP VERIFICATION FORM */
              <div className="flex flex-col gap-6">
                <form
                  onSubmit={handleVerifyOTP}
                  className="flex flex-col gap-5"
                >
                  <div className="flex flex-col items-center gap-3">
                    <label className="text-xs font-bold text-[#131B2A] uppercase tracking-wider">
                      Enter 6-Digit Code
                    </label>

                    {/* 6 OTP Boxes */}
                    <div
                      className="flex items-center justify-center gap-2 sm:gap-3"
                      onPaste={handleOtpPaste}
                    >
                      {otpDigits.map((digit, idx) => (
                        <input
                          key={idx}
                          ref={(el) => {
                            otpInputRefs.current[idx] = el;
                          }}
                          type="text"
                          inputMode="numeric"
                          pattern="[0-9]*"
                          maxLength={1}
                          value={digit}
                          onChange={(e) => handleOtpChange(idx, e.target.value)}
                          onKeyDown={(e) => handleOtpKeyDown(idx, e)}
                          className="w-11 h-13 sm:w-12 sm:h-14 text-center text-xl font-bold text-[#131B2A] bg-white rounded-xl border border-[#BEC8CD] shadow-xs focus:border-[#007D9C] focus:ring-2 focus:ring-[#007D9C]/20 focus:outline-none transition-all"
                        />
                      ))}
                    </div>
                  </div>

                  <AuthSubmitButton
                    loading={loading}
                    loadingContent={
                      <>
                        <Loader2 className="w-4 h-4 animate-spin" />
                        <span>Verifying account...</span>
                      </>
                    }
                  >
                    <CheckCircle2 className="w-4 h-4" />
                    <span>Verify & Complete Registration</span>
                  </AuthSubmitButton>
                </form>

                {/* Resend & Back controls */}
                <div className="flex flex-col sm:flex-row items-center justify-between gap-3 pt-4 border-t border-[#E2E8F0] text-xs">
                  <button
                    type="button"
                    onClick={() => {
                      setStep("form");
                      setGeneralError(null);
                    }}
                    className="flex items-center gap-1.5 text-[#6F797E] hover:text-[#131B2A] font-medium transition-colors cursor-pointer"
                  >
                    <ArrowLeft className="w-4 h-4" />
                    <span>Change email or details</span>
                  </button>

                  <div className="flex items-center gap-1.5">
                    <span className="text-[#6F797E]">
                      Didn&apos;t get code?
                    </span>
                    <button
                      type="button"
                      disabled={resendTimer > 0 || resending}
                      onClick={handleResendOTP}
                      className={`font-bold transition-colors flex items-center gap-1 cursor-pointer ${
                        resendTimer > 0 || resending
                          ? "text-[#6F797E] opacity-60 cursor-not-allowed"
                          : "text-[#00637C] hover:underline"
                      }`}
                    >
                      {resending && (
                        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                      )}
                      <span>
                        {resendTimer > 0
                          ? `Resend in ${resendTimer}s`
                          : "Resend code"}
                      </span>
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* SECTION 2: REQUIREMENTS / OTP INFO PANEL (RIGHT - 5 COLS) */}
          <div className="lg:col-span-5 p-6 sm:p-8 flex flex-col justify-between bg-[#F8FAFC] border-t lg:border-t-0 lg:border-l border-[#E2E8F0]">
            {step === "form" ? (
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
                    <span className="text-[13px] font-bold text-[#3E484D] uppercase tracking-wider block">
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
                    <span className="text-[13px] font-bold text-[#3E484D] uppercase tracking-wider block">
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
                    <span className="text-[13px] font-bold text-[#3E484D] uppercase tracking-wider block">
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
            ) : (
              /* OTP INSTRUCTIONS PANEL */
              <div className="flex flex-col justify-between h-full">
                <div>
                  <div className="flex items-center gap-2 pb-3 border-b border-[#E2E8F0]">
                    <KeyRound className="w-4 h-4 text-[#00637C]" />
                    <h3
                      className="text-xs font-bold text-[#131B2A] uppercase tracking-wide"
                      style={{
                        fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                      }}
                    >
                      Email Verification
                    </h3>
                  </div>

                  <div className="space-y-4 pt-4 text-xs text-[#3E484D]">
                    <p className="leading-relaxed">
                      To ensure security and protect your account from
                      unauthorized access, we require email verification.
                    </p>

                    <div className="p-3.5 bg-blue-50/60 rounded-xl border border-blue-100 text-blue-900 space-y-2">
                      <div className="font-bold flex items-center gap-1.5 text-blue-950">
                        <span>Verification Tips:</span>
                      </div>
                      <ul className="list-disc list-inside space-y-1 text-[13px] text-blue-800">
                        <li>The code expires in 5 minutes.</li>
                        <li>Check your spam or junk folder if not seen.</li>
                        <li>You can request a new code every 60 seconds.</li>
                      </ul>
                    </div>
                  </div>
                </div>

                <div className="pt-6 border-t border-[#E2E8F0] mt-6">
                  <p className="text-[13px] text-[#6F797E] text-center">
                    Need help? Contact support at{" "}
                    <span className="text-[#00637C] font-semibold">
                      support@goshop.com
                    </span>
                  </p>
                </div>
              </div>
            )}
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
