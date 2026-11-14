"use client";

import React, { useEffect, useRef, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth-store";
import { API_BASE_URL, API_KEY } from "@/lib/api";
import { clearDeviceIntent } from "@/lib/device-intent";

interface GoogleLoginButtonProps {
  text?: "signin_with" | "signup_with" | "continue_with";
  onSuccess?: () => void;
  onError?: (errorMsg: string) => void;
}

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: {
            client_id: string;
            callback: (response: { credential: string }) => void;
            auto_select?: boolean;
            cancel_on_tap_outside?: boolean;
          }) => void;
          renderButton: (
            parent: HTMLElement,
            options: {
              type?: "standard" | "icon";
              theme?: "outline" | "filled_blue" | "filled_black";
              size?: "large" | "medium" | "small";
              text?: "signin_with" | "signup_with" | "continue_with";
              shape?: "rectangular" | "pill" | "circle" | "square";
              logo_alignment?: "left" | "center";
              width?: number | string;
            },
          ) => void;
          prompt: () => void;
        };
      };
    };
  }
}

// GSI chỉ có MỘT config toàn cục cho cả trang, nên trạng thái "đã initialize"
// phải nằm ở cấp module chứ không phải trong từng instance component.
let gsiInitializedClientId: string | null = null;

// Handler mới nhất của instance đang mount. Callback đăng ký với GSI là một
// hàm cố định luôn gọi vào biến này, nên không bao giờ bị stale closure.
let latestCredentialHandler: ((r: { credential: string }) => void) | null =
  null;

const GSI_SOURCE = "https://accounts.google.com/gsi/client?hl=en";

export function GoogleLoginButton({
  text = "continue_with",
  onSuccess,
  onError,
}: GoogleLoginButtonProps) {
  const router = useRouter();
  const { setTokens, setUser } = useAuthStore();
  const buttonRef = useRef<HTMLDivElement>(null);
  const [loading, setLoading] = useState(false);

  const clientId = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID;

  // Giữ onSuccess/onError mới nhất mà không làm đổi identity của handler,
  // nên component cha truyền hàm inline cũng không kích hoạt initialize lại.
  const onSuccessRef = useRef(onSuccess);
  const onErrorRef = useRef(onError);
  useEffect(() => {
    onSuccessRef.current = onSuccess;
    onErrorRef.current = onError;
  });

  const handleCredentialResponse = useCallback(
    async (response: { credential: string }) => {
      const idToken = response.credential;
      if (!idToken) {
        onErrorRef.current?.(
          "Did not receive Google credentials. Please try again.",
        );
        return;
      }

      setLoading(true);
      try {
        const res = await fetch(`${API_BASE_URL}/api/v1/auth/google-login`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-API-KEY": API_KEY,
            "X-Client-App": "storefront",
          },
          credentials: "include",
          body: JSON.stringify({ id_token: idToken }),
        });

        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            data?.message ||
              "Google login failed. Please verify your credentials.",
          );
        }

        const accessToken =
          data.data?.access_token || data.data?.accessToken || "";
        // Refresh token arrives as an HttpOnly cookie; never stored client-side.
        setTokens(accessToken);
        // Signing in makes this device's history the account's again.
        clearDeviceIntent();

        // Fetch user profile
        try {
          const profileRes = await fetch(`${API_BASE_URL}/api/v1/users/me`, {
            headers: {
              Authorization: `Bearer ${accessToken}`,
              "X-API-KEY": API_KEY,
            },
          });
          if (profileRes.ok) {
            const profileData = await profileRes.json();
            const p = profileData.data;
            setUser({
              uuid: p.uuid || p.user_uuid,
              email: p.email || p.user_email,
              fullName: p.full_name || p.user_fullname,
              avatarUrl: p.avatar_url || "",
              role: p.roles || (p.user_level === 1 ? ["admin"] : ["member"]),
            });
          }
        } catch (profileErr) {
          console.warn(
            "Failed to fetch user profile after Google login:",
            profileErr,
          );
        }

        if (onSuccessRef.current) {
          onSuccessRef.current();
        } else {
          const redirectUrl =
            new URLSearchParams(window.location.search).get("redirect") || "/";
          router.push(redirectUrl);
        }
      } catch (err: unknown) {
        const message =
          err instanceof Error ? err.message : "Google authentication failed";
        onErrorRef.current?.(message);
      } finally {
        setLoading(false);
      }
    },
    // Không còn onSuccess/onError trong deps
    [router, setTokens, setUser],
  );

  // Luôn đăng ký handler mới nhất vào biến module.
  // Effect này phải khai báo TRƯỚC effect load script.
  useEffect(() => {
    latestCredentialHandler = handleCredentialResponse;
  }, [handleCredentialResponse]);

  const renderGoogleButton = useCallback(() => {
    if (!window.google?.accounts?.id || !buttonRef.current || !clientId) return;

    try {
      // Chỉ initialize đúng 1 lần cho cả trang (theo clientId).
      if (gsiInitializedClientId !== clientId) {
        window.google.accounts.id.initialize({
          client_id: clientId,
          callback: (r) => latestCredentialHandler?.(r),
        });
        gsiInitializedClientId = clientId;
      }

      buttonRef.current.innerHTML = "";
      window.google.accounts.id.renderButton(buttonRef.current, {
        type: "standard",
        theme: "outline",
        size: "large",
        text,
        shape: "pill",
        width: 320,
      });
    } catch (err) {
      console.error("Google button render error:", err);
    }
  }, [clientId, text]);

  // Load GSI một lần mỗi trang. Thẻ script thường, tìm theo src, nên idempotent:
  // lần mount thứ hai chỉ chờ lần load đầu tiên.
  useEffect(() => {
    let cancelled = false;
    const ready = () => {
      if (!cancelled) renderGoogleButton();
    };

    if (window.google?.accounts?.id) {
      ready();
      return () => {
        cancelled = true;
      };
    }

    let script = document.querySelector<HTMLScriptElement>(
      `script[src="${GSI_SOURCE}"]`,
    );
    if (!script) {
      script = document.createElement("script");
      script.src = GSI_SOURCE;
      script.async = true;
      script.defer = true;
      script.addEventListener("error", () =>
        console.error("Failed to load Google Identity Services"),
      );
      document.head.appendChild(script);
    }
    script.addEventListener("load", ready);

    // GSI có thể đã nằm trong cache và không bắn lại sự kiện load.
    const interval = setInterval(() => {
      if (window.google?.accounts?.id) {
        clearInterval(interval);
        ready();
      }
    }, 150);
    const timer = setTimeout(() => clearInterval(interval), 8000);

    return () => {
      cancelled = true;
      clearInterval(interval);
      clearTimeout(timer);
      script?.removeEventListener("load", ready);
    };
  }, [renderGoogleButton]);

  return (
    <div className="w-full flex flex-col items-center">
      {loading && (
        <div className="w-full py-2.5 px-4 rounded-full border border-slate-200 bg-slate-50 flex items-center justify-center gap-2 text-xs font-semibold text-slate-600">
          <div className="w-4 h-4 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
          <span>Signing in with Google...</span>
        </div>
      )}
      {/* Luôn giữ div trong DOM, chỉ ẩn khi loading, để nút Google không bị mất */}
      <div
        ref={buttonRef}
        className={`w-full flex justify-center min-h-[44px] ${loading ? "hidden" : ""}`}
      />
    </div>
  );
}
