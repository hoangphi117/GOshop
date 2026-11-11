"use client";

import { Bot, Headset, ArrowRight } from "lucide-react";
import type { ChatMode } from "@/stores/chat-ui-store";

// The two choices offered before any conversation exists. Shared by the
// first-time screen and the header switcher so the wording cannot drift.
export const MODE_LABELS: Record<ChatMode, { title: string; blurb: string }> = {
  ai: {
    title: "AI Shopping Assistant",
    blurb: "Instant answers on products, prices, shipping and returns.",
  },
  human: {
    title: "Talk to our team",
    blurb: "Chat with a real advisor for orders and anything the bot can't fix.",
  },
};

export function ModeIcon({ mode, className }: { mode: ChatMode; className?: string }) {
  if (mode === "ai") return <Bot className={className} strokeWidth={1.75} />;
  return <Headset className={className} strokeWidth={1.75} />;
}

/**
 * Shown inside the panel while no mode has been chosen yet and the customer
 * has no history. Picking a card is the only way past this screen.
 */
export function ModeChooser({
  onChoose,
  canUseAi = true,
}: {
  onChoose: (mode: ChatMode) => void;
  /** The AI tab is signed-in only; hide it rather than offer a dead end. */
  canUseAi?: boolean;
}) {
  const modes: ChatMode[] = canUseAi ? ["ai", "human"] : ["human"];
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 p-6">
      <div className="space-y-1 text-center">
        <h4 className="text-sm font-semibold text-slate-900">
          How can we help you today?
        </h4>
        <p className="text-xs text-slate-500">
          {canUseAi
            ? "Pick who you would like to talk to. You can switch at any time."
            : "Chat with a real advisor about orders, products and policies."}
        </p>
      </div>

      <div className="w-full space-y-2">
        {modes.map((mode) => (
          <button
            key={mode}
            type="button"
            onClick={() => onChoose(mode)}
            className="group flex w-full items-center gap-3 rounded-lg border border-slate-200 bg-white p-3 text-left transition-colors hover:border-slate-900 hover:bg-slate-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-900"
          >
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-slate-100 text-slate-700 group-hover:bg-slate-900 group-hover:text-white">
              <ModeIcon mode={mode} className="h-4 w-4" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-medium text-slate-900">
                {MODE_LABELS[mode].title}
              </span>
              <span className="block text-xs leading-snug text-slate-500">
                {MODE_LABELS[mode].blurb}
              </span>
            </span>
            <ArrowRight className="h-4 w-4 shrink-0 text-slate-300 group-hover:text-slate-900" />
          </button>
        ))}
      </div>
    </div>
  );
}

/**
 * Two-way tab bar pinned above the transcript. Uses role=tablist so keyboard
 * and screen-reader users get the same switch the mouse users get.
 */
export function ModeSwitcher({
  mode,
  onChange,
  disabled,
  canUseAi = true,
}: {
  mode: ChatMode;
  onChange: (mode: ChatMode) => void;
  /** Set while the AI is mid-answer: switching would orphan the stream. */
  disabled?: boolean;
  /** Signed-in only; anonymous visitors see the support tab alone. */
  canUseAi?: boolean;
}) {
  const modes: ChatMode[] = canUseAi ? ["ai", "human"] : ["human"];
  return (
    <div
      role="tablist"
      aria-label="Conversation type"
      className="flex shrink-0 gap-1 border-b border-slate-200 bg-slate-100 p-1"
    >
      {modes.map((m) => {
        const active = m === mode;
        return (
          <button
            key={m}
            role="tab"
            type="button"
            aria-selected={active}
            disabled={disabled}
            onClick={() => onChange(m)}
            className={`flex flex-1 items-center justify-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
              active
                ? "bg-white text-slate-900 shadow-xs"
                : "text-slate-500 hover:text-slate-800"
            }`}
          >
            <ModeIcon mode={m} className="h-3.5 w-3.5" />
            {m === "ai" ? "AI Assistant" : "Our Team"}
          </button>
        );
      })}
    </div>
  );
}
