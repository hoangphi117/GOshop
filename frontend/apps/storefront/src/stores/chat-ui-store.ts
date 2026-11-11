import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { OrderRef, ProductRef } from "@/hooks/use-chat";

/** Which side of the unified widget is showing. */
export type ChatMode = "ai" | "human";

interface ChatUIState {
  isOpen: boolean;
  setIsOpen: (isOpen: boolean) => void;
  openChat: () => void;
  closeChat: () => void;
  /** Null until the customer picks a mode on the first-time screen. */
  mode: ChatMode | null;
  setMode: (mode: ChatMode | null) => void;
  /** "Chat with a human" entry points jump straight to the human tab. */
  openChatWithHuman: () => void;
  pendingOrder: OrderRef | null;
  setPendingOrder: (order: OrderRef | null) => void;
  openChatWithOrder: (order: OrderRef) => void;
  pendingProduct: ProductRef | null;
  setPendingProduct: (product: ProductRef | null) => void;
  openChatWithProduct: (product: ProductRef) => void;
}

export const useChatUIStore = create<ChatUIState>()(
  persist(
    (set) => ({
      isOpen: false,
      setIsOpen: (isOpen) => set({ isOpen }),
      openChat: () => set({ isOpen: true }),
      closeChat: () => set({ isOpen: false }),
      mode: null,
      setMode: (mode) => set({ mode }),
      openChatWithHuman: () => set({ isOpen: true, mode: "human" }),
      pendingOrder: null,
      setPendingOrder: (pendingOrder) => set({ pendingOrder }),
      openChatWithOrder: (order) =>
        set({ isOpen: true, mode: "human", pendingOrder: order }),
      pendingProduct: null,
      setPendingProduct: (pendingProduct) => set({ pendingProduct }),
      openChatWithProduct: (product) =>
        set({ isOpen: true, mode: "human", pendingProduct: product }),
    }),
    {
      name: "goshop-chat-ui",
      // Only the chosen tab survives a reload. Persisting isOpen would reopen
      // the panel on every navigation and re-fire the first-time greeting.
      partialize: (s) => ({ mode: s.mode }),
    },
  ),
);
