"use client";

import React, { useState, useEffect, useCallback } from "react";
import { authFetch } from "@/lib/api";
import { formatCurrency, formatDate, formatDateTime } from "@/lib/format";
import { extractErrorMessage } from "@/lib/utils";
import {
  X,
  RotateCcw,
  Truck,
  Building2,
  AlertCircle,
  CheckCircle2,
  Clock,
  XCircle,
  Loader2,
  Film,
  Send,
  Edit2,
  ShieldCheck,
} from "lucide-react";
import { toast } from "sonner";

export interface TrackingEvent {
  event_id: number;
  status: string;
  status_text: string;
  note?: string;
  source: string;
  created_at: string;
}

export interface ReturnDetailData {
  return_uuid: string;
  order_uuid: string;
  order_number: string;
  reason: string;
  images: string[];
  video_url?: string | null;
  status: number; // 1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled
  status_name: string;
  status_text: string;
  admin_note?: string | null;
  shipping_provider?: string | null;
  shipping_code?: string | null;
  shipping_note?: string | null;
  bank_name?: string | null;
  bank_account?: string | null;
  bank_account_name?: string | null;
  total_amount: number;
  created_at: string;
  updated_at: string;
  events?: TrackingEvent[];
}

interface ReturnDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  orderUuid: string;
  onUpdated?: () => void;
}

