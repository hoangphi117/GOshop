"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import {
  ArrowUp,
  MessageSquare,
  Package,
  RefreshCw,
  UserRound,
  X,
} from "lucide-react";
import { useAssistant } from "@/hooks/use-assistant";
import { useChat } from "@/hooks/use-chat";
import { useChatUIStore } from "@/stores/chat-ui-store";
import type { ChatMode } from "@/stores/chat-ui-store";
import {
  HUMAN_GREETING,
  useHumanGreeting,
} from "@/stores/human-greeting-store";
import { useMounted } from "@/hooks/use-mounted";
import { requestNotificationPermission } from "@/lib/chat-notifications";
import {
  ModeChooser,
  ModeIcon,
  ModeSwitcher,
} from "@/components/chat/mode-switcher";
import { ChatPanel } from "@/components/chat/chat-widget";
import type { AssistantProduct } from "@/lib/assistant-api";

function fmtCurrency(n: number) {
  return new Intl.NumberFormat("vi-VN", {
    style: "currency",
    currency: "VND",
  }).format(n);
}

function ProductCards({ products }: { products: AssistantProduct[] }) {
  if (!products.length) return null;
  return (
    <div className="mt-2 grid gap-1.5">
      {products.slice(0, 4).map((p) => {
        const outOfStock = p.stock <= 0;
        return (
          <Link
            key={p.uuid}
            href={`/products/${p.slug}`}
            className="group flex items-center gap-2 rounded-lg border border-slate-200 bg-white p-1.5 transition-colors hover:border-slate-300"
          >
            <div className="relative h-10 w-10 shrink-0 overflow-hidden rounded-md border border-slate-200 bg-slate-100">
              {p.image_url ? (
                <img
                  src={p.image_url}
                  alt={p.name}
                  className="h-full w-full object-cover"
                />
              ) : (
                <div className="flex h-full w-full items-center justify-center text-slate-400">
                  <Package className="h-4 w-4" strokeWidth={1.5} />
                </div>
              )}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-semibold text-slate-800 group-hover:underline">
                {p.name}
              </p>
              <div className="mt-0.5 flex items-center justify-between gap-1">
                <span className="text-xs font-bold text-slate-900">
                  {fmtCurrency(p.price)}
                </span>
                <span
                  className={`text-[10px] font-medium ${
                    outOfStock ? "text-red-600" : "text-emerald-700"
                  }`}
                >
                  {outOfStock ? "Out of stock" : `In stock (${p.stock})`}
                </span>
              </div>
            </div>
          </Link>
        );
      })}
    </div>
  );
}

// Questions the seeded catalog and FAQ corpus actually answer. Kept short and
// English like the rest of the UI; Vietnamese input works too because the tools
// are told to translate keywords.
// ponytail: a fixed list, no config endpoint. Move it to the catalog if the
// merchandising team ever needs to change it without a deploy.
const QUICK_QUESTIONS = [
  "Do you sell laptops?",
  "Show me cameras for vlogging",
  "What is your return policy?",
  "How much is shipping?",
  "Is cash on delivery available?",
];

