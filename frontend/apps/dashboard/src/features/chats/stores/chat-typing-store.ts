import { create } from 'zustand'

interface ChatTypingState {
  typingConversations: Record<string, boolean>
  setTyping: (conversationUUID: string, isTyping: boolean) => void
}

const typingTimers: Record<string, ReturnType<typeof setTimeout>> = {}

export const useChatTypingStore = create<ChatTypingState>((set) => ({
  typingConversations: {},
  setTyping: (conversationUUID, isTyping) => {
    if (typingTimers[conversationUUID]) {
      clearTimeout(typingTimers[conversationUUID])
      delete typingTimers[conversationUUID]
    }

    if (isTyping) {
      // Auto reset typing indicator after 3.5 seconds in case stop event is lost
      typingTimers[conversationUUID] = setTimeout(() => {
        set((state) => ({
          typingConversations: {
            ...state.typingConversations,
            [conversationUUID]: false,
          },
        }))
        delete typingTimers[conversationUUID]
      }, 3500)
    }

    set((state) => ({
      typingConversations: {
        ...state.typingConversations,
        [conversationUUID]: isTyping,
      },
    }))
  },
}))
