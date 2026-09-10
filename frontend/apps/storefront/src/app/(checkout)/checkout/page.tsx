"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { formatCurrency } from "@/lib/format";
import { getImageUrl } from "@/lib/utils";
import { ShieldCheck, Banknote, ArrowLeft, ShoppingCart, CheckCircle2 } from "lucide-react";
import { toast } from "sonner";
import { useMounted } from "@/hooks/use-mounted";

export default function CheckoutPage() {
  const { items, removeSelectedItems } = useCartStore();
  const { user } = useAuthStore();

  const mounted = useMounted();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [orderPlaced, setOrderPlaced] = useState<{
    orderId: string;
    paymentMethod: string;
  } | null>(null);

  // Form State
  const [fullName, setFullName] = useState(user?.fullName || "");
  const [email, setEmail] = useState(user?.email || "");
  const [phone, setPhone] = useState("");
  const [address, setAddress] = useState("");
  const [city, setCity] = useState("Ho Chi Minh City");
  const [district, setDistrict] = useState("District 1");
  const [ward, setWard] = useState("Ben Nghe Ward");
  const [note, setNote] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<"COD" | "MOMO">("COD");

  const selectedItems = items.filter((item) => item.selected !== false);

  if (!mounted) {
    return (
      <div className="min-h-[60vh] flex items-center justify-center">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    );
  }

  if (items.length === 0 || (!orderPlaced && selectedItems.length === 0)) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-20 h-20 rounded-3xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto shadow-xs">
            <ShoppingCart className="w-10 h-10" />
          </div>
          <h1 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
            No Items Selected for Checkout
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            Please choose the products you wish to purchase from your shopping
            cart.
          </p>
          <div className="pt-2">
            <Link
              href="/cart"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-2xl bg-[#2196F3] text-white text-sm font-semibold hover:bg-[#0D47A1] transition-all shadow-md"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>Back to Cart</span>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const subtotal = selectedItems.reduce(
    (acc, item) => acc + item.price * item.quantity,
    0,
  );
  const shippingFee =
    subtotal > 500000 || selectedItems.length === 0 ? 0 : 30000;
  const grandTotal = subtotal + shippingFee;

  const handleSubmitOrder = async (e: React.FormEvent) => {
    e.preventDefault();

    if (selectedItems.length === 0) {
      toast.error("No items selected for checkout.");
      return;
    }

    if (!fullName.trim() || !phone.trim() || !address.trim()) {
      toast.error("Please fill in all required shipping fields.");
      return;
    }

    setIsSubmitting(true);

    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const token = useAuthStore.getState().accessToken;
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        "X-API-KEY": process.env.NEXT_PUBLIC_API_KEY || "abcd-1234-1111",
      };
      if (token) {
        headers["Authorization"] = `Bearer ${token}`;
      }

      // Backend expects payment_method: 1 (COD) or 3 (E-Wallet / MoMo)
      const orderPayload = {
        customer_name: fullName,
        customer_email: email,
        customer_phone: phone,
        shipping_name: fullName,
        shipping_phone: phone,
        shipping_address: `${address}, ${ward}, ${district}, ${city}`,
        shipping_fee: shippingFee,
        payment_method: paymentMethod === "COD" ? 1 : 3,
        note,
        items: selectedItems.map((item) => ({
          product_uuid: item.productUuid,
          variant_uuid: item.variantUuid,
          quantity: item.quantity,
          price: item.price,
        })),
        total_amount: grandTotal,
      };

      const res = await fetch(`${baseUrl}/api/v1/orders`, {
        method: "POST",
        headers,
        body: JSON.stringify(orderPayload),
      }).catch(() => null);

      let generatedId = `GS-${Math.floor(100000 + Math.random() * 900000)}`;
      let createdOrderUuid = "";

      if (res && res.ok) {
        const data = await res.json();
        if (data?.data?.uuid) {
          createdOrderUuid = data.data.uuid;
          generatedId =
            data.data.order_number || data.data.uuid.slice(0, 8).toUpperCase();
        }
      } else if (res && !res.ok) {
        const errJson = await res.json().catch(() => null);
        const errMsg =
          errJson?.message || errJson?.detail || "Failed to place order.";
        toast.error(errMsg);
        setIsSubmitting(false);
        return;
      }

      // If MoMo payment method is selected and order was created, initiate MoMo payment intent
      if (paymentMethod === "MOMO" && createdOrderUuid) {
        try {
          const paymentRes = await fetch(
            `${baseUrl}/api/v1/payments/create-intent`,
            {
              method: "POST",
              headers: {
                ...headers,
                "X-Idempotency-Key": `idem-pay-${createdOrderUuid}-${Date.now()}`,
              },
              body: JSON.stringify({
                order_uuid: createdOrderUuid,
                provider: "momo",
              }),
            },
          );

          if (paymentRes.ok) {
            const paymentData = await paymentRes.json();
            const paymentUrl = paymentData?.data?.payment_url;
            if (paymentUrl) {
              removeSelectedItems();
              toast.success("Redirecting to MoMo payment gateway...");
              window.location.href = paymentUrl;
              return;
            }
          } else {
            const payErr = await paymentRes.json().catch(() => null);
            const payMsg =
              payErr?.message ||
              payErr?.detail ||
              "Could not initiate MoMo payment URL.";
            toast.error(payMsg);
          }
        } catch {
          toast.error("Network error while connecting to MoMo gateway.");
        }
      }

      removeSelectedItems();
      setOrderPlaced({
        orderId: generatedId,
        paymentMethod,
      });
      toast.success("Order placed successfully!");
    } catch {
      toast.error("Failed to create order. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  // If order was successfully placed, display confirmation screen
  if (orderPlaced) {
    return (
      <div className="bg-[#FAFBFC] min-h-[75vh] py-16 flex items-center justify-center">
        <div className="max-w-lg w-full mx-auto px-4 text-center space-y-5">
          <div className="w-20 h-20 rounded-full bg-emerald-50 text-emerald-500 flex items-center justify-center mx-auto shadow-xs">
            <CheckCircle2 className="w-12 h-12" />
          </div>
          <h1 className="text-3xl font-black text-[#0D1B2A] tracking-tight">
            Thank You For Your Order!
          </h1>
          <p className="text-sm text-slate-600 leading-relaxed">
            Your order{" "}
            <strong className="text-[#0D47A1]">#{orderPlaced.orderId}</strong>{" "}
            has been placed. A confirmation email has been sent to{" "}
            <strong className="text-slate-800">{email || "your email"}</strong>.
          </p>

          <div className="p-4 rounded-2xl bg-white border border-slate-200/80 text-left space-y-2 text-xs">
            <div className="flex justify-between">
              <span className="text-slate-500">Payment Mode:</span>
              <span className="font-bold text-[#0D1B2A]">
                {orderPlaced.paymentMethod === "COD"
                  ? "Cash on Delivery"
                  : "MoMo E-Wallet"}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Delivery Address:</span>
              <span className="font-medium text-slate-700 truncate max-w-xs">
                {address}, {city}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Shipping Partner:</span>
              <span className="font-semibold text-emerald-600">
                GHN Express (1-3 days)
              </span>
            </div>
          </div>

          <div className="pt-4 flex flex-col sm:flex-row gap-3 justify-center">
            <Link
              href="/"
              className="px-6 py-3 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-md"
            >
              Back to Home
            </Link>
            <Link
              href="/products"
              className="px-6 py-3 rounded-xl bg-white border border-slate-200 text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-all"
            >
              Continue Shopping
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-3">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        {/* Header */}
        <div className="flex items-center justify-between pb-6 border-b border-slate-200/80 mb-8">
          <div>
            <Link
              href="/cart"
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 hover:text-[#2196F3] mb-2"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              <span>Back to cart</span>
            </Link>
            <h1 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
              Checkout
            </h1>
          </div>
        </div>

        <form onSubmit={handleSubmitOrder}>
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
            {/* Left Column: Form Details (7 cols) */}
            <div className="lg:col-span-7 space-y-6">
              {/* Customer Contact */}
              <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-4">
                <h2 className="text-base font-bold text-[#0D1B2A]">
                  1. Contact Information
                </h2>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Full Name <span className="text-rose-500">*</span>
                    </label>
                    <input
                      type="text"
                      required
                      value={fullName}
                      onChange={(e) => setFullName(e.target.value)}
                      placeholder="e.g. John Doe"
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Phone Number <span className="text-rose-500">*</span>
                    </label>
                    <input
                      type="tel"
                      required
                      value={phone}
                      onChange={(e) => setPhone(e.target.value)}
                      placeholder="e.g. 0901234567"
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    Email Address
                  </label>
                  <input
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="e.g. customer@example.com"
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                  />
                </div>
              </div>

              {/* Shipping Address */}
              <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-4">
                <h2 className="text-base font-bold text-[#0D1B2A]">
                  2. Shipping Address
                </h2>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    Street Address <span className="text-rose-500">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    value={address}
                    onChange={(e) => setAddress(e.target.value)}
                    placeholder="Apartment, suite, street name, house number"
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      City / Province
                    </label>
                    <input
                      type="text"
                      value={city}
                      onChange={(e) => setCity(e.target.value)}
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      District
                    </label>
                    <input
                      type="text"
                      value={district}
                      onChange={(e) => setDistrict(e.target.value)}
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Ward
                    </label>
                    <input
                      type="text"
                      value={ward}
                      onChange={(e) => setWard(e.target.value)}
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    Delivery Instructions / Note
                  </label>
                  <textarea
                    rows={2}
                    value={note}
                    onChange={(e) => setNote(e.target.value)}
                    placeholder="e.g. Call before delivery, drop at reception"
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                  />
                </div>
              </div>

              {/* Payment Methods */}
              <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-4">
                <h2 className="text-base font-bold text-[#0D1B2A]">
                  3. Payment Method
                </h2>

                <div className="space-y-3">
                  {/* COD */}
                  <label
                    onClick={() => setPaymentMethod("COD")}
                    className={`flex items-start gap-3 p-4 rounded-xl border cursor-pointer transition-all ${
                      paymentMethod === "COD"
                        ? "border-[#2196F3] bg-[#E3F2FD]/40 shadow-xs"
                        : "border-slate-200 hover:bg-slate-50"
                    }`}
                  >
                    <input
                      type="radio"
                      name="payment"
                      checked={paymentMethod === "COD"}
                      onChange={() => setPaymentMethod("COD")}
                      className="mt-0.5 text-[#2196F3]"
                    />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Banknote className="w-4 h-4 text-[#0D47A1]" />
                        <span className="text-xs font-bold text-[#0D1B2A]">
                          Cash on Delivery (COD)
                        </span>
                      </div>
                      <p className="text-[11px] text-slate-500 mt-1">
                        Pay in cash upon receiving your package from the
                        courier.
                      </p>
                    </div>
                  </label>

                  {/* MoMo E-Wallet */}
                  <label
                    onClick={() => setPaymentMethod("MOMO")}
                    className={`flex items-start gap-3 p-4 rounded-xl border cursor-pointer transition-all ${
                      paymentMethod === "MOMO"
                        ? "border-[#D82D8B] bg-[#FFF0F6] shadow-xs"
                        : "border-slate-200 hover:bg-slate-50"
                    }`}
                  >
                    <input
                      type="radio"
                      name="payment"
                      checked={paymentMethod === "MOMO"}
                      onChange={() => setPaymentMethod("MOMO")}
                      className="mt-0.5 text-[#D82D8B] accent-[#D82D8B]"
                    />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <div className="w-5 h-5 rounded-md bg-[#A50064] text-white font-black text-[11px] flex items-center justify-center tracking-tighter shrink-0">
                          M
                        </div>
                        <span className="text-xs font-bold text-[#0D1B2A]">
                          MoMo E-Wallet
                        </span>
                        <span className="px-2 py-0.5 rounded-full bg-pink-100 text-[#A50064] text-[10px] font-bold">
                          QR & App
                        </span>
                      </div>
                      <p className="text-[11px] text-slate-500 mt-1">
                        Scan QR code with MoMo App or pay directly with MoMo
                        E-Wallet.
                      </p>
                    </div>
                  </label>
                </div>
              </div>
            </div>

            {/* Right Column: Order Summary (5 cols) */}
            <div className="lg:col-span-5 space-y-4">
              <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-5 sticky top-24">
                <h2 className="text-base font-bold text-[#0D1B2A]">
                  Order Items ({selectedItems.length})
                </h2>

                {/* Items List */}
                <div className="max-h-60 overflow-y-auto divide-y divide-slate-100 pr-1">
                  {selectedItems.map((item) => (
                    <div key={item.id} className="py-3 flex items-center gap-3">
                      <div className="w-12 h-12 rounded-lg bg-slate-100 overflow-hidden shrink-0 border border-slate-200/60 flex items-center justify-center">
                        {item.image ? (
                          <img
                            src={getImageUrl(item.image)}
                            alt={item.name}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <ShoppingCart className="w-4 h-4 text-slate-400" />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-xs font-bold text-[#0D1B2A] truncate">
                          {item.name}
                        </p>
                        <p className="text-[11px] text-slate-500">
                          Qty: {item.quantity} × {formatCurrency(item.price)}
                        </p>
                      </div>
                      <span className="text-xs font-bold text-[#0D1B2A]">
                        {formatCurrency(item.price * item.quantity)}
                      </span>
                    </div>
                  ))}
                </div>

                {/* Summary Calculations */}
                <div className="space-y-2 pt-3 border-t border-slate-100 text-xs text-slate-600">
                  <div className="flex justify-between">
                    <span>Subtotal</span>
                    <span className="font-semibold text-[#0D1B2A]">
                      {formatCurrency(subtotal)}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span>Shipping (GHN Express)</span>
                    <span className="font-semibold text-[#0D1B2A]">
                      {shippingFee === 0 ? (
                        <span className="text-emerald-600 font-bold">FREE</span>
                      ) : (
                        formatCurrency(shippingFee)
                      )}
                    </span>
                  </div>
                  <div className="flex justify-between text-base font-black text-[#0D1B2A] pt-3 border-t border-slate-200">
                    <span>Total Payment</span>
                    <span className="text-[#0D47A1]">
                      {formatCurrency(grandTotal)}
                    </span>
                  </div>
                </div>

                {/* Submit Order Button */}
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className={`w-full py-4 px-6 rounded-2xl text-white text-sm font-bold flex items-center justify-center gap-2 transition-all shadow-md active:scale-98 disabled:bg-slate-300 cursor-pointer ${
                    paymentMethod === "MOMO"
                      ? "bg-[#A50064] hover:bg-[#850050]"
                      : "bg-[#2196F3] hover:bg-[#0D47A1]"
                  }`}
                >
                  {isSubmitting ? (
                    <div className="w-5 h-5 rounded-full border-2 border-white border-t-transparent animate-spin" />
                  ) : (
                    <>
                      <span>
                        {paymentMethod === "MOMO"
                          ? "Pay with MoMo"
                          : "Place Order Now"}
                      </span>
                      <ShieldCheck className="w-4 h-4" />
                    </>
                  )}
                </button>

                <p className="text-[11px] text-slate-400 text-center leading-relaxed">
                  By placing your order, you agree to GOshop&apos;s Terms of
                  Service and Privacy Policy.
                </p>
              </div>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}
