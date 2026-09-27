'use client'

import { useState, useRef, useEffect } from 'react'
import Link from 'next/link'
import { MessageSquare, X, Send, User } from 'lucide-react'
import { useChat } from '@/hooks/use-chat'

export function ChatWidget() {
  const [isOpen, setIsOpen] = useState(false)
  const [inputValue, setInputValue] = useState('')
  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)

  const {
    isAuthenticated,
    conversation,
    messages,
    unreadCount,
    sendMessage,
    isSending,
    markAsRead,
  } = useChat()

  // Auto scroll to bottom when new messages arrive or when widget opens
  useEffect(() => {
    if (isOpen) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'auto' })
      if (unreadCount > 0) {
        markAsRead()
      }
    }
  }, [isOpen, messages, unreadCount, markAsRead])

  // Focus input when chat opens
  useEffect(() => {
    if (isOpen && isAuthenticated) {
      inputRef.current?.focus()
    }
  }, [isOpen, isAuthenticated])

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault()
    const content = inputValue.trim()
    if (!content || isSending) return

    setInputValue('')
    try {
      await sendMessage(content)
    } catch {
      setInputValue(content)
    }
  }

  const formatTime = (isoString: string) => {
    try {
      const date = new Date(isoString)
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    } catch {
      return ''
    }
  }

  return (
    <div className="fixed bottom-6 right-6 z-50">
      {/* Chat Popover Window */}
      {isOpen && (
        <div className="mb-3 flex flex-col w-[360px] sm:w-[390px] h-[520px] bg-white border border-slate-200 rounded-lg shadow-xl overflow-hidden text-slate-900">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 bg-slate-900 text-white">
            <div className="flex items-center gap-2.5">
              <div className="w-2.5 h-2.5 rounded-full bg-emerald-400" />
              <div>
                <h3 className="text-sm font-semibold leading-tight">Customer Support</h3>
                <p className="text-xs text-slate-400">We typically reply in minutes</p>
              </div>
            </div>
            <button
              onClick={() => setIsOpen(false)}
              className="p-1 text-slate-400 hover:text-white rounded transition-colors"
              aria-label="Close chat"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Body */}
          <div className="flex-1 p-4 overflow-y-auto bg-slate-50 space-y-3">
            {!isAuthenticated ? (
              <div className="h-full flex flex-col items-center justify-center text-center p-6 space-y-3">
                <div className="w-12 h-12 rounded-full bg-slate-200 flex items-center justify-center text-slate-600">
                  <User className="w-6 h-6" />
                </div>
                <div className="space-y-1">
                  <h4 className="text-sm font-medium text-slate-800">Sign in to chat</h4>
                  <p className="text-xs text-slate-500">
                    Please log in to contact support and view your order conversations.
                  </p>
                </div>
                <Link
                  href="/login"
                  className="inline-flex items-center justify-center px-4 py-2 text-xs font-medium text-white bg-slate-900 rounded hover:bg-slate-800 transition-colors"
                  onClick={() => setIsOpen(false)}
                >
                  Log In
                </Link>
              </div>
            ) : messages.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-center p-6 space-y-2 text-slate-500">
                <MessageSquare className="w-8 h-8 text-slate-400" />
                <p className="text-sm font-medium text-slate-700">How can we help you today?</p>
                <p className="text-xs">Send a message below and an agent will assist you shortly.</p>
              </div>
            ) : (
              <>
                {messages.map((msg) => {
                  const isUser = msg.sender_role === 'user'
                  return (
                    <div
                      key={msg.uuid}
                      className={`flex flex-col ${isUser ? 'items-end' : 'items-start'}`}
                    >
                      <div
                        className={`max-w-[80%] px-3.5 py-2 rounded-lg text-sm break-words ${
                          isUser
                            ? 'bg-slate-900 text-white rounded-br-none'
                            : 'bg-white text-slate-900 border border-slate-200 rounded-bl-none shadow-sm'
                        }`}
                      >
                        {msg.content}
                      </div>
                      <span className="text-[10px] text-slate-400 mt-1 px-1">
                        {formatTime(msg.created_at)}
                      </span>
                    </div>
                  )
                })}
                <div ref={messagesEndRef} />
              </>
            )}
          </div>

          {/* Footer Input */}
          {isAuthenticated && (
            <form
              onSubmit={handleSend}
              className="p-3 bg-white border-t border-slate-200 flex items-center gap-2"
            >
              <input
                ref={inputRef}
                type="text"
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                placeholder="Type your message..."
                className="flex-1 text-sm px-3 py-2 border border-slate-300 rounded focus:outline-none focus:border-slate-900"
                disabled={isSending}
              />
              <button
                type="submit"
                disabled={!inputValue.trim() || isSending}
                className="p-2 bg-slate-900 text-white rounded hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                aria-label="Send message"
              >
                <Send className="w-4 h-4" />
              </button>
            </form>
          )}
        </div>
      )}

      {/* Floating Toggle Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="relative flex items-center justify-center w-12 h-12 rounded-full bg-slate-900 text-white shadow-lg hover:bg-slate-800 focus:outline-none transition-colors"
        aria-label={isOpen ? 'Close chat' : 'Open chat'}
      >
        {isOpen ? <X className="w-5 h-5" /> : <MessageSquare className="w-5 h-5" />}

        {/* Unread Badge */}
        {!isOpen && unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 flex items-center justify-center min-w-[20px] h-5 px-1 text-[11px] font-bold text-white bg-red-600 rounded-full border-2 border-white">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>
    </div>
  )
}
