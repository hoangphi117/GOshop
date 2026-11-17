import { create } from "zustand";
import { persist } from "zustand/middleware";

/**
 * The human side has no server-side "greeting message": the chat service only
 * stores what a real user or admin sends, and inventing a bot row there would
 * need a migration plus a sender_role='bot' the backend does not have.
 *
 * So the opening line lives here, as a local banner rendered above the
 * transcript. It is shown once per browser, then remembered.
 */
interface HumanGreetingState {
  seenConversationId: string | null
  markSeen: (conversationId: string | null) => void
}

export const useHumanGreeting = create<HumanGreetingState>()(
  persist(
    (set) => ({
      seenConversationId: null,
      markSeen: (seenConversationId) => set({ seenConversationId }),
    }),
    { name: "goshop-human-greeting" },
  ),
)

export const HUMAN_GREETING =
  "Hi! You are chatting with our support team. Leave a message and an advisor will reply here as soon as they are free."
