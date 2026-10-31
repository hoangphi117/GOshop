"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { authFetch } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/format";
import {
  RotateCcw,
  Search,
  Clock,
  CheckCircle2,
  XCircle,
  ShieldCheck,
  X,
  ChevronRight,
  ChevronLeft,
  ChevronsLeft,
  ChevronsRight,
  Loader2,
  Package,
  ArrowRight,
  Eye,
  Truck,
  Building2,
  FileText,
  Shield,
  Plus,
} from "lucide-react";
import { ReturnDetailModal } from "@/components/order/return-detail-modal";

export interface UserReturnSummary {
  return_uuid: string;
  order_uuid: string;
  order_number: string;
  total_amount: number;
  payment_method: string;
  payment_status: string;
  reason: string;
  images: string[];
  video_url?: string | null;
  status: number; // 1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled
  status_name: string;
  status_text: string;
  admin_note?: string | null;
  shipping_provider?: string | null;
  shipping_code?: string | null;
  created_at: string;
  updated_at: string;
}

const STATUS_FILTERS = [
  { label: "All Requests", value: 0 },
  { label: "Pending Approval", value: 1 },
  { label: "Approved & Shipping", value: 2 },
  { label: "Completed / Refunded", value: 4 },
  { label: "Rejected", value: 3 },
  { label: "Cancelled", value: 5 },
];