/** AI half of the panel: transcript, product cards, composer. */
function AiThread({
  onSwitchToHuman,
  isAuthenticated,
}: {
  onSwitchToHuman: () => void;
  isAuthenticated: boolean;
}) {
  const { messages, isLoading, ask, greet } = useAssistant();
  const [input, setInput] = useState("");
  const endRef = useRef<HTMLDivElement | null>(null);

  // First-time greeting: a local message, so it renders with no network call
  // and costs nothing against the daily quota.
  useEffect(() => {
    greet();
  }, [greet]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "auto" });
  }, [messages.length, isLoading]);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const text = input.trim();
    if (!text) return;
    setInput("");
    void ask(text);
  };

  // The assistant route is signed-in only (401 otherwise), so gate the composer
  // instead of letting the customer type a question that cannot be sent.
  if (!isAuthenticated) {
    return (
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 bg-slate-50 p-6 text-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-slate-200 text-slate-600">
          <UserRound className="h-6 w-6" />
        </div>
        <div className="space-y-1">
          <h4 className="text-sm font-semibold text-slate-900">
            Sign in to chat
          </h4>
          <p className="text-xs leading-relaxed text-slate-500">
            The AI assistant keeps your conversation and your answers in one
            place, so it needs an account.
          </p>
        </div>
        <Link
          href="/login"
          className="inline-flex items-center justify-center rounded bg-slate-900 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-slate-800"
        >
          Log In
        </Link>
        <button
          type="button"
          onClick={onSwitchToHuman}
          className="text-xs font-medium text-slate-600 underline underline-offset-2 hover:text-slate-900"
        >
          Or talk to a staff member
        </button>
      </div>
    );
  }

  return (
    <>
      <div className="min-w-0 flex-1 space-y-3 overflow-y-auto overflow-x-hidden bg-slate-50 p-4">
        {messages.map((m) => {
          const isUser = m.role === "user";
          return (
            <div
              key={m.id}
              className={`flex ${isUser ? "justify-end" : "justify-start"} animate-fade-up motion-reduce:animate-none`}
            >
              <div
                className={`max-w-[85%] rounded-lg px-3 py-2 text-sm leading-relaxed ${
                  isUser
                    ? "rounded-br-none bg-slate-900 text-white"
                    : m.error
                      ? "border border-red-200 bg-red-50 text-red-800"
                      : "rounded-bl-none border border-slate-200 bg-white text-slate-900"
                }`}
              >
                {m.pending ? (
                  <span className="flex items-center gap-1 py-1">
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-slate-400 [animation-delay:-0.2s]" />
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-slate-400 [animation-delay:-0.1s]" />
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-slate-400" />
                  </span>
                ) : (
                  <p className="whitespace-pre-wrap break-words">{m.content}</p>
                )}
                {!isUser && <ProductCards products={m.products} />}
                {!isUser && m.fallbackToHuman && (
                  <button
                    type="button"
                    onClick={onSwitchToHuman}
                    className="mt-2 flex items-center gap-1 text-xs font-medium text-slate-900 underline underline-offset-2"
                  >
                    <UserRound className="h-3 w-3" />
                    Talk to a staff member
                  </button>
                )}
              </div>
            </div>
          );
        })}
        <div ref={endRef} />
      </div>

      {/* First-open only: a few real questions the catalog can answer, so the
          customer does not have to guess what the bot knows. Keyed off the
          number of user turns rather than raw message count: the transcript is
          persisted, and a stray assistant message (a previous greeting, a
          restored session) would otherwise hide the buttons forever. */}
      {messages.every((m) => m.role === "assistant") && !isLoading && (
        <div className="shrink-0 animate-fade-up border-t border-slate-100 bg-white px-2.5 pt-2 motion-reduce:animate-none">
          <p className="mb-1.5 px-0.5 text-[11px] font-medium uppercase tracking-wide text-slate-400">
            Try asking
          </p>
          <div className="flex flex-wrap gap-1.5">
            {QUICK_QUESTIONS.map((q) => (
              <button
                key={q}
                type="button"
                onClick={() => void ask(q)}
                className="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 text-left text-xs text-slate-700 transition-colors hover:border-slate-900 hover:bg-white hover:text-slate-900"
              >
                {q}
              </button>
            ))}
          </div>
        </div>
      )}

      <form
        onSubmit={submit}
        className="flex items-center gap-2 border-t border-slate-100 bg-white p-2.5"
      >
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          maxLength={2000}
          disabled={isLoading}
          placeholder="Ask about a product or policy..."
          aria-label="Message the shopping assistant"
          className="flex-1 rounded-lg border border-slate-200 bg-slate-50 px-3 py-1.5 text-sm outline-none transition-colors placeholder:text-slate-400 focus:border-slate-900 focus:bg-white disabled:opacity-60"
        />
        <button
          type="submit"
          disabled={isLoading || !input.trim()}
          aria-label="Send question"
          className="rounded bg-slate-900 p-2 text-white transition-colors hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-40"
        >
          <ArrowUp className="h-4 w-4" />
        </button>
      </form>
    </>
  );
}

/**
 * Human half: reuses the existing support panel wholesale, so attachments,
 * order/product pickers, reply and recall keep working untouched. The
 * first-time greeting is a local banner because the chat service has no
 * bot sender role and this feature must stay migration-free.
 */
function HumanThread() {
  const { isAuthenticated, conversation, messages } = useChat();
  const { seenConversationId, markSeen } = useHumanGreeting();
  const conversationId = conversation?.uuid ?? "none";

  const showGreeting =
    isAuthenticated &&
    messages.length === 0 &&
    seenConversationId !== conversationId;

  useEffect(() => {
    if (!isAuthenticated || messages.length > 0 || !showGreeting) return;
    // Only remember it once the customer actually had it on screen.
    const timer = setTimeout(() => markSeen(conversationId), 8000);
    return () => clearTimeout(timer);
  }, [
    isAuthenticated,
    messages.length,
    showGreeting,
    conversationId,
    markSeen,
  ]);

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      {showGreeting && (
        <div className="m-3 mb-0 rounded-lg border border-slate-200 bg-white p-3 text-xs leading-relaxed text-slate-600">
          <p className="mb-1 flex items-center gap-1.5 font-semibold text-slate-900">
            <MessageSquare className="h-3.5 w-3.5" />
            Support team
          </p>
          {HUMAN_GREETING}
        </div>
      )}
      <ChatPanel hideHeader />
    </div>
  );
}

