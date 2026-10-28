"use client";

import React, { useState, useRef } from "react";
import { directUpload } from "@goshop/api-client";
import { authFetch } from "@/lib/api";
import { extractErrorMessage } from "@/lib/utils";
import {
  X,
  RotateCcw,
  Upload,
  Film,
  Trash2,
  Loader2,
  AlertCircle,
  Building2,
} from "lucide-react";
import { toast } from "sonner";

interface RequestReturnModalProps {
  isOpen: boolean;
  onClose: () => void;
  orderUuid: string;
  orderNumber?: string;
  paymentMethod?: string;
  totalAmount?: number;
  onSuccess?: () => void;
}

const COMMON_REASONS = [
  "Damaged or defective item upon arrival",
  "Incorrect product or wrong variant received",
  "Item does not match website description",
  "Missing parts, accessories, or free gifts",
  "Quality issue / Product is not functioning properly",
  "Other reason",
];

const VIETNAM_BANKS = [
  "Vietcombank",
  "Techcombank",
  "MB Bank (Military Bank)",
  "ACB (Asia Commercial Bank)",
  "VPBank",
  "BIDV",
  "VietinBank",
  "TPBank",
  "Sacombank",
  "VIB",
  "HDBank",
  "MSB (Maritime Bank)",
  "SHB",
  "SeABank",
  "Agribank",
  "MoMo E-Wallet Account",
  "Other Bank",
];

