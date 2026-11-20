import { create } from "zustand";
import { toast } from "sonner";
import {
  streamAssistant,
  createAssistantSession,
  getAssistantHistory,
  AssistantError,
} from "@/lib/assistant-api";
import type { AssistantHistoryMessage, AssistantProduct } from "@/lib/assistant-api";

export interface AssistantMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  products: AssistantProduct[];
  fallbackToHuman: boolean;
  pending?: boolean;
  error?: boolean;
}

interface AssistantState {
  isOpen: boolean;
  sessionId: string | null;
  messages: AssistantMessage[];
  isLoading: boolean;
  /** True once the server transcript has been read (or found empty). */
  hydrated: boolean;
  hydrating: boolean;
  open: () => void;
  close: () => void;
  toggle: () => void;
  reset: () => void;
  /** Clears the panel and starts a fresh server conversation. */
  startNewChat: () => void;
  ask: (question: string) => Promise<void>;
  /** Seeds the opening line once, so the panel is never empty on first open. */
  greet: () => void;
  /** Reads the signed-in user's transcript from the server. */
  hydrate: () => Promise<void>;
}

// Only a render bound: the server owns the transcript and its retention.
const MAX_MESSAGES = 40;

export const ASSISTANT_GREETING =
  "Hi there! I'm Maarch, your AI assistant. How can I help you today?";

function newId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function toMessages(history: AssistantHistoryMessage[]): AssistantMessage[] {
  return history.map((m) => ({
    id: newId(),
    role: m.role,
    content: m.content,
    products: m.products,
    fallbackToHuman: m.fallbackToHuman,
  }));
}

function greeting(): AssistantMessage {
  return {
    id: newId(),
    role: "assistant" as const,
    content: ASSISTANT_GREETING,
    products: [],
    fallbackToHuman: false,
  };
}

// Bumped by reset(). A history request started under one account must not land
// in the store after the next account signs in, which is the leak the old
// localStorage shape caused on a shared browser.
let hydrationEpoch = 0;

export const useAssistant = create<AssistantState>()((set, get) => ({
  isOpen: false,
  sessionId: null,
  messages: [],
  isLoading: false,
  hydrated: false,
  hydrating: false,

  open: () => set({ isOpen: true }),
  close: () => set({ isOpen: false }),
  toggle: () => set((s) => ({ isOpen: !s.isOpen })),

  // Wipes the in-memory transcript on sign-out or an account switch. The
  // greeting is not re-seeded here: hydrate() decides, because a returning user
  // should get their own messages instead of a canned line.
  reset: () => {
    hydrationEpoch += 1;
    set({
      sessionId: null,
      messages: [],
      isLoading: false,
      hydrated: false,
      hydrating: false,
    });
  },

  // "New chat": drop the panel and let the next question mint a new session id,
  // which the server stores as a new conversation. Without the epoch bump an
  // in-flight history read would redraw the conversation the customer just left.
  startNewChat: () => {
    hydrationEpoch += 1;
    set({
      sessionId: null,
      messages: [],
      isLoading: false,
      hydrated: true,
      hydrating: false,
    });
    get().greet();
  },

  // A greeting is a local message, not a turn: the server never sees it, so it
  // costs no tokens and cannot be rate limited.
  greet: () => set((s) => (s.messages.length > 0 ? s : { messages: [greeting()] })),

  hydrate: async () => {
    const { hydrated, hydrating } = get();
    if (hydrated || hydrating) return;

    const epoch = hydrationEpoch;
    set({ hydrating: true });

    // Whatever happens, the panel must stay usable: a failed history read falls
    // back to the greeting rather than an error wall.
    let messages: AssistantMessage[] = [];
    let sessionId: string | null = null;
    try {
      const history = await getAssistantHistory();
      messages = toMessages(history.messages);
      sessionId = history.sessionId;
    } catch {
      messages = [];
    }

    if (epoch !== hydrationEpoch) return; // account changed mid-flight

    set({
      sessionId,
      messages: messages.length > 0 ? messages : get().messages,
      hydrating: false,
      hydrated: true,
    });
    get().greet();
  },

  ask: async (question) => {
    const text = question.trim();
    if (!text || get().isLoading) return;

    const userMessage: AssistantMessage = {
      id: newId(),
      role: "user",
      content: text.slice(0, 2000),
      products: [],
      fallbackToHuman: false,
    };
    const pendingId = newId();
    const pendingMessage: AssistantMessage = {
      id: pendingId,
      role: "assistant",
      content: "",
      products: [],
      fallbackToHuman: false,
      pending: true,
    };

    set((s) => ({
      messages: [...s.messages, userMessage, pendingMessage].slice(
        -MAX_MESSAGES,
      ),
      isLoading: true,
    }));

    // Text arrives in pieces; re-rendering the whole store per token would
    // thrash. Coalesce into at most one render per animation frame.
    let shown = "";
    let frame: number | null = null;
    const paint = () => {
      frame = null;
      const content = shown;
      set((s) => ({
        messages: s.messages.map((m) =>
          m.id === pendingId ? { ...m, content, pending: false } : m,
        ),
      }));
    };
    const onDelta = (delta: string) => {
      shown += delta;
      if (typeof requestAnimationFrame === "function") {
        if (frame === null) frame = requestAnimationFrame(paint);
      } else {
        paint();
      }
    };

    try {
      let sessionId = get().sessionId;
      if (!sessionId) {
        sessionId = await createAssistantSession();
        set({ sessionId });
      }

      const reply = await streamAssistant(
        { message: userMessage.content, sessionId },
        onDelta,
      );

      if (frame !== null && typeof cancelAnimationFrame === "function") {
        cancelAnimationFrame(frame);
        frame = null;
      }
      set((s) => ({
        sessionId: reply.sessionId,
        isLoading: false,
        messages: s.messages.map((m) =>
          m.id === pendingId
            ? {
                ...m,
                content: reply.answer,
                products: reply.products ?? [],
                fallbackToHuman: reply.fallbackToHuman,
                pending: false,
              }
            : m,
        ),
      }));
    } catch (err) {
      if (frame !== null && typeof cancelAnimationFrame === "function") {
        cancelAnimationFrame(frame);
      }
      const code = err instanceof AssistantError ? err.code : "UNKNOWN";
      // A truncated stream still showed most of an answer: keep it and just
      // offer the human instead of replacing good text with an error.
      if (code === "STREAM_TRUNCATED" && shown) {
        set((s) => ({
          isLoading: false,
          messages: s.messages.map((m) =>
            m.id === pendingId
              ? {
                  ...m,
                  content: shown,
                  pending: false,
                  fallbackToHuman: true,
                }
              : m,
          ),
        }));
        return;
      }
      const message =
        code === "TOO_MANY_REQUESTS"
          ? "You have reached today’s limit. Please try again tomorrow or contact support."
          : err instanceof Error
            ? err.message
            : "Something went wrong. Please try again.";

      set((s) => ({
        isLoading: false,
        messages: s.messages.map((m) =>
          m.id === pendingId
            ? {
                ...m,
                content: message,
                pending: false,
                error: true,
                fallbackToHuman: true,
              }
            : m,
        ),
      }));
      toast.error(message);
    }
  },
}));