/**
 * One launcher, one panel, two modes. The support widget supplies the base
 * layout; the AI tab reuses the same shell so switching does not feel like a
 * different product.
 */
export function ChatWidget() {
  const mounted = useMounted();
  const { isOpen, setIsOpen, mode, setMode } = useChatUIStore();
  const { isAuthenticated, unreadCount, messages, isShopOnline } = useChat();
  const reset = useAssistant((s) => s.reset);
  const [showChooser, setShowChooser] = useState(false);

  const handleToggle = () => {
    const next = !isOpen;
    setIsOpen(next);
    if (!next) return;
    requestNotificationPermission().catch(() => {});
    // First-run only: no tab remembered AND no conversation already on either
    // side. Someone returning to an existing thread goes straight back to it.
    const hasHistory =
      useAssistant.getState().messages.length > 0 || messages.length > 0;
    if (!mode && !hasHistory) setShowChooser(true);
  };

  const chooseMode = (next: ChatMode) => {
    setMode(next);
    setShowChooser(false);
  };

  if (!mounted) return null;

  // Anonymous visitors keep the support tab; the AI tab needs an account.
  const activeMode: ChatMode = mode === "ai" && !isAuthenticated ? "human" : mode ?? "ai";
  const assistantLoading = useAssistant.getState().isLoading;

  return (
    <div className="fixed bottom-3 right-3 z-50 flex flex-col items-end">
      {isOpen && (
        <div className="relative mb-3 flex h-[600px] max-h-[calc(100vh-6rem)] w-[calc(100vw-2rem)] max-w-[560px] origin-bottom-right animate-chat-in flex-col overflow-hidden rounded-lg border border-slate-200 bg-white text-slate-900 shadow-xl motion-reduce:animate-none">
          {/* One dark header for both tabs, so switching never changes the
              frame. Mirrors the support widget's own header. */}
          <div className="flex items-center justify-between bg-slate-900 px-4 py-3 text-white">
            <div className="flex min-w-0 items-center gap-2.5">
              <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-white/10">
                <ModeIcon
                  mode={showChooser ? "human" : activeMode}
                  className="h-4 w-4"
                />
              </div>
              <div className="min-w-0">
                <h3 className="truncate text-sm font-semibold leading-tight">
                  {showChooser
                    ? "Customer Support"
                    : activeMode === "ai"
                      ? "AI Shopping Assistant"
                      : "Customer Support"}
                </h3>
                <p className="truncate text-xs text-slate-400">
                  {showChooser
                    ? "AI assistant and real advisors"
                    : activeMode === "ai"
                      ? "Product advice and store policies"
                      : isShopOnline
                        ? "Active now"
                        : "Replies in a few minutes"}
                </p>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-1">
              {!showChooser && activeMode === "ai" && (
                <button
                  type="button"
                  onClick={reset}
                  disabled={assistantLoading}
                  aria-label="Start a new assistant conversation"
                  title="New chat"
                  className="rounded p-1 text-slate-400 transition-colors hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <RefreshCw className="h-4 w-4" />
                </button>
              )}
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                aria-label="Close chat"
                className="rounded p-1 text-slate-400 transition-colors hover:text-white"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
          </div>

          {/* Below the shared header: the switcher, then the active thread. */}
          {showChooser ? (
            <div className="min-h-0 flex-1 bg-slate-50">
              <ModeChooser onChoose={chooseMode} canUseAi={isAuthenticated} />
            </div>
          ) : !isAuthenticated && activeMode === "human" ? (
            <HumanThread />
          ) : isAuthenticated ? (
            <>
              <ModeSwitcher
                mode={activeMode}
                onChange={chooseMode}
                disabled={activeMode === "ai" && assistantLoading}
              />
              {activeMode === "human" ? (
                <HumanThread />
              ) : (
                <AiThread isAuthenticated onSwitchToHuman={() => chooseMode("human")} />
              )}
            </>
          ) : (
            <AiThread
              isAuthenticated={false}
              onSwitchToHuman={() => chooseMode("human")}
            />
          )}
        </div>
      )}

      <button
        type="button"
        onClick={handleToggle}
        aria-label={isOpen ? "Close chat" : "Open chat"}
        className="relative flex h-12 w-12 items-center justify-center rounded-full bg-slate-900 text-white shadow-lg transition-colors hover:bg-slate-800 focus:outline-none focus-visible:ring-2 focus-visible:ring-slate-900 focus-visible:ring-offset-2"
      >
        {isOpen ? (
          <X className="h-5 w-5" />
        ) : (
          <MessageSquare className="h-5 w-5" />
        )}
        {!isOpen && unreadCount > 0 && (
          <span className="absolute -right-1 -top-1 flex h-5 min-w-[20px] items-center justify-center rounded-full border-2 border-white bg-red-600 px-1 text-[11px] font-bold text-white">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </button>
    </div>
  );
}