export function RequestReturnModal({
  isOpen,
  onClose,
  orderUuid,
  orderNumber,
  onSuccess,
}: RequestReturnModalProps) {
  const [selectedReasonPrefix, setSelectedReasonPrefix] = useState(
    COMMON_REASONS[0],
  );
  const [detailedReason, setDetailedReason] = useState("");
  const [bankName, setBankName] = useState("");
  const [bankAccount, setBankAccount] = useState("");
  const [bankAccountName, setBankAccountName] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [previews, setPreviews] = useState<
    Array<{ url: string; isVideo: boolean; file: File }>
  >([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [fieldErrors, setFieldErrors] = useState({
    detailedReason: "",
    bankName: "",
    bankAccount: "",
    bankAccountName: "",
    proofs: "",
  });

  const validateForm = () => {
    const errors = {
      detailedReason: "",
      bankName: "",
      bankAccount: "",
      bankAccountName: "",
      proofs: "",
    };

    const trimmedReason = detailedReason.trim();
    const trimmedBankName = bankName.trim();
    const trimmedBankAccount = bankAccount.trim();
    const trimmedBankAccountName = bankAccountName.trim();

    if (!trimmedReason) {
      errors.detailedReason = "Please provide a detailed explanation.";
    } else if (trimmedReason.length < 10) {
      errors.detailedReason =
        "Detailed explanation must be at least 10 characters.";
    }

    if (!trimmedBankName) {
      errors.bankName = "Please enter your bank name.";
    }

    if (!trimmedBankAccount) {
      errors.bankAccount = "Please enter your account number.";
    }

    if (!trimmedBankAccountName) {
      errors.bankAccountName = "Please enter the account holder name.";
    }

    if (previews.length === 0) {
      errors.proofs = "Please upload at least 1 photo or video as proof.";
    }

    setFieldErrors(errors);

    return !Object.values(errors).some(Boolean);
  };

  if (!isOpen) return null;

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(e.target.files || []);
    if (!selected.length) return;

    let currentImages = previews.filter((p) => !p.isVideo).length;
    let currentVideos = previews.filter((p) => p.isVideo).length;

    const validNewFiles: File[] = [];
    const newPreviews: Array<{ url: string; isVideo: boolean; file: File }> =
      [];

    for (const file of selected) {
      const isImg = file.type.startsWith("image/");
      const isVid = file.type.startsWith("video/");

      if (!isImg && !isVid) {
        toast.error(`Unsupported file type: ${file.name}`);
        continue;
      }

      if (file.size > 25 * 1024 * 1024) {
        toast.error(`File ${file.name} exceeds 25MB limit.`);
        continue;
      }

      if (isImg) {
        if (currentImages >= 5) {
          toast.warning("Maximum 5 proof images allowed.");
          continue;
        }
        currentImages++;
      } else if (isVid) {
        if (currentVideos >= 1) {
          toast.warning("Maximum 1 video allowed.");
          continue;
        }
        currentVideos++;
      }

      validNewFiles.push(file);
      newPreviews.push({
        url: URL.createObjectURL(file),
        isVideo: isVid,
        file,
      });
    }

    setFiles((prev) => [...prev, ...validNewFiles]);
    setPreviews((prev) => [...prev, ...newPreviews]);

    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleRemoveFile = (index: number) => {
    const target = previews[index];
    if (target?.url) {
      URL.revokeObjectURL(target.url);
    }
    setPreviews((prev) => prev.filter((_, i) => i !== index));
    setFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);

    const fullReason =
      selectedReasonPrefix === "Other reason"
        ? detailedReason.trim()
        : detailedReason.trim()
          ? `${selectedReasonPrefix}: ${detailedReason.trim()}`
          : selectedReasonPrefix;

    if (!validateForm()) {
      return;
    }

    if (!bankName.trim()) {
      setErrorMsg("Please enter or select your refund bank name.");
      return;
    }

    if (!bankAccount.trim()) {
      setErrorMsg("Please enter your refund bank account number.");
      return;
    }

    if (!bankAccountName.trim()) {
      setErrorMsg("Please enter the beneficiary account holder name.");
      return;
    }

    if (files.length === 0) {
      setErrorMsg(
        "Please upload at least 1 photo or video proof of the item defect or issue.",
      );
      return;
    }

    setIsSubmitting(true);

    try {
      // 1. Upload files to /api/v1/upload (folder='returns') if any
      const uploadedImageUrls: string[] = [];
      let uploadedVideoUrl: string | undefined = undefined;

      if (files.length > 0) {
        for (const file of files) {
          let fileUrl: string;
          try {
            const uploaded = await directUpload(file, "returns", authFetch);
            fileUrl = uploaded.url;
          } catch (err) {
            const msg = `Failed to upload "${file.name}". ${(err as Error).message}`;
            setErrorMsg(msg);
            toast.error(msg);
            setIsSubmitting(false);
            return;
          }

          if (file.type.startsWith("video/")) {
            uploadedVideoUrl = fileUrl;
          } else {
            uploadedImageUrls.push(fileUrl);
          }
        }
      }

      // 2. Submit pure JSON return request
      const res = await authFetch(`/api/v1/orders/${orderUuid}/return`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          reason: fullReason,
          bank_name: bankName.trim() || undefined,
          bank_account: bankAccount.trim() || undefined,
          bank_account_name: bankAccountName.trim().toUpperCase() || undefined,
          images: uploadedImageUrls,
          video_url: uploadedVideoUrl,
        }),
      });

      if (!res.ok) {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(
          errJson,
          "Failed to submit return request.",
        );
        setErrorMsg(msg);
        toast.error(msg);
        setIsSubmitting(false);
        return;
      }

      toast.success(
        "Return & refund request submitted successfully! Our support team will review it shortly.",
      );
      onSuccess?.();
      onClose();
    } catch {
      setErrorMsg("Network error. Please try again.");
      toast.error("Network error. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 overflow-y-auto font-body">
      <div className="w-full max-w-xl bg-white rounded-md border border-[#E2E8F0] shadow-lg max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-start justify-between gap-4 px-6 py-5 border-b border-[#E2E8F0]">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-md bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/30 flex items-center justify-center shrink-0">
              <RotateCcw className="w-5 h-5" />
            </div>

            <div>
              <h2 className="text-base sm:text-lg font-bold text-[#0F172A] font-heading">
                Request Return & Refund
              </h2>

              <p className="text-xs text-[#64748B] mt-0.5">
                Order #{orderNumber || orderUuid.slice(0, 8).toUpperCase()}
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 rounded-md flex items-center justify-center text-[#64748B] hover:bg-[#F8FAFC] hover:text-[#0F172A] border border-[#E2E8F0] shrink-0 transition-colors cursor-pointer"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* General Error */}
        {errorMsg && (
          <div className="mx-6 mt-4 p-3 rounded-md bg-[#FEF2F2] border border-[#FECACA] text-xs text-[#DC2626] flex items-start gap-2.5">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />

            <span>{errorMsg}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} noValidate className="p-6 space-y-5">
          {/* ====================================================== */}
          {/* Reason */}
          {/* ====================================================== */}

          <div className="space-y-1.5">
            <label className="text-xs font-semibold text-[#0F172A]">
              Reason for Return <span className="text-[#DC2626]">*</span>
            </label>

            <select
              value={selectedReasonPrefix}
              onChange={(e) => setSelectedReasonPrefix(e.target.value)}
              className="w-full h-9 px-3 rounded-md border border-[#E2E8F0] bg-white text-xs font-medium text-[#0F172A] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#00ADD8]/30"
            >
              {COMMON_REASONS.map((r) => (
                <option key={r} value={r}>
                  {r}
                </option>
              ))}
            </select>
          </div>

          {/* ====================================================== */}
          {/* Detailed Explanation */}
          {/* ====================================================== */}

          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-3">
              <label className="text-xs font-semibold text-[#0F172A]">
                Detailed Explanation <span className="text-[#DC2626]">*</span>
              </label>

              <span className="text-[11px] text-[#64748B]">
                {detailedReason.length}/1000
              </span>
            </div>

            <textarea
              rows={3}
              value={detailedReason}
              onChange={(e) => {
                setDetailedReason(e.target.value);

                if (fieldErrors.detailedReason) {
                  setFieldErrors((prev) => ({
                    ...prev,
                    detailedReason: "",
                  }));
                }
              }}
              placeholder="Please describe the item defect or reason for return in detail..."
              maxLength={1000}
              className={`w-full px-3 py-2 rounded-md border bg-white text-xs text-[#0F172A] placeholder-[#64748B] focus:outline-none resize-none ${
                fieldErrors.detailedReason
                  ? "border-[#DC2626] focus:border-[#DC2626] focus:ring-1 focus:ring-[#DC2626]/20"
                  : "border-[#E2E8F0] focus:border-[#007D9C] focus:ring-1 focus:ring-[#00ADD8]/30"
              }`}
            />

            {fieldErrors.detailedReason ? (
              <p className="text-[11px] text-[#DC2626] flex items-center gap-1">
                <AlertCircle className="w-3 h-3 shrink-0" />
                {fieldErrors.detailedReason}
              </p>
            ) : (
              <p className="text-[11px] text-[#64748B]">
                Minimum 10 characters required.
              </p>
            )}
          </div>

          {/* ====================================================== */}
          {/* Refund Bank Information */}
          {/* ====================================================== */}

          <div className="rounded-md border border-[#E2E8F0] bg-[#F8FAFC] p-4 space-y-3">
            {/* Section Header */}
            <div className="flex items-start gap-2.5">
              <div className="w-7 h-7 rounded-md bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/30 flex items-center justify-center shrink-0 mt-0.5">
                <Building2 className="w-4 h-4" />
              </div>

              <div>
                <h3 className="text-xs sm:text-sm font-bold text-[#0F172A] font-heading">
                  Refund Bank Account Details{" "}
                  <span className="text-[#DC2626]">*</span>
                </h3>

                <p className="text-[11px] text-[#64748B] mt-0.5 leading-relaxed">
                  Required. Enter your bank details so that the refund payout
                  can be credited once approved.
                </p>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
              {/* ================================================== */}
              {/* Bank Name */}
              {/* ================================================== */}

              <div className="sm:col-span-2 space-y-1">
                <label className="text-xs font-semibold text-[#0F172A]">
                  Bank Name <span className="text-[#DC2626]">*</span>
                </label>

                <input
                  type="text"
                  list="vietnam-banks-list"
                  value={bankName}
                  onChange={(e) => {
                    setBankName(e.target.value);

                    if (fieldErrors.bankName) {
                      setFieldErrors((prev) => ({
                        ...prev,
                        bankName: "",
                      }));
                    }
                  }}
                  placeholder="e.g. Vietcombank, Techcombank, MB Bank"
                  className={`w-full h-9 px-3 rounded-md border bg-white text-xs text-[#0F172A] placeholder-[#64748B] focus:outline-none ${
                    fieldErrors.bankName
                      ? "border-[#DC2626] focus:border-[#DC2626] focus:ring-1 focus:ring-[#DC2626]/20"
                      : "border-[#E2E8F0] focus:border-[#007D9C] focus:ring-1 focus:ring-[#00ADD8]/30"
                  }`}
                />

                {fieldErrors.bankName && (
                  <p className="text-[11px] text-[#DC2626] flex items-center gap-1">
                    <AlertCircle className="w-3 h-3 shrink-0" />
                    {fieldErrors.bankName}
                  </p>
                )}

                <datalist id="vietnam-banks-list">
                  {VIETNAM_BANKS.map((b) => (
                    <option key={b} value={b} />
                  ))}
                </datalist>
              </div>

              {/* ================================================== */}
              {/* Account Number */}
              {/* ================================================== */}

              <div className="space-y-1">
                <label className="text-xs font-semibold text-[#0F172A]">
                  Account Number <span className="text-[#DC2626]">*</span>
                </label>

                <input
                  type="text"
                  value={bankAccount}
                  onChange={(e) => {
                    setBankAccount(e.target.value.replace(/[^0-9A-Za-z]/g, ""));

                    if (fieldErrors.bankAccount) {
                      setFieldErrors((prev) => ({
                        ...prev,
                        bankAccount: "",
                      }));
                    }
                  }}
                  placeholder="e.g. 012345678910"
                  className={`w-full h-9 px-3 rounded-md border bg-white text-xs font-mono text-[#0F172A] placeholder-[#64748B] focus:outline-none ${
                    fieldErrors.bankAccount
                      ? "border-[#DC2626] focus:border-[#DC2626] focus:ring-1 focus:ring-[#DC2626]/20"
                      : "border-[#E2E8F0] focus:border-[#007D9C] focus:ring-1 focus:ring-[#00ADD8]/30"
                  }`}
                />

                {fieldErrors.bankAccount && (
                  <p className="text-[11px] text-[#DC2626] flex items-center gap-1">
                    <AlertCircle className="w-3 h-3 shrink-0" />
                    {fieldErrors.bankAccount}
                  </p>
                )}
              </div>

              {/* ================================================== */}
              {/* Account Holder */}
              {/* ================================================== */}

              <div className="space-y-1">
                <label className="text-xs font-semibold text-[#0F172A]">
                  Account Holder Name <span className="text-[#DC2626]">*</span>
                </label>

                <input
                  type="text"
                  value={bankAccountName}
                  onChange={(e) => {
                    setBankAccountName(e.target.value);

                    if (fieldErrors.bankAccountName) {
                      setFieldErrors((prev) => ({
                        ...prev,
                        bankAccountName: "",
                      }));
                    }
                  }}
                  placeholder="e.g. NGUYEN VAN A"
                  className={`w-full h-9 px-3 rounded-md border bg-white text-xs uppercase text-[#0F172A] placeholder-[#64748B] focus:outline-none ${
                    fieldErrors.bankAccountName
                      ? "border-[#DC2626] focus:border-[#DC2626] focus:ring-1 focus:ring-[#DC2626]/20"
                      : "border-[#E2E8F0] focus:border-[#007D9C] focus:ring-1 focus:ring-[#00ADD8]/30"
                  }`}
                />

                {fieldErrors.bankAccountName && (
                  <p className="text-[11px] text-[#DC2626] flex items-center gap-1">
                    <AlertCircle className="w-3 h-3 shrink-0" />
                    {fieldErrors.bankAccountName}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* ====================================================== */}
          {/* Proof Upload */}
          {/* ====================================================== */}

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <div>
                <label className="text-xs font-semibold text-[#0F172A]">
                  Proof Photos & Video <span className="text-[#DC2626]">*</span>
                </label>

                <p className="text-[11px] text-[#64748B] mt-0.5">
                  At least 1 photo or video required (Up to 5 images and 1
                  video)
                </p>
              </div>

              <span
                className={`text-[11px] font-semibold ${
                  previews.length === 0 ? "text-[#DC2626]" : "text-[#007D9C]"
                }`}
              >
                {previews.length}/6 {previews.length === 0 ? "(Required)" : ""}
              </span>
            </div>

            <div
              className={`grid grid-cols-4 sm:grid-cols-6 gap-2.5 p-1 rounded-md ${
                fieldErrors.proofs
                  ? "border border-[#DC2626]/40 bg-[#FEF2F2]/40"
                  : ""
              }`}
            >
              {previews.map((item, idx) => (
                <div
                  key={idx}
                  className="relative group aspect-square rounded-md border border-[#E2E8F0] overflow-hidden bg-[#F8FAFC] flex items-center justify-center"
                >
                  {item.isVideo ? (
                    <div className="relative w-full h-full bg-[#0F172A] flex flex-col items-center justify-center text-white">
                      <Film className="w-5 h-5 text-[#00ADD8]" />

                      <span className="text-[10px] font-semibold mt-1">
                        Video
                      </span>
                    </div>
                  ) : (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={item.url}
                      alt={`Preview ${idx + 1}`}
                      className="w-full h-full object-cover"
                    />
                  )}

                  <button
                    type="button"
                    onClick={() => handleRemoveFile(idx)}
                    className="absolute top-1 right-1 w-5 h-5 rounded-md bg-[#DC2626] text-white flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                    title="Remove file"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              ))}

              {previews.length < 6 && (
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  className="aspect-square rounded-md border border-dashed border-[#E2E8F0] bg-[#F8FAFC] hover:border-[#00ADD8] hover:bg-[#E6F7FB] text-[#64748B] hover:text-[#007D9C] flex flex-col items-center justify-center gap-1 cursor-pointer transition-colors"
                >
                  <Upload className="w-4 h-4" />

                  <span className="text-[10px] font-semibold">Upload</span>
                </button>
              )}
            </div>

            {/* Inline Proof Error */}
            {fieldErrors.proofs && (
              <p className="text-[11px] text-[#DC2626] flex items-center gap-1">
                <AlertCircle className="w-3 h-3 shrink-0" />
                {fieldErrors.proofs}
              </p>
            )}

            <input
              ref={fileInputRef}
              type="file"
              accept="image/*,video/*"
              multiple
              onChange={(e) => {
                handleFileChange(e);

                if (fieldErrors.proofs) {
                  setFieldErrors((prev) => ({
                    ...prev,
                    proofs: "",
                  }));
                }
              }}
              className="hidden"
            />
          </div>

          {/* ====================================================== */}
          {/* Actions */}
          {/* ====================================================== */}

          <div className="flex gap-2.5 pt-4 border-t border-[#E2E8F0]">
            <button
              type="button"
              disabled={isSubmitting}
              onClick={onClose}
              className="flex-1 h-9 rounded-md border border-[#E2E8F0] bg-white text-[#0F172A] text-xs font-semibold hover:bg-[#F8FAFC] disabled:opacity-50 cursor-pointer transition-colors"
            >
              Cancel
            </button>

            <button
              type="submit"
              disabled={isSubmitting}
              className="flex-1 h-9 rounded-md bg-[#007D9C] hover:bg-[#006782] text-white text-xs font-semibold flex items-center justify-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer transition-colors"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Submitting...</span>
                </>
              ) : (
                <>
                  <RotateCcw className="w-3.5 h-3.5" />
                  <span>Submit Return Request</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