const COMMON_RETURN_COURIERS = [
  "GHN Express (Giao Hàng Nhanh)",
  "GHTK (Giao Hàng Tiết Kiệm)",
  "Viettel Post",
  "VNPost (Vietnam Post)",
  "J&T Express",
  "Ninja Van",
  "BEST Express",
  "Other Carrier",
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

export function ReturnDetailModal({
  isOpen,
  onClose,
  orderUuid,
  onUpdated,
}: ReturnDetailModalProps) {
  const [data, setData] = useState<ReturnDetailData | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // Sub-actions state
  const [isCancelling, setIsCancelling] = useState(false);
  const [cancelReason, setCancelReason] = useState("");
  const [showCancelPrompt, setShowCancelPrompt] = useState(false);

  // Return shipment form
  const [shippingProvider, setShippingProvider] = useState(
    COMMON_RETURN_COURIERS[0],
  );
  const [shippingCode, setShippingCode] = useState("");
  const [shippingNote, setShippingNote] = useState("");
  const [isSubmittingShipping, setIsSubmittingShipping] = useState(false);

  // Edit Bank Account form
  const [isEditingBank, setIsEditingBank] = useState(false);
  const [editBankName, setEditBankName] = useState("");
  const [editBankAccount, setEditBankAccount] = useState("");
  const [editBankAccountName, setEditBankAccountName] = useState("");
  const [isSavingBank, setIsSavingBank] = useState(false);

  const fetchReturnDetail = useCallback(async () => {
    setLoading(true);
    setErrorMsg(null);
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/return`);
      if (!res.ok) {
        const err = await res.json().catch(() => null);
        const msg = extractErrorMessage(
          err,
          "Could not retrieve return details.",
        );
        setErrorMsg(msg);
        return;
      }
      const json = await res.json();
      const returnData: ReturnDetailData = json.data;
      setData(returnData);
      if (returnData.bank_name) setEditBankName(returnData.bank_name);
      if (returnData.bank_account) setEditBankAccount(returnData.bank_account);
      if (returnData.bank_account_name)
        setEditBankAccountName(returnData.bank_account_name);
    } catch {
      setErrorMsg("Failed to load return details. Please check your network.");
    } finally {
      setLoading(false);
    }
  }, [orderUuid]);

  useEffect(() => {
    if (isOpen && orderUuid) {
      fetchReturnDetail();
    }
  }, [isOpen, orderUuid, fetchReturnDetail]);

  if (!isOpen) return null;

  const handleCancelReturn = async () => {
    if (!data) return;
    setIsCancelling(true);
    try {
      const res = await authFetch(`/api/v1/orders/${orderUuid}/return/cancel`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reason: cancelReason.trim() || undefined }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => null);
        toast.error(
          extractErrorMessage(err, "Failed to cancel return request."),
        );
        setIsCancelling(false);
        return;
      }
      toast.success("Return request cancelled successfully.");
      setShowCancelPrompt(false);
      fetchReturnDetail();
      onUpdated?.();
    } catch {
      toast.error("Network error while cancelling return.");
    } finally {
      setIsCancelling(false);
    }
  };

  const handleSubmitReturnShipping = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!shippingCode.trim()) {
      toast.error("Please enter the tracking code provided by your courier.");
      return;
    }
    setIsSubmittingShipping(true);
    try {
      const res = await authFetch(
        `/api/v1/orders/${orderUuid}/return/shipping`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            shipping_provider: shippingProvider,
            shipping_code: shippingCode.trim(),
            note: shippingNote.trim() || undefined,
          }),
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => null);
        toast.error(
          extractErrorMessage(err, "Failed to submit return shipping details."),
        );
        setIsSubmittingShipping(false);
        return;
      }
      toast.success("Return shipping information saved!");
      fetchReturnDetail();
      onUpdated?.();
    } catch {
      toast.error("Network error. Please try again.");
    } finally {
      setIsSubmittingShipping(false);
    }
  };

  const handleSaveBankInfo = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editBankName || !editBankAccount || !editBankAccountName) {
      toast.error("Please fill in all bank details.");
      return;
    }
    setIsSavingBank(true);
    try {
      const res = await authFetch(
        `/api/v1/orders/${orderUuid}/return/bank-account`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            bank_name: editBankName.trim(),
            bank_account: editBankAccount.trim(),
            bank_account_name: editBankAccountName.trim().toUpperCase(),
          }),
        },
      );
      if (!res.ok) {
        const err = await res.json().catch(() => null);
        toast.error(extractErrorMessage(err, "Failed to update bank details."));
        setIsSavingBank(false);
        return;
      }
      toast.success("Refund bank account updated!");
      setIsEditingBank(false);
      fetchReturnDetail();
      onUpdated?.();
    } catch {
      toast.error("Network error. Please try again.");
    } finally {
      setIsSavingBank(false);
    }
  };

  const getStatusBadge = (status: number, statusText: string) => {
    switch (status) {
      case 1:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold bg-[#FFFBEB] text-[#D97706] border border-[#FDE68A]">
            <Clock className="w-3.5 h-3.5" />
            <span>{statusText || "Pending Approval"}</span>
          </span>
        );
      case 2:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/40">
            <Truck className="w-3.5 h-3.5" />
            <span>{statusText || "Approved (Awaiting Shipment)"}</span>
          </span>
        );
      case 3:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold bg-[#FEF2F2] text-[#DC2626] border border-[#FECACA]">
            <XCircle className="w-3.5 h-3.5" />
            <span>{statusText || "Rejected"}</span>
          </span>
        );
      case 4:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold bg-[#ECFDF5] text-[#059669] border border-[#A7F3D0]">
            <ShieldCheck className="w-3.5 h-3.5" />
            <span>{statusText || "Completed & Refunded"}</span>
          </span>
        );
      case 5:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold bg-[#F1F5F9] text-[#475569] border border-[#E2E8F0]">
            <X className="w-3.5 h-3.5" />
            <span>{statusText || "Cancelled"}</span>
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs font-semibold bg-[#F1F5F9] text-[#475569] border border-[#E2E8F0]">
            <span>{statusText || "Processing"}</span>
          </span>
        );
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 overflow-y-auto font-body">
      <div className="w-full max-w-2xl bg-white rounded-md border border-[#E2E8F0] shadow-lg max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-start justify-between gap-4 px-6 py-5 border-b border-[#E2E8F0]">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-10 h-10 rounded-md bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/30 flex items-center justify-center shrink-0">
              <RotateCcw className="w-5 h-5" />
            </div>

            <div className="min-w-0">
              <div className="flex items-center gap-2 flex-wrap">
                <h2 className="text-base sm:text-lg font-bold text-[#0F172A] font-heading">
                  Return Request #
                  {data?.return_uuid.slice(0, 8).toUpperCase() || "..."}
                </h2>
                {data && getStatusBadge(data.status, data.status_text)}
              </div>

              <p className="text-xs text-[#64748B] mt-1">
                For Order #
                {data?.order_number || orderUuid.slice(0, 8).toUpperCase()} •
                Total:{" "}
                <strong className="text-[#007D9C] font-mono">
                  {formatCurrency(data?.total_amount || 0)}
                </strong>
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 rounded-md flex items-center justify-center text-[#64748B] hover:bg-[#F8FAFC] hover:text-[#0F172A] border border-[#E2E8F0] shrink-0"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Modal Content */}
        <div className="p-6">
          {loading ? (
            <div className="py-16 flex flex-col items-center justify-center gap-3">
              <Loader2 className="w-7 h-7 text-[#007D9C] animate-spin" />
              <p className="text-xs text-[#64748B] font-medium">
                Loading return details...
              </p>
            </div>
          ) : errorMsg ? (
            <div className="p-4 rounded-md bg-[#FEF2F2] border border-[#FECACA] text-xs text-[#DC2626] flex items-start gap-3">
              <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
              <div>
                <p className="font-bold">Could not load return request</p>
                <p className="mt-1">{errorMsg}</p>
              </div>
            </div>
          ) : data ? (
            <div className="space-y-6">
              {/* Admin note */}
              {data.admin_note && (
                <section className="rounded-md border border-[#FDE68A] bg-[#FFFBEB] p-4">
                  <div className="flex items-start gap-3">
                    <AlertCircle className="w-4 h-4 text-[#D97706] shrink-0 mt-0.5" />
                    <div className="min-w-0 text-xs">
                      <p className="font-bold text-[#92400E] font-heading">
                        Support Note / Review Response
                      </p>
                      <p className="text-[#475569] leading-relaxed mt-1">
                        {data.admin_note}
                      </p>
                    </div>
                  </div>
                </section>
              )}

              {/* Reason & media */}
              <section className="rounded-md border border-[#E2E8F0] bg-white p-4 space-y-4">
                <div className="flex items-center justify-between gap-4 pb-3 border-b border-[#E2E8F0]">
                  <h3 className="text-xs sm:text-sm font-bold text-[#0F172A] font-heading">
                    Return Reason & Description
                  </h3>
                  <span className="text-[11px] text-[#64748B]">
                    {formatDateTime(data.created_at)}
                  </span>
                </div>

                <p className="text-xs text-[#0F172A] whitespace-pre-line leading-relaxed">
                  {data.reason}
                </p>

                {/* Submitted Proof */}
                {(data.images?.length > 0 || data.video_url) && (
                  <div className="pt-3 border-t border-[#E2E8F0] space-y-2">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-semibold text-[#0F172A]">
                        Attached Proof Files
                      </span>
                      <span className="text-[11px] text-[#64748B]">
                        {data.images?.length || 0} images
                        {data.video_url ? " + 1 video" : ""}
                      </span>
                    </div>

                    <div className="flex gap-2 flex-wrap">
                      {data.images?.map((img, idx) => {
                        const imageUrl = img.startsWith("http")
                          ? img
                          : `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}${img}`;
                        return (
                          <a
                            key={idx}
                            href={imageUrl}
                            target="_blank"
                            rel="noreferrer"
                            className="w-16 h-16 rounded-md border border-[#E2E8F0] overflow-hidden bg-[#F8FAFC] block"
                          >
                            {/* eslint-disable-next-line @next/next/no-img-element */}
                            <img
                              src={imageUrl}
                              alt={`Proof ${idx + 1}`}
                              className="w-full h-full object-cover"
                            />
                          </a>
                        );
                      })}

                      {data.video_url && (
                        <a
                          href={
                            data.video_url.startsWith("http")
                              ? data.video_url
                              : `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}${data.video_url}`
                          }
                          target="_blank"
                          rel="noreferrer"
                          className="w-16 h-16 rounded-md border border-[#E2E8F0] bg-[#0F172A] flex flex-col items-center justify-center text-[#00ADD8]"
                        >
                          <Film className="w-5 h-5" />
                          <span className="text-[10px] font-semibold text-white mt-1">
                            Video
                          </span>
                        </a>
                      )}
                    </div>
                  </div>
                )}
              </section>

              {/* Shipping section */}
              {data.status === 2 && (
                <section className="rounded-md border border-[#00ADD8]/40 bg-[#E6F7FB]/40 p-4 space-y-3">
                  <div className="flex items-start gap-3">
                    <Truck className="w-4 h-4 text-[#007D9C] shrink-0 mt-0.5" />
                    <div>
                      <h3 className="text-xs sm:text-sm font-bold text-[#0F172A] font-heading">
                        Return Shipment Information
                      </h3>
                      <p className="text-xs text-[#64748B] mt-0.5 leading-relaxed">
                        Please pack the items securely and dispatch via a
                        registered carrier. Submit your waybill tracking number
                        below.
                      </p>
                    </div>
                  </div>

                  {data.shipping_code ? (
                    <div className="rounded-md border border-[#E2E8F0] bg-white p-3 space-y-2 text-xs">
                      <div className="flex justify-between gap-4">
                        <span className="text-[#64748B]">Courier</span>
                        <span className="font-semibold text-[#0F172A]">
                          {data.shipping_provider}
                        </span>
                      </div>
                      <div className="flex justify-between gap-4">
                        <span className="text-[#64748B]">Tracking Waybill</span>
                        <span className="font-mono font-bold text-[#007D9C]">
                          {data.shipping_code}
                        </span>
                      </div>
                      {data.shipping_note && (
                        <div className="flex justify-between gap-4">
                          <span className="text-[#64748B]">Note</span>
                          <span className="text-[#0F172A] text-right">
                            {data.shipping_note}
                          </span>
                        </div>
                      )}
                    </div>
                  ) : (
                    <form
                      onSubmit={handleSubmitReturnShipping}
                      className="space-y-3 pt-2"
                    >
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                        <div className="space-y-1">
                          <label className="text-xs font-semibold text-[#0F172A]">
                            Courier Service
                          </label>
                          <select
                            value={shippingProvider}
                            onChange={(e) =>
                              setShippingProvider(e.target.value)
                            }
                            className="w-full h-9 px-3 rounded-md border border-[#E2E8F0] bg-white text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C]"
                          >
                            {COMMON_RETURN_COURIERS.map((c) => (
                              <option key={c} value={c}>
                                {c}
                              </option>
                            ))}
                          </select>
                        </div>

                        <div className="space-y-1">
                          <label className="text-xs font-semibold text-[#0F172A]">
                            Waybill / Tracking Number{" "}
                            <span className="text-rose-500">*</span>
                          </label>
                          <input
                            type="text"
                            required
                            value={shippingCode}
                            onChange={(e) => setShippingCode(e.target.value)}
                            placeholder="e.g. GHN123456789"
                            className="w-full h-9 px-3 rounded-md border border-[#E2E8F0] bg-white text-xs font-mono text-[#0F172A] placeholder-[#64748B] focus:outline-none focus:border-[#007D9C]"
                          />
                        </div>
                      </div>

                      <div className="space-y-1">
                        <label className="text-xs font-semibold text-[#0F172A]">
                          Shipping Note (Optional)
                        </label>
                        <input
                          type="text"
                          value={shippingNote}
                          onChange={(e) => setShippingNote(e.target.value)}
                          placeholder="e.g. Dispatched at central post office..."
                          className="w-full h-9 px-3 rounded-md border border-[#E2E8F0] bg-white text-xs text-[#0F172A] placeholder-[#64748B] focus:outline-none focus:border-[#007D9C]"
                        />
                      </div>

                      <button
                        type="submit"
                        disabled={isSubmittingShipping}
                        className="h-9 px-4 rounded-md bg-[#007D9C] hover:bg-[#006782] text-white text-xs font-semibold flex items-center gap-2 disabled:opacity-50"
                      >
                        {isSubmittingShipping ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <Send className="w-3.5 h-3.5" />
                        )}
                        <span>Save Return Tracking</span>
                      </button>
                    </form>
                  )}
                </section>
              )}

              {/* Refund Bank section */}
              <section className="rounded-md border border-[#E2E8F0] bg-[#F8FAFC] p-4 space-y-3">
                <div className="flex items-center justify-between gap-4">
                  <div className="flex items-center gap-2">
                    <Building2 className="w-4 h-4 text-[#007D9C]" />
                    <h3 className="text-xs sm:text-sm font-bold text-[#0F172A] font-heading">
                      Refund Bank Account
                    </h3>
                  </div>

                  {data.status < 4 && !isEditingBank && (
                    <button
                      type="button"
                      onClick={() => setIsEditingBank(true)}
                      className="h-7 px-2.5 rounded-md text-xs font-semibold text-[#007D9C] hover:bg-[#E6F7FB] border border-[#00ADD8]/30 flex items-center gap-1"
                    >
                      <Edit2 className="w-3 h-3" />
                      <span>
                        {data.bank_account ? "Edit Account" : "Add Account"}
                      </span>
                    </button>
                  )}
                </div>

                {isEditingBank ? (
                  <form
                    onSubmit={handleSaveBankInfo}
                    className="space-y-3 pt-1"
                  >
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-[#0F172A]">
                        Bank Name
                      </label>
                      <input
                        type="text"
                        list="edit-vietnam-banks"
                        required
                        value={editBankName}
                        onChange={(e) => setEditBankName(e.target.value)}
                        placeholder="e.g. Vietcombank"
                        className="w-full h-9 px-3 rounded-md border border-[#E2E8F0] bg-white text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C]"
                      />
                      <datalist id="edit-vietnam-banks">
                        {VIETNAM_BANKS.map((b) => (
                          <option key={b} value={b} />
                        ))}
                      </datalist>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      <div className="space-y-1">
                        <label className="text-xs font-semibold text-[#0F172A]">
                          Account Number
                        </label>
                        <input
                          type="text"
                          required
                          value={editBankAccount}
                          onChange={(e) => setEditBankAccount(e.target.value)}
                          placeholder="e.g. 0123456789"
                          className="w-full h-9 px-3 rounded-md border border-[#E2E8F0] bg-white text-xs font-mono text-[#0F172A] focus:outline-none focus:border-[#007D9C]"
                        />
                      </div>

                      <div className="space-y-1">
                        <label className="text-xs font-semibold text-[#0F172A]">
                          Beneficiary Name
                        </label>
                        <input
                          type="text"
                          required
                          value={editBankAccountName}
                          onChange={(e) =>
                            setEditBankAccountName(e.target.value)
                          }
                          placeholder="e.g. NGUYEN VAN A"
                          className="w-full h-9 px-3 rounded-md border border-[#E2E8F0] bg-white text-xs uppercase text-[#0F172A] focus:outline-none focus:border-[#007D9C]"
                        />
                      </div>
                    </div>

                    <div className="flex justify-end gap-2 pt-1">
                      <button
                        type="button"
                        onClick={() => setIsEditingBank(false)}
                        className="h-8 px-3 rounded-md border border-[#E2E8F0] bg-white text-[#0F172A] text-xs font-semibold hover:bg-[#F8FAFC]"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        disabled={isSavingBank}
                        className="h-8 px-3 rounded-md bg-[#007D9C] hover:bg-[#006782] text-white text-xs font-semibold disabled:opacity-50"
                      >
                        {isSavingBank ? "Saving..." : "Save Details"}
                      </button>
                    </div>
                  </form>
                ) : (
                  <div className="bg-white rounded-md border border-[#E2E8F0] divide-y divide-[#E2E8F0] text-xs">
                    <div className="flex justify-between gap-4 px-3.5 py-2.5">
                      <span className="text-[#64748B]">Bank</span>
                      <span className="font-semibold text-[#0F172A]">
                        {data.bank_name || "Not provided"}
                      </span>
                    </div>
                    <div className="flex justify-between gap-4 px-3.5 py-2.5">
                      <span className="text-[#64748B]">Account Number</span>
                      <span className="font-mono font-semibold text-[#0F172A]">
                        {data.bank_account || "Not provided"}
                      </span>
                    </div>
                    <div className="flex justify-between gap-4 px-3.5 py-2.5">
                      <span className="text-[#64748B]">Account Holder</span>
                      <span className="font-semibold uppercase text-[#0F172A]">
                        {data.bank_account_name || "Not provided"}
                      </span>
                    </div>
                  </div>
                )}
              </section>

              {/* Events timeline */}
              {data.events && data.events.length > 0 && (
                <section className="rounded-md border border-[#E2E8F0] bg-white p-4 space-y-3">
                  <h3 className="text-xs sm:text-sm font-bold text-[#0F172A] font-heading">
                    Return Activity History
                  </h3>

                  <div className="relative pl-6 space-y-4 before:absolute before:left-2 before:top-2 before:bottom-2 before:w-px before:bg-[#E2E8F0]">
                    {data.events.map((evt, idx) => (
                      <div key={idx} className="relative">
                        <div className="absolute -left-5 top-1 w-2.5 h-2.5 rounded-md bg-[#007D9C]" />
                        <div className="space-y-0.5 text-xs">
                          <div className="flex items-center justify-between gap-2">
                            <span className="font-semibold text-[#0F172A]">
                              {evt.status_text || evt.status}
                            </span>
                            <span className="text-[11px] text-[#64748B]">
                              {formatDateTime(evt.created_at)}
                            </span>
                          </div>
                          {evt.note && (
                            <p className="text-[11px] text-[#64748B]">
                              {evt.note}
                            </p>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </section>
              )}

              {/* Cancel Request Action */}
              {data.status === 1 && (
                <div className="pt-2">
                  {showCancelPrompt ? (
                    <div className="rounded-md border border-[#FECACA] bg-[#FEF2F2] p-4 space-y-3">
                      <div className="flex items-start gap-2.5">
                        <AlertCircle className="w-4 h-4 text-[#DC2626] shrink-0 mt-0.5" />
                        <div className="text-xs">
                          <p className="font-bold text-[#991B1B] font-heading">
                            Cancel Return Request?
                          </p>
                          <p className="text-[#B91C1C] mt-0.5">
                            Are you sure you want to cancel this request? This
                            action cannot be undone.
                          </p>
                        </div>
                      </div>

                      <input
                        type="text"
                        value={cancelReason}
                        onChange={(e) => setCancelReason(e.target.value)}
                        placeholder="Optional cancellation note..."
                        className="w-full h-9 px-3 rounded-md border border-[#FECACA] bg-white text-xs text-[#0F172A] placeholder-[#94A3B8] focus:outline-none"
                      />

                      <div className="flex justify-end gap-2">
                        <button
                          type="button"
                          onClick={() => setShowCancelPrompt(false)}
                          className="h-8 px-3 rounded-md border border-[#E2E8F0] bg-white text-[#0F172A] text-xs font-semibold hover:bg-[#F8FAFC]"
                        >
                          Keep Request
                        </button>
                        <button
                          type="button"
                          disabled={isCancelling}
                          onClick={handleCancelReturn}
                          className="h-8 px-3 rounded-md bg-[#DC2626] hover:bg-rose-700 text-white text-xs font-semibold disabled:opacity-50"
                        >
                          {isCancelling
                            ? "Cancelling..."
                            : "Confirm Cancellation"}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <button
                      type="button"
                      onClick={() => setShowCancelPrompt(true)}
                      className="w-full h-9 rounded-md border border-[#FECACA] text-[#DC2626] hover:bg-[#FEF2F2] text-xs font-semibold"
                    >
                      Cancel Return Request
                    </button>
                  )}
                </div>
              )}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
