import React from "react";
import { AlertCircle, type LucideIcon } from "lucide-react";

export function AuthCard({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
}) {
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
              {title}
            </h1>
            {description && (
              <p className="mt-2 text-xs leading-relaxed text-[#3E484D] max-w-[320px]">
                {description}
              </p>
            )}
          </div>

          {children}
        </div>
      </div>
    </div>
  );
}

export function AuthError({ message }: { message: string }) {
  return (
    <div className="mb-4 p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-xs text-rose-600 flex items-start gap-2.5">
      <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
      <span className="font-medium leading-relaxed">{message}</span>
    </div>
  );
}

export function AuthField({
  id,
  label,
  icon: Icon,
  error,
  required = false,
  rightControl,
  className,
  ...inputProps
}: React.InputHTMLAttributes<HTMLInputElement> & {
  id: string;
  label: string;
  icon: LucideIcon;
  error?: string;
  required?: boolean;
  rightControl?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label
        htmlFor={id}
        className="text-xs font-bold text-[#131B2A] flex items-center gap-1"
      >
        <span>{label}</span>
        {required && <span className="text-rose-600">*</span>}
      </label>
      <div className="relative flex items-center">
        <Icon className="w-5 h-5 absolute left-3 text-[#6F797E] pointer-events-none" />
        <input
          id={id}
          required={required}
          className={`w-full bg-white text-[#131B2A] text-sm pl-10 ${
            rightControl ? "pr-11" : "pr-4"
          } py-2.5 rounded-lg border shadow-2xs focus:outline-none focus:ring-2 focus:ring-[#007D9C] placeholder-[#6F797E]/70 transition-all ${
            error ? "border-rose-500 bg-rose-50/20" : "border-[#BEC8CD]"
          } ${className ?? ""}`}
          {...inputProps}
        />
        {rightControl}
      </div>
      {error && (
        <p className="text-[11px] font-medium text-rose-600 flex items-center gap-1 mt-0.5">
          <AlertCircle className="w-3 h-3 shrink-0" />
          <span>{error}</span>
        </p>
      )}
    </div>
  );
}

export function AuthDivider({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative my-6 flex items-center justify-center">
      <div className="w-full h-px bg-[#E2E7FF]" />
      <span className="absolute px-3 bg-white font-bold text-[11px] text-[#6F797E] tracking-wider uppercase">
        {children}
      </span>
    </div>
  );
}

export function AuthSubmitButton({
  loading,
  loadingContent,
  children,
}: {
  loading: boolean;
  loadingContent: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <button
      type="submit"
      disabled={loading}
      className="w-full mt-1 py-3 px-4 rounded-lg bg-[#00637C] hover:bg-[#004D62] text-white text-xs font-bold flex items-center justify-center gap-2 shadow-md shadow-[#00637C]/25 transition-all group cursor-pointer disabled:opacity-50"
      style={{
        fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
      }}
    >
      {loading ? loadingContent : children}
    </button>
  );
}
