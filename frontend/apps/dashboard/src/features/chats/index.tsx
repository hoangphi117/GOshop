import { useState, useRef, useEffect } from 'react'
import { format, isToday } from 'date-fns'
import {
  Search as SearchIcon,
  Send,
  MessagesSquare,
  CheckCheck,
  Check,
  Lock,
} from 'lucide-react'
import { cn, getDisplayNameInitials } from '@/lib/utils'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import {
  useAdminConversations,
  useAdminConversationDetails,
  useAdminMessages,
  useAdminSendMessage,
  useAdminMarkRead,
  useAdminCloseConversation,
} from './hooks/use-admin-chats'
import type { ConversationItem } from './types'

export function Chats() {
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'open' | 'closed'>('all')
  const [selectedConvUUID, setSelectedConvUUID] = useState<string | null>(null)
  const [inputText, setInputText] = useState('')
  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)

  // 1. Fetch conversations list
  const { data: conversations = [], isLoading: isConversationsLoading } =
    useAdminConversations({
      status: statusFilter === 'all' ? undefined : statusFilter,
      search: search.trim() || undefined,
    })

  // 2. Fetch active conversation details
  const { data: activeConversation } = useAdminConversationDetails(
    selectedConvUUID || undefined
  )

  // 3. Fetch active conversation messages
  const { data: messages = [], isLoading: isMessagesLoading } = useAdminMessages(
    selectedConvUUID || undefined
  )

  // 4. Mutations
  const { mutateAsync: sendMessage, isPending: isSending } = useAdminSendMessage()
  const { mutate: markRead } = useAdminMarkRead()
  const { mutate: closeConversation, isPending: isClosing } = useAdminCloseConversation()

  // Select first conversation by default if none selected
  useEffect(() => {
    if (!selectedConvUUID && conversations.length > 0) {
      setSelectedConvUUID(conversations[0].uuid)
    }
  }, [conversations, selectedConvUUID])

  // Mark conversation as read when selected
  useEffect(() => {
    if (selectedConvUUID) {
      markRead(selectedConvUUID)
    }
  }, [selectedConvUUID, markRead])

  // Scroll to bottom when messages update
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'auto' })
  }, [messages])

  // Focus input when conversation changes
  useEffect(() => {
    if (selectedConvUUID && activeConversation?.status !== 'closed') {
      inputRef.current?.focus()
    }
  }, [selectedConvUUID, activeConversation?.status])

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault()
    const content = inputText.trim()
    if (!content || !selectedConvUUID || isSending) return

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

  return (
    <>
      <DashboardHeader title="Customer Support Chat" />
      <Main fixed fluid className="p-0 flex flex-col flex-1 h-[calc(100vh-4rem)]">
        <div className="flex flex-1 overflow-hidden border-t">
          {/* Left Pane: Conversation List */}
          <div className="w-80 sm:w-96 flex flex-col border-r bg-card shrink-0">
            {/* Search and Filters */}
            <div className="p-3 border-b space-y-2">
              <div className="relative">
                <SearchIcon className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search customer name or email..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="pl-8 h-9 text-xs"
                />
              </div>

              {/* Status Filter Tabs */}
              <div className="flex gap-1 p-0.5 bg-muted rounded-md text-xs">
                {(['all', 'open', 'closed'] as const).map((st) => (
                  <button
                    key={st}
                    onClick={() => setStatusFilter(st)}
                    className={cn(
                      'flex-1 py-1 px-2 rounded capitalize font-medium transition-colors text-center',
                      statusFilter === st
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    {st}
                  </button>
                ))}
              </div>
            </div>

            {/* Conversation Items */}
            <ScrollArea className="flex-1">
              {isConversationsLoading ? (
                <div className="p-6 text-center text-xs text-muted-foreground">
                  Loading conversations...
                </div>
              ) : conversations.length === 0 ? (
                <div className="p-8 text-center text-xs text-muted-foreground space-y-1">
                  <MessagesSquare className="w-6 h-6 mx-auto text-muted-foreground/60 mb-2" />
                  <p className="font-medium">No conversations found</p>
                  <p className="text-[11px]">Customer conversations will appear here.</p>
                </div>
              ) : (
                <div className="divide-y divide-border/60">
                  {conversations.map((conv: ConversationItem) => {
                    const isSelected = selectedConvUUID === conv.uuid
                    return (
                      <button
                        key={conv.uuid}
                        onClick={() => setSelectedConvUUID(conv.uuid)}
                        className={cn(
                          'w-full text-left p-3.5 flex items-start gap-3 transition-colors hover:bg-muted/50',
                          isSelected && 'bg-muted border-l-2 border-l-primary'
                        )}
                      >
                        <Avatar className="h-10 w-10 shrink-0 border">
                          <AvatarImage src={conv.user_avatar} alt={conv.user_fullname} />
                          <AvatarFallback className="text-xs font-semibold">
                            {getDisplayNameInitials(conv.user_fullname || conv.user_email)}
                          </AvatarFallback>
                        </Avatar>

                        <div className="flex-1 min-w-0">
                          <div className="flex items-center justify-between gap-1 mb-0.5">
                            <span className="text-xs font-semibold truncate text-foreground">
                              {conv.user_fullname || conv.user_email}
                            </span>
                            <span className="text-[10px] text-muted-foreground shrink-0">
                              {conv.last_message_at
                                ? formatMessageTime(conv.last_message_at)
                                : ''}
                            </span>
                          </div>

                          <p className="text-xs text-muted-foreground truncate mb-1">
                            {conv.last_message_content || 'No messages yet'}
                          </p>

                          <div className="flex items-center justify-between">
                            <Badge
                              variant={conv.status === 'open' ? 'outline' : 'secondary'}
                              className={cn(
                                'text-[10px] px-1.5 py-0 h-4 font-normal uppercase',
                                conv.status === 'open' &&
                                  'border-emerald-500/50 text-emerald-600 dark:text-emerald-400'
                              )}
                            >
                              {conv.status}
                            </Badge>

                            {conv.unread_count > 0 && (
                              <span className="flex items-center justify-center min-w-[18px] h-[18px] px-1 text-[10px] font-bold text-white bg-red-600 rounded-full">
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
          <div className="flex-1 flex flex-col bg-background overflow-hidden">
            {!selectedConvUUID || !activeConversation ? (
              <div className="flex-1 flex flex-col items-center justify-center text-center p-8 text-muted-foreground space-y-2">
                <MessagesSquare className="w-10 h-10 text-muted-foreground/40" />
                <h3 className="text-sm font-semibold text-foreground">
                  Select a conversation
                </h3>
                <p className="text-xs max-w-sm">
                  Choose a customer conversation from the list to view chat history and send
                  replies.
                </p>
              </div>
            ) : (
              <>
                {/* Chat Header */}
                <div className="h-14 px-4 border-b flex items-center justify-between bg-card shrink-0">
                  <div className="flex items-center gap-3">
                    <Avatar className="h-8 w-8 border">
                      <AvatarImage
                        src={activeConversation.user_avatar}
                        alt={activeConversation.user_fullname}
                      />
                      <AvatarFallback className="text-xs">
                        {getDisplayNameInitials(
                          activeConversation.user_fullname || activeConversation.user_email
                        )}
                      </AvatarFallback>
                    </Avatar>
                    <div>
                      <h4 className="text-xs font-semibold text-foreground leading-tight">
                        {activeConversation.user_fullname || activeConversation.user_email}
                      </h4>
                      <p className="text-[11px] text-muted-foreground">
                        {activeConversation.user_email}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    <Badge
                      variant={activeConversation.status === 'open' ? 'outline' : 'secondary'}
                      className={cn(
                        'text-xs capitalize',
                        activeConversation.status === 'open' &&
                          'border-emerald-500 text-emerald-600 dark:text-emerald-400'
                      )}
                    >
                      {activeConversation.status}
                    </Badge>

                    {activeConversation.status === 'open' && (
                      <Button
                        variant="outline"
                        size="sm"
                        className="h-8 text-xs gap-1.5"
                        disabled={isClosing}
                        onClick={() => closeConversation(activeConversation.uuid)}
                      >
                        <Lock className="w-3.5 h-3.5 text-muted-foreground" />
                        Close Chat
                      </Button>
                    )}
                  </div>
                </div>

                {/* Messages Stream */}
                <ScrollArea className="flex-1 p-4 bg-muted/20">
                  {isMessagesLoading ? (
                    <div className="p-8 text-center text-xs text-muted-foreground">
                      Loading messages...
                    </div>
                  ) : messages.length === 0 ? (
                    <div className="p-8 text-center text-xs text-muted-foreground">
                      No messages in this conversation.
                    </div>
                  ) : (
                    <div className="space-y-3 max-w-3xl mx-auto">
                      {messages.map((msg) => {
                        const isAdmin = msg.sender_role === 'admin'
                        return (
                          <div
                            key={msg.uuid}
                            className={cn(
                              'flex flex-col',
                              isAdmin ? 'items-end' : 'items-start'
                            )}
                          >
                            <div
                              className={cn(
                                'max-w-[75%] px-3.5 py-2 rounded-lg text-xs break-words shadow-sm',
                                isAdmin
                                  ? 'bg-primary text-primary-foreground rounded-br-none'
                                  : 'bg-card text-card-foreground border rounded-bl-none'
                              )}
                            >
                              {msg.content}
                            </div>
                            <div className="flex items-center gap-1 text-[10px] text-muted-foreground mt-1 px-1">
                              <span>{formatMessageTime(msg.created_at)}</span>
                              {isAdmin && (
                                msg.is_read ? (
                                  <CheckCheck className="w-3 h-3 text-emerald-500" />
                                ) : (
                                  <Check className="w-3 h-3 text-muted-foreground" />
                                )
                              )}
                            </div>
                          </div>
                        )
                      })}
                      <div ref={messagesEndRef} />
                    </div>
                  )}
                </ScrollArea>

                {/* Message Input Bar */}
                <div className="p-3 border-t bg-card shrink-0">
                  {activeConversation.status === 'closed' ? (
                    <div className="py-2 text-center text-xs text-muted-foreground bg-muted/50 rounded border border-dashed">
                      This conversation has been closed. Re-open by sending a new message or wait for the customer to reach out.
                    </div>
                  ) : (
                    <form onSubmit={handleSend} className="flex items-center gap-2">
                      <Input
                        ref={inputRef}
                        type="text"
                        placeholder="Type your reply to customer..."
                        value={inputText}
                        onChange={(e) => setInputText(e.target.value)}
                        disabled={isSending}
                        className="flex-1 text-xs h-9"
                      />
                      <Button
                        type="submit"
                        size="sm"
                        disabled={!inputText.trim() || isSending}
                        className="h-9 px-3 gap-1.5 text-xs"
                      >
                        <Send className="w-3.5 h-3.5" />
                        Send
                      </Button>
                    </form>
                  )}
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
