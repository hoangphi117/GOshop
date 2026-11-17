import { create } from "zustand";
import { persist } from "zustand/middleware";
import { toast } from "sonner";
import {
  streamAssistant,
  createAssistantSession,
  AssistantError,
} from "@/lib/assistant-api";
import type { AssistantProduct } from "@/lib/assistant-api";

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
  open: () => void;
  close: () => void;
  toggle: () => void;
  reset: () => void;
  ask: (question: string) => Promise<void>;
  /** Seeds the opening line once, so the panel is never empty on first open. */
  greet: () => void;
}

// The transcript and session id survive a reload (session TTL is 1h
// server-side); anything the server dropped simply stops being replayed.
const MAX_MESSAGES = 40;

export const ASSISTANT_GREETING =
  "Hi there! I'm Maarch, your AI assistant. How can I help you today?";

function newId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export const useAssistant = create<AssistantState>()(
  persist(
    (set, get) => ({
      isOpen: false,
      sessionId: null,
      messages: [],
      isLoading: false,

      open: () => set({ isOpen: true }),
      close: () => set({ isOpen: false }),
      toggle: () => set((s) => ({ isOpen: !s.isOpen })),
      reset: () =>
        set({
          sessionId: null,
          messages: [
            {
              id: newId(),
              role: "assistant" as const,
              content: ASSISTANT_GREETING,
              products: [],
              fallbackToHuman: false,
            },
          ],
        }),

      // A greeting is a local message, not a turn: the server never sees it, so
      // it costs no tokens and cannot be rate limited.
      greet: () =>
        set((s) =>
          s.messages.length > 0
            ? s
            : {
                messages: [
                  {
                    id: newId(),
                    role: "assistant" as const,
                    content: ASSISTANT_GREETING,
                    products: [],
                    fallbackToHuman: false,
                  },
                ],
              },
        ),

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
    }),
    {
      name: "goshop-assistant",
      partialize: (s) => ({
        sessionId: s.sessionId,
        messages: s.messages.slice(-MAX_MESSAGES),
      }),
    },
  ),
);
