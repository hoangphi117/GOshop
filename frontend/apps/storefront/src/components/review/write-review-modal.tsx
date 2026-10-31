"use client";

import React, { useState, useRef } from "react";
import { directUpload } from "@goshop/api-client";
import { authFetch } from "@/lib/api";
import { extractErrorMessage } from "@/lib/utils";
import {
  X,
  Star,
  Upload,
  Image as ImageIcon,
  Film,
  Trash2,
  Loader2,
  CheckCircle2,
  AlertCircle,
} from "lucide-react";
import { toast } from "sonner";

interface WriteReviewModalProps {
  isOpen: boolean;
  onClose: () => void;
  productUuid: string;
  productName: string;
  productImageUrl?: string;
  orderUuid?: string;
  eligibleOrders?: Array<{
    order_uuid: string;
    order_number: string;
    created_at?: string;
  }>;
  onSuccess?: () => void;
}

const RATING_LABELS: Record<number, { text: string; color: string }> = {
  1: { text: "Poor - Very Dissatisfied", color: "text-rose-500" },
  2: { text: "Fair - Needs Improvement", color: "text-amber-500" },
  3: { text: "Average - Met Expectations", color: "text-yellow-600" },
  4: { text: "Good - Highly Satisfied", color: "text-emerald-500" },
  5: { text: "Excellent - Loved It!", color: "text-emerald-600" },
};

