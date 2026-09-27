import { useState, useRef, useEffect } from 'react'
import { format, isToday } from 'date-fns'
import {
  Search as SearchIcon,
  Send,
  MessagesSquare,
  Check,
  CheckCheck,
} from 'lucide-react'
import { cn, getDisplayNameInitials } from '@/lib/utils'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import {
  useAdminConversations,
  useAdminConversationDetails,
  useAdminMessages,
  useAdminSendMessage,
  useAdminMarkRead,
  useAdminSendTyping,
} from './hooks/use-admin-chats'
import { useChatTypingStore } from './stores/chat-typing-store'
import type { ConversationItem } from './types'

export function Chats() {
  const [search, setSearch] = useState('')
  const [selectedConvUUID, setSelectedConvUUID] = useState<string | null>(null)
  const [inputText, setInputText] = useState('')
  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const scrollContainerRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const typingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isAtBottomRef = useRef(true)
  const prevMessagesLengthRef = useRef(0)
  const prevConvUUIDRef = useRef<string | null>(null)

  // 1. Fetch conversations list
  const { data: conversations = [], isLoading: isConversationsLoading } =
    useAdminConversations({
      search: search.trim() || undefined,
    })

  // 2. Fetch active conversation details
  const { data: activeConversation } = useAdminConversationDetails(
    selectedConvUUID || undefined
  )

  // 3. Fetch active conversation messages
  const { data: messages = [], isLoading: isMessagesLoading } =
    useAdminMessages(selectedConvUUID || undefined)

  // 4. Mutations & Typing state
  const { mutateAsync: sendMessage, isPending: isSending } =
    useAdminSendMessage()
  const { mutate: markRead } = useAdminMarkRead()
  const { mutate: sendAdminTyping } = useAdminSendTyping()

  const typingConversations = useChatTypingStore((s) => s.typingConversations)
  const isCustomerTyping = selectedConvUUID
    ? Boolean(typingConversations[selectedConvUUID])
    : false

  // Track user scroll position in message list
  const handleMessagesScroll = () => {
    const el = scrollContainerRef.current
    if (!el) return
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    isAtBottomRef.current = distanceFromBottom < 80
  }

  // Select first conversation by default if none selected
  useEffect(() => {
    if (!selectedConvUUID && conversations.length > 0) {
      setSelectedConvUUID(conversations[0].uuid)
    }
  }, [conversations, selectedConvUUID])

  // Mark conversation as read when selected or when new unread messages arrive while active
  useEffect(() => {
    if (!selectedConvUUID) return
    const hasUnread = messages.some(
      (m) => m.sender_role === 'user' && !m.is_read
    )
    if (hasUnread && document.visibilityState === 'visible') {
      markRead(selectedConvUUID)
    }
  }, [selectedConvUUID, messages, markRead])

  // Mark as read on window focus/tab visibility change
  useEffect(() => {
    const handleFocus = () => {
      if (selectedConvUUID && document.visibilityState === 'visible') {
        const hasUnread = messages.some(
          (m) => m.sender_role === 'user' && !m.is_read
        )
        if (hasUnread) {
          markRead(selectedConvUUID)
        }
      }
    }

    window.addEventListener('focus', handleFocus)
    document.addEventListener('visibilitychange', handleFocus)
    return () => {
      window.removeEventListener('focus', handleFocus)
      document.removeEventListener('visibilitychange', handleFocus)
    }
  }, [selectedConvUUID, messages, markRead])

  // Smart auto-scroll: only scroll down when switching conversations or when user is already at the bottom
  useEffect(() => {
    const isConvChange = prevConvUUIDRef.current !== selectedConvUUID
    const isNewMessage = messages.length > prevMessagesLengthRef.current

    prevConvUUIDRef.current = selectedConvUUID
    prevMessagesLengthRef.current = messages.length

    if (isConvChange) {
      isAtBottomRef.current = true
      // Small timeout to allow DOM layout to finish calculation
      setTimeout(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'auto' })
      }, 50)
    } else if (isNewMessage && isAtBottomRef.current) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages, selectedConvUUID])

  // Scroll to bottom when customer starts typing
  useEffect(() => {
    if (isCustomerTyping && isAtBottomRef.current) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [isCustomerTyping])

  // Focus input when conversation changes
  useEffect(() => {
    if (selectedConvUUID) {
      inputRef.current?.focus()
    }
  }, [selectedConvUUID])

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setInputText(val)

    if (!selectedConvUUID) return

    if (val.trim()) {
      sendAdminTyping({ conversationUUID: selectedConvUUID, isTyping: true })
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current)
      }
      typingTimeoutRef.current = setTimeout(() => {
        sendAdminTyping({ conversationUUID: selectedConvUUID, isTyping: false })
      }, 2000)
    } else {
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current)
      }
      sendAdminTyping({ conversationUUID: selectedConvUUID, isTyping: false })
    }
  }

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault()
    const content = inputText.trim()
    if (!content || !selectedConvUUID || isSending) return

    if (typingTimeoutRef.current) {
      clearTimeout(typingTimeoutRef.current)
    }
    sendAdminTyping({ conversationUUID: selectedConvUUID, isTyping: false })

    setInputText('')
    try {
      await sendMessage({
        conversationUUID: selectedConvUUID,
        content,
      })
    } catch {
      setInputText(content)
    }
  }

  const formatMessageTime = (dateStr: string) => {
    try {
      const d = new Date(dateStr)
      if (isToday(d)) {
        return format(d, 'HH:mm')
      }
      return format(d, 'MMM d, HH:mm')
    } catch {
      return ''
    }
  }

  const formatDividerTime = (dateStr: string) => {
    try {
      const d = new Date(dateStr)
      const now = new Date()
      const isThisYear = d.getFullYear() === now.getFullYear()
      const isSameDay =
        d.getDate() === now.getDate() &&
        d.getMonth() === now.getMonth() &&
        isThisYear

      const time = format(d, 'HH:mm')
      const dayMonth = format(d, 'dd/MM')
      const year = format(d, 'yy')

      if (isSameDay) {
        return time
      }
      if (isThisYear) {
        return `${time} ${dayMonth}`
      }
      return `${time} ${dayMonth}/${year}`
    } catch {
      return ''
    }
  }

  const shouldShowTimeDivider = (
    currentMsgCreatedAt: string,
    prevMsgCreatedAt?: string
  ) => {
    if (!prevMsgCreatedAt) return true

    try {
      const curr = new Date(currentMsgCreatedAt).getTime()
      const prev = new Date(prevMsgCreatedAt).getTime()
      const diffInHours = (curr - prev) / (1000 * 60 * 60)
      return diffInHours >= 1
    } catch {
      return false
    }
  }

  return (
    <>
      <DashboardHeader />
      <Main
        fixed
        fluid
        className='flex h-[calc(100svh-4rem)] flex-1 flex-col overflow-hidden !p-0'
      >
        <div className='flex h-full w-full flex-1 overflow-hidden border-t'>
          {/* Left Pane: Conversation List */}
          <div className='flex w-80 shrink-0 flex-col border-r bg-card sm:w-96'>
            {/* Search */}
            <div className='border-b p-3'>
              <div className='relative'>
                <SearchIcon className='absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground' />
                <Input
                  placeholder='Search customer name or email...'
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className='h-9 pl-8 text-xs'
                />
              </div>
            </div>

            {/* Conversation Items */}
            <ScrollArea className='flex-1'>
              {isConversationsLoading ? (
                <div className='p-6 text-center text-xs text-muted-foreground'>
                  Loading conversations...
                </div>
              ) : conversations.length === 0 ? (
                <div className='space-y-1 p-8 text-center text-xs text-muted-foreground'>
                  <MessagesSquare className='mx-auto mb-2 h-6 w-6 text-muted-foreground/60' />
                  <p className='font-medium'>No conversations found</p>
                  <p className='text-[11px]'>
                    Customer conversations will appear here.
                  </p>
                </div>
              ) : (
                <div className='divide-y divide-border/60'>
                  {conversations.map((conv: ConversationItem) => {
                    const isSelected = selectedConvUUID === conv.uuid
                    return (
                      <button
                        key={conv.uuid}
                        onClick={() => setSelectedConvUUID(conv.uuid)}
                        className={cn(
                          'flex w-full items-start gap-3 p-3.5 text-left transition-colors hover:bg-muted/50',
                          isSelected && 'border-l-2 border-l-primary bg-muted'
                        )}
                      >
                        <Avatar className='h-10 w-10 shrink-0 border'>
                          <AvatarImage
                            src={conv.user_avatar}
                            alt={conv.user_fullname}
                          />
                          <AvatarFallback className='text-xs font-semibold'>
                            {getDisplayNameInitials(
                              conv.user_fullname || conv.user_email
                            )}
                          </AvatarFallback>
                        </Avatar>

                        <div className='min-w-0 flex-1'>
                          <div className='mb-0.5 flex items-center justify-between gap-1'>
                            <span className='truncate text-xs font-semibold text-foreground'>
                              {conv.user_fullname || conv.user_email}
                            </span>
                            <span className='shrink-0 text-[10px] text-muted-foreground'>
                              {conv.last_message_at
                                ? formatMessageTime(conv.last_message_at)
                                : ''}
                            </span>
                          </div>

                          <div className='flex items-center justify-between gap-2'>
                            <p className='truncate text-xs text-muted-foreground'>
                              {typingConversations[conv.uuid] ? (
                                <span className='font-medium italic text-emerald-600 dark:text-emerald-400'>
                                  Customer is typing...
                                </span>
                              ) : (
                                conv.last_message_content || 'No messages yet'
                              )}
                            </p>
                            {conv.unread_count > 0 && (
                              <span className='flex h-[18px] min-w-[18px] shrink-0 items-center justify-center rounded-full bg-red-600 px-1 text-[10px] font-bold text-white'>
                                {conv.unread_count}
                              </span>
                            )}
                          </div>
                        </div>
                      </button>
                    )
                  })}
                </div>
              )}
            </ScrollArea>
          </div>

          {/* Right Pane: Active Conversation Room */}
          <div className='flex flex-1 flex-col overflow-hidden bg-background'>
            {!selectedConvUUID || !activeConversation ? (
              <div className='flex flex-1 flex-col items-center justify-center space-y-2 p-8 text-center text-muted-foreground'>
                <MessagesSquare className='h-10 w-10 text-muted-foreground/40' />
                <h3 className='text-sm font-semibold text-foreground'>
                  Select a conversation
                </h3>
                <p className='max-w-sm text-xs'>
                  Choose a customer conversation from the list to view chat
                  history and send replies.
                </p>
              </div>
            ) : (
              <>
                {/* Chat Header */}
                <div className='flex h-14 shrink-0 items-center justify-between border-b bg-card px-4'>
                  <div className='flex items-center gap-3'>
                    <Avatar className='h-8 w-8 border'>
                      <AvatarImage
                        src={activeConversation.user_avatar}
                        alt={activeConversation.user_fullname}
                      />
                      <AvatarFallback className='text-xs'>
                        {getDisplayNameInitials(
                          activeConversation.user_fullname ||
                            activeConversation.user_email
                        )}
                      </AvatarFallback>
                    </Avatar>
                    <div>
                      <h4 className='text-xs leading-tight font-semibold text-foreground'>
                        {activeConversation.user_fullname ||
                          activeConversation.user_email}
                      </h4>
                      <p className='text-[11px] text-muted-foreground'>
                        {activeConversation.user_email}
                      </p>
                    </div>
                  </div>
                </div>

                {/* Messages Stream */}
                <div
                  ref={scrollContainerRef}
                  onScroll={handleMessagesScroll}
                  className='flex-1 min-h-0 overflow-y-auto bg-muted/20 p-4'
                >
                  {isMessagesLoading ? (
                    <div className='p-8 text-center text-xs text-muted-foreground'>
                      Loading messages...
                    </div>
                  ) : messages.length === 0 ? (
                    <div className='p-8 text-center text-xs text-muted-foreground'>
                      No messages in this conversation.
                    </div>
                  ) : (
                    <div className='w-full space-y-3'>
                      {messages.map((msg, index) => {
                        const prevMsg = index > 0 ? messages[index - 1] : undefined
                        const showDivider = shouldShowTimeDivider(
                          msg.created_at,
                          prevMsg?.created_at
                        )
                        const isAdmin = msg.sender_role === 'admin'
                        return (
                          <div key={msg.uuid} className='flex flex-col'>
                            {showDivider && (
                              <div className='flex items-center justify-center my-2'>
                                <span className='rounded-full bg-muted px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground border shadow-xs'>
                                  {formatDividerTime(msg.created_at)}
                                </span>
                              </div>
                            )}
                            <div
                              className={cn(
                                'flex flex-col',
                                isAdmin ? 'items-end' : 'items-start'
                              )}
                            >
                              <div
                                className={cn(
                                  'max-w-[75%] rounded-lg px-3.5 py-2 text-xs break-words shadow-sm',
                                  isAdmin
                                    ? 'rounded-br-none bg-primary text-primary-foreground'
                                    : 'rounded-bl-none border bg-card text-card-foreground'
                                )}
                              >
                                {msg.content}
                              </div>
                              <div className='mt-1 flex items-center gap-1 px-1 text-[10px] text-muted-foreground'>
                                <span>{formatMessageTime(msg.created_at)}</span>
                                {isAdmin &&
                                  (msg.is_read ? (
                                    <CheckCheck className='h-3 w-3 text-emerald-600' />
                                  ) : (
                                    <Check className='h-3 w-3 text-muted-foreground' />
                                  ))}
                              </div>
                            </div>
                          </div>
                        )
                      })}
                      {isCustomerTyping && (
                        <div className='flex flex-col items-start'>
                          <div className='flex items-center gap-1.5 rounded-lg rounded-bl-none border bg-card px-3.5 py-2.5 shadow-sm'>
                            <span className='h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/60 [animation-delay:-0.3s]' />
                            <span className='h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/60 [animation-delay:-0.15s]' />
                            <span className='h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/60' />
                          </div>
                          <span className='mt-0.5 px-1 text-[10px] text-muted-foreground'>
                            Customer is typing...
                          </span>
                        </div>
                      )}
                      <div ref={messagesEndRef} />
                    </div>
                  )}
                </div>

                {/* Message Input Bar */}
                <div className='shrink-0 border-t bg-card p-3'>
                  <form
                    onSubmit={handleSend}
                    className='flex items-center gap-2'
                  >
                    <Input
                      ref={inputRef}
                      type='text'
                      placeholder='Type your reply to customer...'
                      value={inputText}
                      onChange={handleInputChange}
                      disabled={isSending}
                      className='h-9 flex-1 text-xs'
                    />
                    <Button
                      type='submit'
                      size='sm'
                      disabled={!inputText.trim() || isSending}
                      className='h-9 gap-1.5 px-3 text-xs'
                    >
                      <Send className='h-3.5 w-3.5' />
                      Send
                    </Button>
                  </form>
                </div>
              </>
            )}
          </div>
        </div>
      </Main>
    </>
  )
}
export default Chats
