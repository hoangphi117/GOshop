"use client";

import React, { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  CheckCircle2,
  XCircle,
  ShoppingBag,
  Home,
  Receipt,
  ArrowRight,
  ShieldCheck,
  AlertTriangle,
  RotateCcw,
  RefreshCw,
} from "lucide-react";
import { formatCurrency } from "@/lib/format";
import { authFetch } from "@/lib/api";
import { extractErrorMessage } from "@/lib/utils";
import { useCartStore } from "@/stores/cart-store";

function PaymentResultContent() {
  const searchParams = useSearchParams();
  const removeSelectedItems = useCartStore(
    (state) => state.removeSelectedItems,
  );

  // Payment gateway return URL parameters (Supports MoMo & VNPAY)
  const vnpResponseCode = searchParams.get("vnp_ResponseCode");
  const isVnPay = vnpResponseCode !== null;
  const resultCode = searchParams.get("resultCode");

  const orderId = searchParams.get("vnp_TxnRef") || searchParams.get("orderId") || "";
  const transId = searchParams.get("vnp_TransactionNo") || searchParams.get("transId") || "";
  const vnpAmount = searchParams.get("vnp_Amount");
  const amountStr = searchParams.get("amount") || "0";
  const message = searchParams.get("message") || "";
  const orderInfo = searchParams.get("vnp_OrderInfo") || searchParams.get("orderInfo") || "";
  const payType = searchParams.get("vnp_CardType") || searchParams.get("vnp_BankCode") || searchParams.get("payType") || "";
  const responseTime = searchParams.get("vnp_PayDate") || searchParams.get("responseTime") || "";

  const isSuccess = isVnPay ? vnpResponseCode === "00" : resultCode === "0";
  const amount = isVnPay && vnpAmount ? parseInt(vnpAmount, 10) / 100 : (parseInt(amountStr, 10) || 0);
  const provider = isVnPay ? "vnpay" : "momo";
  const providerName = isVnPay ? "VNPAY" : "MoMo";

  // If orderId is in format "ORDERNUMBER_TIMESTAMP", extract clean order number
  const displayOrderNumber = orderId.includes("_")
    ? orderId.split("_")[0]
    : orderId;

  // Clean formatted date from responseTime if available
  let formattedDate = "";
  if (responseTime) {
    if (isVnPay && responseTime.length === 14) {
      const year = responseTime.substring(0, 4);
      const month = responseTime.substring(4, 6);
      const day = responseTime.substring(6, 8);
      const hour = responseTime.substring(8, 10);
      const min = responseTime.substring(10, 12);
      const sec = responseTime.substring(12, 14);
      const d = new Date(`${year}-${month}-${day}T${hour}:${min}:${sec}`);
      if (!isNaN(d.getTime())) {
        formattedDate = d.toLocaleString("en-US", {
          dateStyle: "medium",
          timeStyle: "short",
        });
      }
    } else {
      const timestamp = parseInt(responseTime, 10);
      if (!isNaN(timestamp)) {
        formattedDate = new Date(timestamp).toLocaleString("en-US", {
          dateStyle: "medium",
          timeStyle: "short",
        });
      }
    }
  }

  const [isRetrying, setIsRetrying] = useState(false);
  const [resolvedOrderUuid, setResolvedOrderUuid] = useState<string | null>(null);
  const [isOrderExpired, setIsOrderExpired] = useState(false);
  const [isOrderCancelled, setIsOrderCancelled] = useState(false);

  useEffect(() => {
    if (isSuccess) {
      removeSelectedItems();
      // Synchronize with payment service to verify return signature and finalize order status to Paid
      const confirmPayment = async () => {
        try {
          const baseUrl =
            process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
          const query = window.location.search.startsWith("?")
            ? window.location.search.slice(1)
            : window.location.search;
          if (query) {
            await authFetch(`${baseUrl}/api/v1/payments/webhook/${provider}?${query}`, {
              method: "GET",
            });
          }
        } catch {
          // Ignore background sync errors
        }
      };
      confirmPayment();
    } else if (displayOrderNumber) {
      // Resolve Order UUID in background to prepare for instant retry
      const fetchOrderUuid = async () => {
        try {
          const baseUrl =
            process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
          const res = await authFetch(`${baseUrl}/api/v1/orders`);
          if (res.ok) {
            const json = await res.json();
            const orderList = Array.isArray(json?.data)
              ? json.data
              : Array.isArray(json?.data?.items)
              ? json.data.items
              : [];
            const matched = orderList.find(
              (o: { order_number?: string; uuid?: string; created_at?: string; status?: string }) =>
                o.order_number === displayOrderNumber ||
                o.uuid === displayOrderNumber
            );
            if (matched) {
              if (matched.uuid) {
                setResolvedOrderUuid(matched.uuid);
              }
              if (matched.status?.toUpperCase() === "CANCELLED") {
                setIsOrderCancelled(true);
              }
              if (
                matched.created_at &&
                Date.now() - new Date(matched.created_at).getTime() > 15 * 60 * 1000
              ) {
                setIsOrderExpired(true);
              }
            }
          }
        } catch {
          // Ignore background fetch error
        }
      };
      fetchOrderUuid();
    }
  }, [isSuccess, displayOrderNumber, removeSelectedItems]);

  const handleRetryPayment = async () => {
    setIsRetrying(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      let targetUuid = resolvedOrderUuid;

      // If not yet resolved, query the user's orders
      if (!targetUuid && displayOrderNumber) {
        const res = await authFetch(`${baseUrl}/api/v1/orders`);
        if (res.ok) {
          const json = await res.json();
          const orderList = Array.isArray(json?.data)
            ? json.data
            : Array.isArray(json?.data?.items)
            ? json.data.items
            : [];
          const matched = orderList.find(
            (o: { order_number?: string; uuid?: string }) =>
              o.order_number === displayOrderNumber ||
              o.uuid === displayOrderNumber
          );
          if (matched?.uuid) {
            targetUuid = matched.uuid;
            setResolvedOrderUuid(matched.uuid);
          }
        }
      }

      if (!targetUuid) {
        toast.error("Could not find order. Redirecting to My Orders...");
        window.location.href = "/account/orders";
        return;
      }

      const paymentRes = await authFetch(
        `${baseUrl}/api/v1/payments/create-intent`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Idempotency-Key": `idem-pay-${targetUuid}`,
          },
          body: JSON.stringify({
            order_uuid: targetUuid,
            provider: provider,
          }),
        }
      );

      if (paymentRes.ok) {
        const paymentData = await paymentRes.json();
        const paymentUrl = paymentData?.data?.payment_url;
        if (paymentUrl) {
          toast.success(`Redirecting to ${providerName} payment gateway...`);
          window.location.href = paymentUrl;
          return;
        }
      }

      const errJson = await paymentRes.json().catch(() => null);
      const errMsg = extractErrorMessage(
        errJson,
        `Unable to create ${providerName} payment link.`
      );
      toast.error(errMsg);
    } catch {
      toast.error("Network error while attempting to retry payment.");
    } finally {
      setIsRetrying(false);
    }
  };

  return (
    <div className="min-h-[75vh] py-8 px-4 flex items-center justify-center">
      <div className="max-w-lg w-full mx-auto space-y-6">
        {/* Status Header */}
        <div className="text-center space-y-3">
          <div
            className={`w-20 h-20 rounded-full flex items-center justify-center mx-auto transition-transform ${
              isSuccess
                ? "bg-emerald-50 text-emerald-500 ring-8 ring-emerald-50/50"
                : "bg-rose-50 text-rose-500 ring-8 ring-rose-50/50"
            }`}
          >
            {isSuccess ? (
              <CheckCircle2 className="w-12 h-12" />
            ) : (
              <XCircle className="w-12 h-12" />
            )}
          </div>

          <div className="space-y-1">
            <span
              className={`inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-bold uppercase tracking-wider ${
                isSuccess
                  ? "bg-emerald-100/80 text-emerald-800"
                  : "bg-rose-100/80 text-rose-800"
              }`}
            >
              {isSuccess ? (
                <>
                  <ShieldCheck className="w-3.5 h-3.5" />
                  Payment Successful
                </>
              ) : (
                <>
                  <AlertTriangle className="w-3.5 h-3.5" />
                  Payment Incomplete
                </>
              )}
            </span>
            <h1 className="text-2xl sm:text-3xl font-black text-[#0D1B2A] tracking-tight">
              {isSuccess
                ? "Thank You For Your Payment!"
                : "Transaction Unsuccessful"}
            </h1>
            <p className="text-sm text-slate-600 max-w-sm mx-auto">
              {isSuccess
                ? `Your transaction has been processed securely via ${providerName}.`
                : message ||
                  "The transaction was cancelled or could not be completed. No funds have been deducted."}
            </p>
          </div>
        </div>

        {/* Transaction Summary Card */}
        <div className="bg-white rounded-2xl border border-slate-200/80 shadow-xs overflow-hidden">
          <div className="px-5 py-4 bg-slate-50/60 border-b border-slate-200/70 flex items-center justify-between">
            <span className="text-xs font-bold uppercase tracking-wider text-slate-500 flex items-center gap-1.5">
              <Receipt className="w-4 h-4 text-slate-400" />
              Receipt Details
            </span>
            <span className="text-xs font-semibold text-slate-500">
              Provider:{" "}
              <span
                className={`font-bold ${
                  isVnPay ? "text-[#005BAA]" : "text-[#A50064]"
                }`}
              >
                {providerName}
              </span>
            </span>
          </div>

          <div className="p-5 space-y-3 text-xs">
            {displayOrderNumber && (
              <div className="flex justify-between items-center py-1 border-b border-slate-100">
                <span className="text-slate-500">Order Reference</span>
                <span className="font-bold text-[#0D47A1]">
                  #{displayOrderNumber}
                </span>
              </div>
            )}

            {transId && (
              <div className="flex justify-between items-center py-1 border-b border-slate-100">
                <span className="text-slate-500">{providerName} Transaction ID</span>
                <span className="font-mono font-medium text-slate-700">
                  {transId}
                </span>
              </div>
            )}

            {amount > 0 && (
              <div className="flex justify-between items-center py-1 border-b border-slate-100">
                <span className="text-slate-500">Amount Paid</span>
                <span className="text-sm font-black text-emerald-600">
                  {formatCurrency(amount)}
                </span>
              </div>
            )}

            {orderInfo && (
              <div className="flex justify-between items-center py-1 border-b border-slate-100">
                <span className="text-slate-500">Order Note</span>
                <span className="text-slate-700 font-medium truncate max-w-[240px]">
                  {orderInfo}
                </span>
              </div>
            )}

            {payType && (
              <div className="flex justify-between items-center py-1 border-b border-slate-100">
                <span className="text-slate-500">Payment Channel</span>
                <span className="font-medium text-slate-700 uppercase">
                  {payType}
                </span>
              </div>
            )}

            {formattedDate && (
              <div className="flex justify-between items-center py-1">
                <span className="text-slate-500">Transaction Time</span>
                <span className="text-slate-700 font-medium">
                  {formattedDate}
                </span>
              </div>
            )}
          </div>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-col sm:flex-row gap-3 justify-center pt-2">
          {isSuccess ? (
            <>
              <Link
                href="/account/orders"
                className="px-6 py-3 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-md flex items-center justify-center gap-2"
              >
                View My Orders
                <ArrowRight className="w-4 h-4" />
              </Link>
              <Link
                href="/products"
                className="px-6 py-3 rounded-xl bg-white border border-slate-200 text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-all flex items-center justify-center gap-2"
              >
                <ShoppingBag className="w-4 h-4" />
                Continue Shopping
              </Link>
            </>
          ) : (
            <>
              {/* Expired / Cancelled notice */}
              {isOrderExpired || isOrderCancelled ? (
                <div className="w-full p-4 rounded-xl bg-amber-50 border border-amber-200 text-amber-800 text-xs text-center space-y-1">
                  <p className="font-bold">Payment Window Expired</p>
                  <p className="text-amber-700">
                    The 15-minute payment window for this order has expired. Unpaid orders are automatically cancelled to release reserved stock.
                  </p>
                </div>
              ) : (
                /* Retry Payment Button */
                <button
                  type="button"
                  onClick={handleRetryPayment}
                  disabled={isRetrying}
                  className={`px-6 py-3 rounded-xl text-white text-xs font-bold transition-all shadow-md flex items-center justify-center gap-2 cursor-pointer disabled:opacity-60 ${
                    isVnPay
                      ? "bg-[#005BAA] hover:bg-[#004A8B]"
                      : "bg-[#A50064] hover:bg-[#850050]"
                  }`}
                >
                  {isRetrying ? (
                    <RefreshCw className="w-4 h-4 animate-spin" />
                  ) : (
                    <RotateCcw className="w-4 h-4" />
                  )}
                  <span>
                    {isRetrying
                      ? `Connecting ${providerName}...`
                      : "Retry Payment"}
                  </span>
                </button>
              )}

              {/* View My Orders Button */}
              <Link
                href="/account/orders"
                className="px-6 py-3 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-md flex items-center justify-center gap-2"
              >
                <ShoppingBag className="w-4 h-4" />
                <span>View My Orders</span>
              </Link>

              {/* Back to Home Button */}
              <Link
                href="/"
                className="px-6 py-3 rounded-xl bg-white border border-slate-200 text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-all flex items-center justify-center gap-2"
              >
                <Home className="w-4 h-4" />
                Back to Home
              </Link>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

export default function PaymentResultPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-[75vh] flex items-center justify-center">
          <div className="animate-spin rounded-full h-10 w-10 border-b-2 border-[#2196F3]" />
        </div>
      }
    >
      <PaymentResultContent />
    </Suspense>
  );
}