export function WriteReviewModal({
  isOpen,
  onClose,
  productUuid,
  productName,
  productImageUrl,
  orderUuid: initialOrderUuid,
  eligibleOrders,
  onSuccess,
}: WriteReviewModalProps) {
  const [rating, setRating] = useState<number>(5);
  const [hoverRating, setHoverRating] = useState<number | null>(null);
  const [title, setTitle] = useState("");
  const [comment, setComment] = useState("");
  const [selectedOrderUuid, setSelectedOrderUuid] = useState<string>(
    initialOrderUuid || eligibleOrders?.[0]?.order_uuid || "",
  );
  const [files, setFiles] = useState<File[]>([]);
  const [previews, setPreviews] = useState<
    Array<{ url: string; isVideo: boolean; file: File }>
  >([]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  if (!isOpen) return null;

  const handleRatingClick = (star: number) => {
    setRating(star);
  };

  const handleFilesSelected = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files) return;
    const selectedFiles = Array.from(e.target.files);
    setErrorMsg(null);

    // Validate media counts
    const currentImages = previews.filter((p) => !p.isVideo).length;
    const currentVideos = previews.filter((p) => p.isVideo).length;

    const newPreviews: Array<{ url: string; isVideo: boolean; file: File }> =
      [];
    const newFiles: File[] = [];

    let addedImages = 0;
    let addedVideos = 0;

    for (const file of selectedFiles) {
      const isImage = file.type.startsWith("image/");
      const isVideo = file.type.startsWith("video/");

      if (!isImage && !isVideo) {
        toast.error(`"${file.name}" is not a supported image or video file.`);
        continue;
      }

      if (isImage) {
        if (currentImages + addedImages >= 5) {
          toast.error("You can upload a maximum of 5 images.");
          continue;
        }
        addedImages++;
      } else if (isVideo) {
        if (currentVideos + addedVideos >= 1) {
          toast.error("You can upload a maximum of 1 video.");
          continue;
        }
        addedVideos++;
      }

      if (file.size > 20 * 1024 * 1024) {
        toast.error(`"${file.name}" exceeds 20MB file size limit.`);
        continue;
      }

      const url = URL.createObjectURL(file);
      newPreviews.push({ url, isVideo, file });
      newFiles.push(file);
    }

    setPreviews((prev) => [...prev, ...newPreviews]);
    setFiles((prev) => [...prev, ...newFiles]);

    // Reset input
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleRemoveFile = (index: number) => {
    const previewToRemove = previews[index];
    if (previewToRemove) {
      URL.revokeObjectURL(previewToRemove.url);
    }
    setPreviews((prev) => prev.filter((_, i) => i !== index));
    setFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);

    const finalOrderUuid = initialOrderUuid || selectedOrderUuid;
    if (!finalOrderUuid) {
      setErrorMsg(
        "Please select a verified purchase order to attach your review.",
      );
      return;
    }

    if (rating < 1 || rating > 5) {
      setErrorMsg("Please select a rating score between 1 and 5 stars.");
      return;
    }

    setIsSubmitting(true);

    try {
      // 1. Upload files to /api/v1/upload (folder='reviews') if any
      const uploadedImageUrls: string[] = [];
      let uploadedVideoUrl: string | undefined = undefined;

      if (files.length > 0) {
        for (const file of files) {
          let fileUrl: string;
          try {
            const uploaded = await directUpload(file, "reviews", authFetch);
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

      // 2. Submit pure JSON review
      const res = await authFetch("/api/v1/reviews", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          product_uuid: productUuid,
          order_uuid: finalOrderUuid,
          rating,
          title: title.trim() || undefined,
          comment: comment.trim() || undefined,
          images: uploadedImageUrls,
          video_url: uploadedVideoUrl,
        }),
      });

      if (res.ok) {
        toast.success("Thank you! Your product review has been submitted.");
        onSuccess?.();
        onClose();
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(errJson, "Failed to submit review.");
        setErrorMsg(msg);
        toast.error(msg);
      }
    } catch {
      const msg = "Network error occurred while submitting your review.";
      setErrorMsg(msg);
      toast.error(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  const currentDisplayRating = hoverRating || rating;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-fade-in">
      <div
        className="w-full max-w-xl max-h-[90vh] bg-white rounded-md shadow-xl border border-[#E2E8F0] flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="px-5 py-4 border-b border-[#E2E8F0] flex items-center justify-between bg-[#F8FAFC]">
          <div className="flex items-center gap-3 min-w-0">
            {productImageUrl && (
              <img
                src={productImageUrl}
                alt={productName}
                className="w-10 h-10 rounded-md object-cover border border-[#E2E8F0] shrink-0"
              />
            )}
            <div className="min-w-0">
              <h2 className="font-heading text-sm sm:text-base font-bold text-[#0F172A] truncate">
                Rate & Review Product
              </h2>
              <p className="text-xs text-[#64748B] truncate">{productName}</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 rounded-md flex items-center justify-center text-[#64748B] hover:text-[#0F172A] hover:bg-[#E2E8F0]/60 transition-colors cursor-pointer"
            aria-label="Close review dialog"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Content Form */}
        <form
          onSubmit={handleSubmit}
          className="flex-1 overflow-y-auto p-5 space-y-4"
        >
          {errorMsg && (
            <div className="p-3 rounded-md bg-rose-50 border border-rose-200 text-rose-700 text-xs flex items-start gap-2.5">
              <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
              <span>{errorMsg}</span>
            </div>
          )}

          {/* Order Selection if not fixed */}
          {!initialOrderUuid && eligibleOrders && eligibleOrders.length > 0 && (
            <div className="space-y-1.5">
              <label className="text-xs font-bold text-[#0F172A]">
                Associated Purchase Order{" "}
                <span className="text-rose-500">*</span>
              </label>
              <select
                value={selectedOrderUuid}
                onChange={(e) => setSelectedOrderUuid(e.target.value)}
                className="w-full px-3 py-2 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] text-xs font-medium text-[#0F172A] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] transition-all"
                required
              >
                {eligibleOrders.map((ord) => (
                  <option key={ord.order_uuid} value={ord.order_uuid}>
                    Order #{ord.order_number}{" "}
                    {ord.created_at ? `(${ord.created_at})` : ""}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Star Rating Selector */}
          <div className="p-4 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] flex flex-col items-center justify-center gap-2 text-center">
            <span className="font-heading text-xs font-bold text-[#0F172A] uppercase tracking-wider">
              Overall Rating
            </span>
            <div className="flex items-center gap-2">
              {[1, 2, 3, 4, 5].map((star) => (
                <button
                  key={star}
                  type="button"
                  onClick={() => handleRatingClick(star)}
                  onMouseEnter={() => setHoverRating(star)}
                  onMouseLeave={() => setHoverRating(null)}
                  className="p-1 text-amber-400 hover:scale-110 transition-transform cursor-pointer focus:outline-none"
                  aria-label={`Rate ${star} star`}
                >
                  <Star
                    className={`w-7 h-7 ${
                      star <= currentDisplayRating
                        ? "fill-amber-400 text-amber-400"
                        : "text-slate-200"
                    }`}
                  />
                </button>
              ))}
            </div>
            <span
              className={`text-xs font-bold ${
                RATING_LABELS[currentDisplayRating]?.color || "text-[#64748B]"
              }`}
            >
              {RATING_LABELS[currentDisplayRating]?.text}
            </span>
          </div>

          {/* Title */}
          <div className="space-y-1.5">
            <label className="text-xs font-bold text-[#0F172A]">
              Review Title{" "}
              <span className="text-[#64748B] font-normal">(Optional)</span>
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={200}
              placeholder="e.g. Great quality and lightning fast delivery!"
              className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs font-medium text-[#0F172A] placeholder:text-[#94A3B8] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] shadow-2xs transition-all"
            />
          </div>

          {/* Comment */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <label className="text-xs font-bold text-[#0F172A]">
                Review Experience{" "}
                <span className="text-[#64748B] font-normal">(Optional)</span>
              </label>
              <span className="text-[10px] text-[#94A3B8]">
                {comment.length}/2000
              </span>
            </div>
            <textarea
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              maxLength={2000}
              rows={4}
              placeholder="What did you like or dislike? How was the fit, material, packaging or customer support?"
              className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs font-medium text-[#0F172A] placeholder:text-[#94A3B8] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] shadow-2xs transition-all resize-none"
            />
          </div>

          {/* Media Uploads */}
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="font-bold text-[#0F172A]">Photos & Video</span>
              <span className="text-[#64748B] text-[11px]">
                Max 5 images, 1 video (20MB max)
              </span>
            </div>

            {/* Media Preview Grid */}
            <div className="grid grid-cols-4 gap-2.5">
              {previews.map((preview, index) => (
                <div
                  key={index}
                  className="relative group aspect-square rounded-md overflow-hidden border border-[#E2E8F0] bg-[#F8FAFC]"
                >
                  {preview.isVideo ? (
                    <video
                      src={preview.url}
                      className="w-full h-full object-cover"
                      muted
                      playsInline
                    />
                  ) : (
                    <img
                      src={preview.url}
                      alt="Upload preview"
                      className="w-full h-full object-cover"
                    />
                  )}

                  <div className="absolute top-1 left-1 px-1.5 py-0.5 rounded bg-black/60 text-white text-[9px] font-bold flex items-center gap-1">
                    {preview.isVideo ? (
                      <>
                        <Film className="w-2.5 h-2.5" />
                        <span>Video</span>
                      </>
                    ) : (
                      <>
                        <ImageIcon className="w-2.5 h-2.5" />
                        <span>Photo</span>
                      </>
                    )}
                  </div>

                  <button
                    type="button"
                    onClick={() => handleRemoveFile(index)}
                    className="absolute top-1 right-1 w-6 h-6 rounded-full bg-rose-600 text-white flex items-center justify-center opacity-90 hover:opacity-100 transition-opacity shadow-xs cursor-pointer"
                    aria-label="Remove uploaded media"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              ))}

              {previews.length < 6 && (
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  className="aspect-square rounded-md border-2 border-dashed border-[#CBD5E1] hover:border-[#007D9C] bg-[#F8FAFC] hover:bg-[#F0F9FB] text-[#64748B] hover:text-[#007D9C] flex flex-col items-center justify-center gap-1 transition-all cursor-pointer"
                >
                  <Upload className="w-4 h-4" />
                  <span className="text-[10px] font-bold">Add Media</span>
                </button>
              )}
            </div>

            <input
              type="file"
              ref={fileInputRef}
              onChange={handleFilesSelected}
              multiple
              accept="image/*,video/*"
              className="hidden"
            />
          </div>
          {/* Actions */}
          <div className="pt-3 border-t border-[#E2E8F0] flex items-center justify-end gap-2.5">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="px-4 py-2 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] text-[#0F172A] text-xs font-semibold transition-all cursor-pointer disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="inline-flex items-center justify-center gap-2 px-5 py-2 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-xs font-bold transition-all shadow-xs disabled:opacity-50 cursor-pointer"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Submitting...</span>
                </>
              ) : (
                <span>Submit Review</span>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
