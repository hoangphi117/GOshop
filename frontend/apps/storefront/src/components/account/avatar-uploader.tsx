"use client";

import React, { useRef, useState } from "react";
import { Camera, Trash2, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { directUpload, UploadError } from "@goshop/api-client";
import { authFetch } from "@/lib/api";
import { getImageUrl } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";

/** Matches the presign allowlist; checked here so a wrong pick fails fast. */
const ACCEPTED = "image/jpeg,image/png,image/webp";
const MAX_BYTES = 5 << 20;

interface AvatarUploaderProps {
  /** Current avatar path or URL; empty renders the initial-letter placeholder. */
  value?: string;
  /** Called with the verified avatar URL after the API confirms the change. */
  onUploaded: (url: string) => void;
}

function initialsOf(fullName?: string, email?: string): string {
  const name = (fullName || "").trim();
  if (name) return name[0].toUpperCase();
  const local = (email || "").split("@")[0];
  return (local[0] || "U").toUpperCase();
}

export function AvatarUploader({ value, onUploaded }: AvatarUploaderProps) {
  const { user } = useAuthStore();
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<string | null>(null);

  const shown = preview || value;

  const applyKey = async (fileKey: string) => {
    const res = await authFetch("/api/v1/users/me/avatar", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ file_key: fileKey }),
    });
    const json = await res.json().catch(() => null);
    if (!res.ok) {
      throw new UploadError(
        json?.message || json?.error || "Could not save your avatar",
        res.status,
      );
    }
    return (json?.data?.avatar_url as string) || "";
  };

  const handleFile = async (file: File) => {
    if (!ACCEPTED.split(",").includes(file.type)) {
      toast.error("Please choose a JPEG, PNG, or WebP image.");
      return;
    }
    if (file.size > MAX_BYTES) {
      toast.error("Image is too large (max 5MB).");
      return;
    }

    // Show the picked bytes immediately; the server round-trip replaces them.
    const objectUrl = URL.createObjectURL(file);
    setPreview(objectUrl);
    setBusy(true);

    try {
      const uploaded = await directUpload(file, "avatars", authFetch);
      const url = await applyKey(uploaded.fileKey);
      onUploaded(url || uploaded.url);
      toast.success("Avatar updated.");
    } catch (err: unknown) {
      setPreview(null);
      const msg =
        err instanceof Error ? err.message : "Could not upload your avatar";
      toast.error(msg);
    } finally {
      URL.revokeObjectURL(objectUrl);
      setBusy(false);
    }
  };

  const handleRemove = async () => {
    setBusy(true);
    try {
      // An empty key is the documented "remove my avatar" request.
      await applyKey("");
      setPreview(null);
      onUploaded("");
      toast.success("Avatar removed.");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Could not remove avatar";
      toast.error(msg);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex items-center gap-4">
      <div className="relative w-20 h-20 rounded-full overflow-hidden border border-[#E2E8F0] bg-slate-100 flex items-center justify-center shrink-0">
        {shown ? (
          <img
            src={getImageUrl(shown)}
            alt="Your avatar"
            className="w-full h-full object-cover"
          />
        ) : (
          <span className="text-2xl font-bold text-[#007D9C] font-heading">
            {initialsOf(user?.fullName, user?.email)}
          </span>
        )}
        {busy && (
          <div className="absolute inset-0 bg-white/70 flex items-center justify-center">
            <Loader2 className="w-5 h-5 text-[#007D9C] animate-spin" />
          </div>
        )}
      </div>

      <div className="space-y-2">
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPTED}
          className="hidden"
          onChange={(e) => {
            const file = e.target.files?.[0];
            // Reset so picking the same file twice still fires a change.
            e.target.value = "";
            if (file) void handleFile(file);
          }}
        />
        <div className="flex items-center gap-2">
          <button
            type="button"
            disabled={busy}
            onClick={() => inputRef.current?.click()}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-md border border-[#E2E8F0] bg-white text-xs font-semibold text-[#0F172A] hover:border-[#007D9C] transition-colors disabled:opacity-60 cursor-pointer"
          >
            <Camera className="w-4 h-4" />
            <span>{shown ? "Change photo" : "Upload photo"}</span>
          </button>
          {shown && (
            <button
              type="button"
              disabled={busy}
              onClick={() => void handleRemove()}
              className="inline-flex items-center gap-1 px-3 py-2 rounded-md text-xs font-semibold text-slate-500 hover:text-red-600 transition-colors disabled:opacity-60 cursor-pointer"
            >
              <Trash2 className="w-4 h-4" />
              <span>Remove</span>
            </button>
          )}
        </div>
        <p className="text-[11px] text-slate-400">
          JPEG, PNG, or WebP up to 5MB.
        </p>
      </div>
    </div>
  );
}
