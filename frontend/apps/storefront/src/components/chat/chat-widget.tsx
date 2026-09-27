"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { MessageSquare, X, Send, User, Check, CheckCheck } from "lucide-react";
import { useChat } from "@/hooks/use-chat";

export function ChatWidget() {
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const typingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const {
    isAuthenticated,
    conversation,
    messages,
    unreadCount,
    isAdminTyping,
    sendTyping,
    sendMessage,
    isSending,
    markAsRead,
  } = useChat();

  // Auto scroll to bottom when new messages arrive or when widget opens
  useEffect(() => {
    if (isOpen) {
      messagesEndRef.current?.scrollIntoView({ behavior: "auto" });
      if (unreadCount > 0) {
        markAsRead();
      }
    }
  }, [isOpen, messages, unreadCount, markAsRead]);

  // Scroll to bottom when agent starts typing
  useEffect(() => {
    if (isOpen && isAdminTyping) {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [isOpen, isAdminTyping]);

  // Focus input when chat opens
  useEffect(() => {
    if (isOpen && isAuthenticated) {
      inputRef.current?.focus();
    }
  }, [isOpen, isAuthenticated]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setInputValue(val);

    if (val.trim()) {
      sendTyping(true);
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current);
      }
      typingTimeoutRef.current = setTimeout(() => {
        sendTyping(false);
      }, 2000);
    } else {
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current);
      }
      sendTyping(false);
    }
  };

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    const content = inputValue.trim();
    if (!content || isSending) return;

    if (typingTimeoutRef.current) {
      clearTimeout(typingTimeoutRef.current);
    }
    sendTyping(false);

    setInputValue("");
    try {
      await sendMessage(content);
    } catch {
      setInputValue(content);
    }
  };

  const formatTime = (isoString: string) => {
    try {
      const date = new Date(isoString);
      return date.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return "";
    }
  };

  const formatDividerTime = (isoString: string) => {
    try {
      const d = new Date(isoString);
      const now = new Date();
      const isThisYear = d.getFullYear() === now.getFullYear();
      const isSameDay =
        d.getDate() === now.getDate() &&
        d.getMonth() === now.getMonth() &&
        isThisYear;

      const hours = String(d.getHours()).padStart(2, "0");
      const minutes = String(d.getMinutes()).padStart(2, "0");
      const day = String(d.getDate()).padStart(2, "0");
      const month = String(d.getMonth() + 1).padStart(2, "0");
      const year = String(d.getFullYear()).slice(-2);

      const time = `${hours}:${minutes}`;
      const dayMonth = `${day}/${month}`;

      if (isSameDay) {
        return time;
      }
      if (isThisYear) {
        return `${time} ${dayMonth}`;
      }
      return `${time} ${dayMonth}/${year}`;
    } catch {
      return "";
    }
  };

  const shouldShowTimeDivider = (
    currentMsgCreatedAt: string,
    prevMsgCreatedAt?: string
  ) => {
    if (!prevMsgCreatedAt) return true;
    try {
      const curr = new Date(currentMsgCreatedAt).getTime();
      const prev = new Date(prevMsgCreatedAt).getTime();
      const diffInHours = (curr - prev) / (1000 * 60 * 60);
      return diffInHours >= 1;
    } catch {
      return false;
    }
  };

  return (
    <div className="fixed bottom-6 right-6 z-50 flex flex-col items-end">
      {/* Chat Popover Window */}
      {isOpen && (
        <div className="mb-3 flex flex-col w-[360px] sm:w-[390px] h-[520px] bg-white border border-slate-200 rounded-lg shadow-xl overflow-hidden text-slate-900">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 bg-slate-900 text-white">
            <div className="flex items-center gap-2.5">
              <div className="w-2.5 h-2.5 rounded-full bg-emerald-400" />
              <div>
                <h3 className="text-sm font-semibold leading-tight">
                  Customer Support
                </h3>
                <p className="text-xs text-slate-400">
                  We typically reply in minutes
                </p>
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
                  <h4 className="text-sm font-medium text-slate-800">
                    Sign in to chat
                  </h4>
                  <p className="text-xs text-slate-500">
                    Please log in to contact support and view your order
                    conversations.
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
                <p className="text-sm font-medium text-slate-700">
                  How can we help you today?
                </p>
                <p className="text-xs">
                  Send a message below and an agent will assist you shortly.
                </p>
              </div>
            ) : (
              <>
                {messages.map((msg, index) => {
                  const prevMsg = index > 0 ? messages[index - 1] : undefined;
                  const showDivider = shouldShowTimeDivider(
                    msg.created_at,
                    prevMsg?.created_at
                  );
                  const isUser = msg.sender_role === "user";
                  return (
                    <div key={msg.uuid} className="flex flex-col">
                      {showDivider && (
                        <div className="flex items-center justify-center my-2">
                          <span className="rounded-full bg-slate-200/80 border border-slate-300/60 px-2.5 py-0.5 text-[11px] font-medium text-slate-600 shadow-xs">
                            {formatDividerTime(msg.created_at)}
                          </span>
                        </div>
                      )}
                      <div
                        className={`flex flex-col ${isUser ? "items-end" : "items-start"}`}
                      >
                        <div
                          className={`max-w-[80%] px-3.5 py-2 rounded-lg text-sm break-words ${
                            isUser
                              ? "bg-slate-900 text-white rounded-br-none"
                              : "bg-white text-slate-900 border border-slate-200 rounded-bl-none shadow-sm"
                          }`}
                        >
                          {msg.content}
                        </div>
                        <div className="flex items-center gap-1 text-[10px] text-slate-400 mt-1 px-1">
                          <span>{formatTime(msg.created_at)}</span>
                          {isUser &&
                            (msg.is_read ? (
                              <CheckCheck className="h-3 w-3 text-emerald-600" />
                            ) : (
                              <Check className="w-3 h-3 text-slate-400" />
                            ))}
                        </div>
                      </div>
                    </div>
                  );
                })}
                {isAdminTyping && (
                  <div className="flex flex-col items-start">
                    <div className="flex items-center gap-1.5 px-3.5 py-2.5 bg-white border border-slate-200 rounded-lg rounded-bl-none shadow-sm">
                      <span className="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce [animation-delay:-0.3s]" />
                      <span className="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce [animation-delay:-0.15s]" />
                      <span className="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce" />
                    </div>
                    <span className="text-[10px] text-slate-400 mt-0.5 px-1">Support is typing...</span>
                  </div>
                )}
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
                onChange={handleInputChange}
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
        aria-label={isOpen ? "Close chat" : "Open chat"}
      >
        {isOpen ? (
          <X className="w-5 h-5" />
        ) : (
          <MessageSquare className="w-5 h-5" />
        )}

        {/* Unread Badge */}
        {!isOpen && unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 flex items-center justify-center min-w-[20px] h-5 px-1 text-[11px] font-bold text-white bg-red-600 rounded-full border-2 border-white">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </button>
    </div>
  );
}
