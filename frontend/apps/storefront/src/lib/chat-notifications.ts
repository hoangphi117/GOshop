/**
 * Audio and Web Notifications for Storefront Customer Chat
 */

/**
 * Play a crisp, gentle "ding" chime using standard Web Audio API.
 * No external audio files or network requests required.
 */
export function playNotificationSound() {
  try {
    const AudioCtx =
      window.AudioContext ||
      (window as unknown as { webkitAudioContext: typeof AudioContext })
        .webkitAudioContext;
    if (!AudioCtx) return;

    const ctx = new AudioCtx();
    if (ctx.state === "suspended") {
      ctx.resume().catch(() => {});
    }

    const now = ctx.currentTime;
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();

    // Two-tone bell chime: start at 880Hz (A5), fast glide to 1760Hz (A6)
    osc.type = "sine";
    osc.frequency.setValueAtTime(880, now);
    osc.frequency.exponentialRampToValueAtTime(1760, now + 0.06);

    gain.gain.setValueAtTime(0.2, now);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.35);

    osc.connect(gain);
    gain.connect(ctx.destination);

    osc.start(now);
    osc.stop(now + 0.35);
  } catch {
    // AudioContext blocked by autoplay policy before initial interaction
  }
}

/**
 * Request desktop notification permission from user.
 */
export async function requestNotificationPermission(): Promise<NotificationPermission> {
  if (typeof window === "undefined" || !("Notification" in window)) {
    return "denied";
  }
  if (Notification.permission === "default") {
    try {
      return await Notification.requestPermission();
    } catch {
      return "denied";
    }
  }
  return Notification.permission;
}

/**
 * Display a native browser notification if user is in background or permission granted.
 */
export function showChatWebNotification({
  title = "GOshop Support",
  body,
  tag = "goshop-chat",
  onClick,
}: {
  title?: string;
  body: string;
  tag?: string;
  onClick?: () => void;
}) {
  if (typeof window === "undefined" || !("Notification" in window)) return;
  if (Notification.permission !== "granted") return;

  try {
    const notification = new Notification(title, {
      body,
      tag,
      icon: "/favicon.ico",
    });

    notification.onclick = () => {
      window.focus();
      onClick?.();
      notification.close();
    };
  } catch {
    // Ignore notification error (e.g. Android WebView limitations)
  }
}

/**
 * Format notification body text based on message content type.
 */
export function formatMessageNotificationSnippet(
  contentType: string,
  content?: string,
  attachmentName?: string,
): string {
  if (contentType === "image") return "Sent you an image";
  if (contentType === "video") return "Sent you a video";
  if (contentType === "media_group") return "Sent you attachments";
  if (contentType === "order_ref") return "Sent an order update";
  if (contentType === "product_ref") {
    const parts = (attachmentName || "").split("|");
    const name = parts[0] || "product";
    return `Shared product: ${name}`;
  }
  return content?.trim() || "Sent you a message";
}