export default function MyReturnsPage() {
  const [returns, setReturns] = useState<UserReturnSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedStatus, setSelectedStatus] = useState<number>(0);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [currentPage, setCurrentPage] = useState(1);
  const limit = 10;
  const [pagination, setPagination] = useState({
    page: 1,
    limit: 10,
    total_records: 0,
    total_pages: 1,
  });

  // Selected return to view in modal
  const [selectedOrderUuid, setSelectedOrderUuid] = useState<string | null>(
    null,
  );

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery);
      setCurrentPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  const fetchReturns = useCallback(async () => {
    setLoading(true);
    try {
      const query = new URLSearchParams();
      query.set("page", String(currentPage));
      query.set("limit", String(limit));
      if (selectedStatus > 0) {
        query.set("status", String(selectedStatus));
      }
      if (debouncedSearch.trim()) {
        query.set("search", debouncedSearch.trim());
      }

      const res = await authFetch(`/api/v1/orders/returns?${query.toString()}`);
      if (res.ok) {
        const json = await res.json();
        setReturns(json.data || []);
        if (json.pagination) {
          setPagination({
            page: json.pagination.page,
            limit: json.pagination.limit,
            total_records: json.pagination.total_records,
            total_pages: json.pagination.total_pages || 1,
          });
        }
      }
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [currentPage, limit, selectedStatus, debouncedSearch]);

  useEffect(() => {
    fetchReturns();
  }, [fetchReturns]);

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
    <div className="space-y-6 font-body">
      {/* Top Header Card */}
      <section className="bg-white border border-[#E2E8F0] rounded-md p-6">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <h1 className="text-xl sm:text-2xl font-bold text-[#0F172A] font-heading tracking-tight">
              Returns & Refunds
            </h1>
            <p className="text-xs sm:text-sm text-[#64748B] mt-1">
              Manage product return requests, track return parcel shipments, and
              verify refund payouts.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-3 shrink-0">
            <div className="bg-[#F8FAFC] text-[#0F172A] border border-[#E2E8F0] px-3 py-2 rounded-md text-xs font-heading font-medium flex items-center gap-1.5">
              <Shield className="w-4 h-4 text-[#007D9C]" />
              <span>30-Day Policy</span>
            </div>
            <Link
              href="/account/orders"
              className="bg-[#007D9C] hover:bg-[#006782] text-white font-heading font-semibold px-4 py-2 rounded-md text-xs sm:text-sm flex items-center gap-1.5"
            >
              <Plus className="w-4 h-4" />
              <span>Request Return</span>
            </Link>
          </div>
        </div>

        {/* Status Filter Tabs */}
        <div className="flex items-center gap-2 overflow-x-auto pb-1 border-t border-[#E2E8F0] pt-4 mt-6">
          {STATUS_FILTERS.map((f) => {
            const isActive = selectedStatus === f.value;
            return (
              <button
                key={f.value}
                type="button"
                onClick={() => {
                  setSelectedStatus(f.value);
                  setCurrentPage(1);
                }}
                className={`px-3.5 py-1.5 rounded-md text-xs font-semibold whitespace-nowrap cursor-pointer ${
                  isActive
                    ? "bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/40 font-heading"
                    : "bg-white border border-[#E2E8F0] text-[#64748B] hover:text-[#0F172A] hover:border-slate-300"
                }`}
              >
                {f.label}
              </button>
            );
          })}
        </div>
      </section>

      {/* Search Toolbar */}
      <div className="relative">
        <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-[#64748B]" />
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          placeholder="Search by order number or return reason..."
          className="w-full pl-10 pr-4 py-2.5 rounded-md border border-[#E2E8F0] bg-white text-xs text-[#0F172A] placeholder-[#64748B] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#00ADD8]"
        />
      </div>

      {/* Main List Content */}
      {loading ? (
        <div className="py-20 flex flex-col items-center justify-center space-y-3 bg-white border border-[#E2E8F0] rounded-md">
          <Loader2 className="w-7 h-7 text-[#007D9C] animate-spin" />
          <p className="text-xs text-[#64748B] font-semibold">
            Loading return requests...
          </p>
        </div>
      ) : returns.length === 0 ? (
        <div className="py-16 text-center space-y-4 bg-white rounded-md border border-[#E2E8F0] p-8">
          <div className="w-14 h-14 rounded-md bg-[#F8FAFC] text-[#64748B] border border-[#E2E8F0] flex items-center justify-center mx-auto">
            <RotateCcw className="w-6 h-6 text-[#007D9C]" />
          </div>
          <div className="space-y-1">
            <h3 className="text-base font-bold text-[#0F172A] font-heading">
              No return requests found
            </h3>
            <p className="text-xs text-[#64748B] max-w-sm mx-auto">
              You do not have any return or refund requests matching the
              selected filter.
            </p>
          </div>
          <Link
            href="/account/orders"
            className="inline-flex items-center gap-2 px-5 py-2.5 rounded-md bg-[#007D9C] hover:bg-[#006782] text-white text-xs font-semibold"
          >
            <span>View Delivered Orders</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      ) : (
        <div className="space-y-4">
          {returns.map((ret) => (
            <article
              key={ret.return_uuid}
              className="bg-white rounded-md border border-[#E2E8F0] p-5 sm:p-6 space-y-4"
            >
              {/* Card Header Row */}
              <div className="flex flex-col sm:flex-row sm:items-center justify-between pb-4 border-b border-[#E2E8F0] gap-2">
                <div className="flex flex-wrap items-center gap-3">
                  <span className="font-heading font-bold text-sm text-[#0F172A]">
                    Return #{ret.return_uuid.slice(0, 8).toUpperCase()}
                  </span>
                  <span className="text-[#E2E8F0]">•</span>
                  <span className="text-xs text-[#64748B]">
                    Linked Order:{" "}
                    <strong className="text-[#0F172A]">
                      #
                      {ret.order_number ||
                        ret.order_uuid.slice(0, 8).toUpperCase()}
                    </strong>
                  </span>
                  <span className="text-[#E2E8F0]">•</span>
                  <span className="text-xs text-[#64748B]">
                    Requested:{" "}
                    <span className="text-[#0F172A]">
                      {formatDate(ret.created_at)}
                    </span>
                  </span>
                </div>

                <div>{getStatusBadge(ret.status, ret.status_text)}</div>
              </div>

              {/* Card Body */}
              <div className="grid grid-cols-1 md:grid-cols-12 gap-4 text-xs">
                {/* Left info (8 cols) */}
                <div className="md:col-span-8 space-y-2">
                  <div className="space-y-1">
                    <span className="text-[#64748B] font-semibold block text-[11px]">
                      Return Reason
                    </span>
                    <p className="text-[#0F172A] font-medium leading-relaxed">
                      {ret.reason}
                    </p>
                  </div>

                  {/* Attached images preview */}
                  {ret.images && ret.images.length > 0 && (
                    <div className="flex items-center gap-2 pt-1">
                      <span className="text-[11px] text-[#64748B]">
                        Proof ({ret.images.length}):
                      </span>
                      <div className="flex items-center gap-1.5">
                        {ret.images.slice(0, 3).map((img, i) => {
                          const imageUrl = img.startsWith("http")
                            ? img
                            : `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}${img}`;
                          return (
                            <a
                              key={i}
                              href={imageUrl}
                              target="_blank"
                              rel="noreferrer"
                              className="w-8 h-8 rounded-md border border-[#E2E8F0] overflow-hidden bg-[#F8FAFC] inline-block"
                            >
                              {/* eslint-disable-next-line @next/next/no-img-element */}
                              <img
                                src={imageUrl}
                                alt={`Proof ${i + 1}`}
                                className="w-full h-full object-cover"
                              />
                            </a>
                          );
                        })}
                      </div>
                    </div>
                  )}

                  {/* Shipment pill if code exists */}
                  {ret.shipping_code && (
                    <div className="flex items-center gap-2 text-[11px] text-[#007D9C] bg-[#E6F7FB] border border-[#00ADD8]/30 px-3 py-1.5 rounded-md w-fit mt-2">
                      <Truck className="w-3.5 h-3.5 shrink-0" />
                      <span>
                        Return Waybill:{" "}
                        <strong className="font-mono font-bold">
                          {ret.shipping_code}
                        </strong>{" "}
                        ({ret.shipping_provider || "Standard Carrier"})
                      </span>
                    </div>
                  )}
                </div>

                {/* Right actions & amount (4 cols) */}
                <div className="md:col-span-4 flex flex-col md:items-end justify-between border-t md:border-t-0 border-[#E2E8F0] pt-3 md:pt-0 gap-3">
                  <div className="text-left md:text-right">
                    <span className="text-[11px] text-text-secondary block">
                      Refund Amount
                    </span>
                    <span className="text-base font-bold text-[#007D9C] font-heading">
                      {formatCurrency(ret.total_amount)}
                    </span>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    {ret.status === 2 && !ret.shipping_code ? (
                      <button
                        type="button"
                        onClick={() => setSelectedOrderUuid(ret.order_uuid)}
                        className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-md bg-[#007D9C] hover:bg-[#006782] text-white text-xs font-semibold cursor-pointer"
                      >
                        <Truck className="w-3.5 h-3.5" />
                        <span>Submit Tracking</span>
                      </button>
                    ) : (
                      <button
                        type="button"
                        onClick={() => setSelectedOrderUuid(ret.order_uuid)}
                        className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-md bg-white hover:bg-[#F8FAFC] text-[#0F172A] border border-[#E2E8F0] text-xs font-semibold cursor-pointer"
                      >
                        <Eye className="w-3.5 h-3.5 text-[#64748B]" />
                        <span>View Details</span>
                      </button>
                    )}
                  </div>
                </div>
              </div>
            </article>
          ))}

          {/* Pagination */}
          {pagination.total_pages > 1 && (
            <div className="flex items-center justify-between gap-4 pt-4 border-t border-[#E2E8F0] flex-wrap">
              <span className="text-xs text-[#64748B]">
                Page {currentPage} of {pagination.total_pages} (
                {pagination.total_records} requests)
              </span>

              <div className="flex items-center gap-1">
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage(1)}
                  className="p-2 rounded-md border border-[#E2E8F0] bg-white disabled:opacity-40 hover:bg-[#F8FAFC]"
                  title="First Page"
                >
                  <ChevronsLeft className="w-4 h-4 text-[#0F172A]" />
                </button>
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  className="p-2 rounded-md border border-[#E2E8F0] bg-white disabled:opacity-40 hover:bg-[#F8FAFC]"
                  title="Previous Page"
                >
                  <ChevronLeft className="w-4 h-4 text-[#0F172A]" />
                </button>
                <span className="px-3 py-1.5 rounded-md bg-[#007D9C] text-white text-xs font-bold font-heading">
                  {currentPage}
                </span>
                <button
                  type="button"
                  disabled={currentPage >= pagination.total_pages || loading}
                  onClick={() =>
                    setCurrentPage((p) =>
                      Math.min(pagination.total_pages, p + 1),
                    )
                  }
                  className="p-2 rounded-md border border-[#E2E8F0] bg-white disabled:opacity-40 hover:bg-[#F8FAFC]"
                  title="Next Page"
                >
                  <ChevronRight className="w-4 h-4 text-[#0F172A]" />
                </button>
                <button
                  type="button"
                  disabled={currentPage >= pagination.total_pages || loading}
                  onClick={() => setCurrentPage(pagination.total_pages)}
                  className="p-2 rounded-md border border-[#E2E8F0] bg-white disabled:opacity-40 hover:bg-[#F8FAFC]"
                  title="Last Page"
                >
                  <ChevronsRight className="w-4 h-4 text-[#0F172A]" />
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Return Detail Modal */}
      {selectedOrderUuid && (
        <ReturnDetailModal
          isOpen={!!selectedOrderUuid}
          onClose={() => setSelectedOrderUuid(null)}
          orderUuid={selectedOrderUuid}
          onUpdated={fetchReturns}
        />
      )}
    </div>
  );
}
