import { create } from "zustand";
import type { OrderRef } from "@/hooks/use-chat";

interface ChatUIState {
  isOpen: boolean;
  setIsOpen: (isOpen: boolean) => void;
  openChat: () => void;
  closeChat: () => void;
  pendingOrder: OrderRef | null;
  setPendingOrder: (order: OrderRef | null) => void;
  openChatWithOrder: (order: OrderRef) => void;
}

export const useChatUIStore = create<ChatUIState>((set) => ({
  isOpen: false,
  setIsOpen: (isOpen) => set({ isOpen }),
  openChat: () => set({ isOpen: true }),
  closeChat: () => set({ isOpen: false }),
  pendingOrder: null,
  setPendingOrder: (pendingOrder) => set({ pendingOrder }),
  openChatWithOrder: (order) => set({ isOpen: true, pendingOrder: order }),
}));
