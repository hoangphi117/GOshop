import { useState, useRef, useEffect, useCallback } from 'react'
import { format, isToday } from 'date-fns'
import { Link, getRouteApi } from '@tanstack/react-router'
import {
  Search as SearchIcon,
  Send,
  MessagesSquare,
  MessageSquarePlus,
  Loader2,
  Check,
  CheckCheck,
  Paperclip,
  X,
  ShoppingBag,
  Package,
  ChevronLeft,
  ChevronRight,
  Download,
  ZoomIn,
  Tag,
  Archive,
  ArchiveRestore,
  Undo2,
  MoreHorizontal,
  Reply,
  Copy,
  Bell,
  BellOff,
} from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { cn, getDisplayNameInitials } from '@/lib/utils'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import { useAdminUsers } from '@/features/users/hooks/use-users'
import {
  useAdminConversations,
  useAdminConversationDetails,
  useAdminMessages,
  useAdminSendMessage,
  useAdminMarkRead,
  useAdminCloseConversation,
  useAdminReopenConversation,
  useAdminRecallMessage,
  useAdminChatRoom,
  useAdminGetOrCreateConversation,
  uploadChatFile,
} from './hooks/use-admin-chats'
import {
  playNotificationSound,
  requestNotificationPermission,
} from './lib/chat-notifications'
import { useChatTypingStore } from './stores/chat-typing-store'
import type { ConversationItem } from './types'

function formatLastMessage(content?: string) {
  if (!content) return 'No messages yet'
  const trimmed = content.trim()
  if (
    trimmed === '[Images]' ||
    /^\d+\s+(attachments|photos|files)$/i.test(trimmed)
  ) {
    return 'Sent attachments'
  }
  if (
    trimmed === '[Image]' ||
    /\.(jpg|jpeg|png|webp|gif|svg)$/i.test(trimmed) ||
    (trimmed.includes('/uploads/chat/') && !trimmed.includes('.mp4'))
  ) {
    return 'Sent an image'
  }
  if (trimmed === '[Video]' || /\.(mp4|webm|mov|avi)$/i.test(trimmed)) {
    return 'Sent a video'
  }
  if (
    trimmed === '[Order]' ||
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      trimmed
    )
  ) {
    return 'Sent an order'
  }
  if (trimmed === '[Product]') {
    return 'Sent a product'
  }
  if (trimmed.startsWith('> ')) {
    const parts = trimmed.split('\n\n')
    return parts.length > 1 ? parts.slice(1).join(' ') : trimmed
  }
  return trimmed
}

function parseReplyContent(content: string): {
  replyQuote: string | null
  body: string
} {
  if (content && content.startsWith('> ')) {
    const doubleNewlineIdx = content.indexOf('\n\n')
    if (doubleNewlineIdx !== -1) {
      return {
        replyQuote: content.slice(2, doubleNewlineIdx).trim(),
        body: content.slice(doubleNewlineIdx + 2).trim(),
      }
    }
    const singleNewlineIdx = content.indexOf('\n')
    if (singleNewlineIdx !== -1) {
      return {
        replyQuote: content.slice(2, singleNewlineIdx).trim(),
        body: content.slice(singleNewlineIdx + 1).trim(),
      }
    }
  }
  return { replyQuote: null, body: content }
}

function resolveMediaUrl(url?: string) {
  if (!url) return ''
  if (
    url.startsWith('http://') ||
    url.startsWith('https://') ||
    url.startsWith('blob:') ||
    url.startsWith('data:')
  ) {
    return url
  }
  const baseUrl = import.meta.env.VITE_API_BASE_URL || ''
  return `${baseUrl.replace(/\/$/, '')}${url.startsWith('/') ? '' : '/'}${url}`
}

interface MediaItem {
  url: string
  name?: string
  type?: string
}

function parseMediaItems(
  contentType: string,
  attachmentUrl?: string,
  attachmentName?: string
): MediaItem[] {
  if (!attachmentUrl) return []
  if (contentType === 'media_group') {
    try {
      const parsed = JSON.parse(attachmentUrl)
      if (Array.isArray(parsed)) {
        return parsed.map((it) => ({
          url: resolveMediaUrl(it.url),
          name: it.name,
          type: it.type || 'image',
        }))
      }
    } catch {
      // fallback
    }
  }
  return [
    {
      url: resolveMediaUrl(attachmentUrl),
      name: attachmentName,
      type: contentType === 'video' ? 'video' : 'image',
    },
  ]
}

/** Messenger-style cluster grid for 1, 2, 3, 4, 5+ media items */
function MediaGroupGrid({
  items,
  onImageClick,
}: {
  items: MediaItem[]
  onImageClick?: (items: MediaItem[], index: number) => void
}) {
  if (items.length === 0) return null

  if (items.length === 1) {
    const it = items[0]
    if (it.type === 'video') {
      return (
        <video
          src={it.url}
          controls
          className='max-h-56 max-w-full rounded-xl shadow-xs'
        />
      )
    }
    return (
      <button
        type='button'
        onClick={() => onImageClick?.(items, 0)}
        className='group relative block cursor-zoom-in overflow-hidden rounded-xl text-left focus:outline-hidden'
      >
        <img
          src={it.url}
          alt={it.name || 'Shared image'}
          className='max-h-56 max-w-full rounded-xl object-cover shadow-xs transition-transform duration-200 group-hover:scale-[1.02]'
        />
        <div className='absolute inset-0 flex items-center justify-center bg-black/0 transition-colors group-hover:bg-black/20'>
          <div className='rounded-full bg-black/60 p-1.5 text-white opacity-0 shadow-md backdrop-blur-xs transition-opacity group-hover:opacity-100'>
            <ZoomIn className='h-4 w-4' />
          </div>
        </div>
      </button>
    )
  }

  const displayItems = items.slice(0, 4)
  const remainingCount = items.length - 4

  if (items.length === 2) {
    return (
      <div className='grid w-[240px] grid-cols-2 gap-1 overflow-hidden rounded-xl shadow-2xs sm:w-[270px]'>
        {displayItems.map((it, idx) => (
          <button
            key={idx}
            type='button'
            onClick={() => onImageClick?.(items, idx)}
            className='group relative aspect-square cursor-zoom-in overflow-hidden bg-black/5'
          >
            {it.type === 'video' ? (
              <video src={it.url} className='h-full w-full object-cover' />
            ) : (
              <img
                src={it.url}
                alt={it.name || 'Attachment'}
                className='h-full w-full object-cover transition-transform duration-200 group-hover:scale-105'
              />
            )}
          </button>
        ))}
      </div>
    )
  }

  if (items.length === 3) {
    return (
      <div className='grid w-[240px] grid-cols-2 gap-1 overflow-hidden rounded-xl shadow-2xs sm:w-[270px]'>
        <button
          type='button'
          onClick={() => onImageClick?.(items, 0)}
          className='group relative col-span-2 h-28 cursor-zoom-in overflow-hidden bg-black/5'
        >
          {items[0].type === 'video' ? (
            <video src={items[0].url} className='h-full w-full object-cover' />
          ) : (
            <img
              src={items[0].url}
              alt={items[0].name || 'Attachment'}
              className='h-full w-full object-cover transition-transform duration-200 group-hover:scale-105'
            />
          )}
        </button>
        {items.slice(1).map((it, idx) => (
          <button
            key={idx + 1}
            type='button'
            onClick={() => onImageClick?.(items, idx + 1)}
            className='group relative aspect-square cursor-zoom-in overflow-hidden bg-black/5'
          >
            {it.type === 'video' ? (
              <video src={it.url} className='h-full w-full object-cover' />
            ) : (
              <img
                src={it.url}
                alt={it.name || 'Attachment'}
                className='h-full w-full object-cover transition-transform duration-200 group-hover:scale-105'
              />
            )}
          </button>
        ))}
      </div>
    )
  }

  return (
    <div className='grid w-[240px] grid-cols-2 gap-1 overflow-hidden rounded-xl shadow-2xs sm:w-[270px]'>
      {displayItems.map((it, idx) => {
        const isLastAndMore = idx === 3 && remainingCount > 0
        return (
          <button
            key={idx}
            type='button'
            onClick={() => onImageClick?.(items, idx)}
            className='group relative aspect-square cursor-zoom-in overflow-hidden bg-black/5'
          >
            {it.type === 'video' ? (
              <video src={it.url} className='h-full w-full object-cover' />
            ) : (
              <img
                src={it.url}
                alt={it.name || 'Attachment'}
                className='h-full w-full object-cover transition-transform duration-200 group-hover:scale-105'
              />
            )}
            {isLastAndMore && (
              <div className='absolute inset-0 flex items-center justify-center bg-black/65 text-base font-extrabold tracking-wide text-white backdrop-blur-[1px]'>
                +{remainingCount + 1}
              </div>
            )}
          </button>
        )
      })}
    </div>
  )
}

// Render attachment or order_ref message bubble content
function MessageContent({
  content,
  content_type,
  attachment_url,
  attachment_name,
  onImageClick,
}: {
  content: string
  content_type: string
  attachment_url?: string
  attachment_name?: string
  attachment_size?: number
  onImageClick?: (items: MediaItem[], index: number) => void
}) {
  if (content_type === 'recalled') {
    return (
      <div className='flex items-center gap-1.5 py-0.5 text-xs italic opacity-75 select-none'>
        <Undo2 className='h-3.5 w-3.5 shrink-0' />
        <span>This message was unsent</span>
      </div>
    )
  }
  if (
    content_type === 'image' ||
    content_type === 'video' ||
    content_type === 'media_group'
  ) {
    const items = parseMediaItems(content_type, attachment_url, attachment_name)
    const isSpecialTag =
      content === '[Images]' ||
      content === '[Image]' ||
      content === '[Video]' ||
      /^\d+\s+(attachments|photos|files)$/i.test(content.trim())

    return (
      <div className='flex flex-col gap-1.5'>
        <MediaGroupGrid items={items} onImageClick={onImageClick} />
        {content && !isSpecialTag && (
          <p className='px-1 pt-0.5 text-xs leading-relaxed'>{content}</p>
        )}
      </div>
    )
  }
  if (content_type === 'order_ref') {
    const parts = (attachment_name || '').split('|')
    const orderNumber = parts[0] || content.slice(0, 8)
    const status = parts[1] || ''
    const total = parts[2] ? Number(parts[2]) : null
    const productName = parts[3] || ''
    const imageUrl = parts[4] || ''
    const itemCount = parts[5] ? Number(parts[5]) : null
    const orderUuid = content

    return (
      <Link
        to='/orders/$uuid'
        params={{ uuid: orderUuid }}
        className='group flex max-w-[250px] min-w-[200px] cursor-pointer flex-col gap-1.5 p-0.5 text-inherit hover:opacity-95'
      >
        <div className='flex items-center justify-between gap-2 border-b border-current/10 pb-1'>
          <div className='flex items-center gap-1 font-mono text-[10.5px] opacity-75'>
            <ShoppingBag className='h-3 w-3 shrink-0' />
            <span>#{orderNumber}</span>
          </div>
          <div className='flex items-center gap-1'>
            {status && (
              <span className='text-[10px] font-medium capitalize opacity-80'>
                {status}
              </span>
            )}
            <ChevronRight className='h-3 w-3 opacity-60 transition-all group-hover:translate-x-0.5 group-hover:opacity-100' />
          </div>
        </div>

        {(productName || imageUrl) && (
          <div className='flex items-center gap-2 py-0.5'>
            {imageUrl ? (
              <img
                src={imageUrl}
                alt={productName || 'product'}
                className='h-10 w-10 shrink-0 rounded-md border border-current/10 bg-black/5 object-cover'
              />
            ) : (
              <div className='flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-current/10 bg-black/5 opacity-60'>
                <Package className='h-5 w-5 stroke-[1.5]' />
              </div>
            )}
            <div className='min-w-0 flex-1'>
              <p
                className='truncate text-xs leading-tight font-medium group-hover:underline'
                title={productName}
              >
                {productName}
              </p>
              {itemCount && itemCount > 1 && (
                <p className='mt-0.5 text-[10px] opacity-70'>
                  +{itemCount - 1} other{' '}
                  {itemCount - 1 === 1 ? 'item' : 'items'}
                </p>
              )}
            </div>
          </div>
        )}

        {total !== null && (
          <div className='flex items-center justify-between border-t border-current/10 pt-1 text-[11px] opacity-90'>
            <span className='opacity-75'>
              {itemCount
                ? `${itemCount} ${itemCount === 1 ? 'item' : 'items'}`
                : 'Total:'}
            </span>
            <span className='font-semibold'>
              {new Intl.NumberFormat('vi-VN', {
                style: 'currency',
                currency: 'VND',
              }).format(total)}
            </span>
          </div>
        )}
      </Link>
    )
  }
  if (content_type === 'product_ref') {
    const parts = (attachment_name || '').split('|')
    const productName = parts[0] || content
    const price = parts[1] ? Number(parts[1]) : null
    const stock = parts[2] ? Number(parts[2]) : null
    const isOutOfStock =
      parts[5] === 'out_of_stock' || (stock !== null && stock <= 0)
    const imageUrl = attachment_url || ''
    const productUuid =
      content &&
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
        content
      )
        ? content
        : parts[6] || ''

    const cardContent = (
      <>
        <div className='flex items-center justify-between gap-2 border-b border-current/10 pb-1'>
          <div className='flex items-center gap-1 font-mono text-[10.5px] opacity-75'>
            <Tag className='h-3 w-3 shrink-0' />
            <span>Product</span>
          </div>
          <div className='flex items-center gap-1'>
            {isOutOfStock ? (
              <span className='rounded bg-red-500/20 px-1 py-0.5 text-[9px] font-semibold text-red-500'>
                Out of stock
              </span>
            ) : (
              <span className='rounded bg-emerald-500/20 px-1 py-0.5 text-[9px] font-medium text-emerald-600'>
                In stock
              </span>
            )}
            {productUuid && (
              <ChevronRight className='h-3 w-3 opacity-60 transition-all group-hover:translate-x-0.5 group-hover:opacity-100' />
            )}
          </div>
        </div>

        <div className='flex items-center gap-2 py-0.5'>
          <div className='relative h-10 w-10 shrink-0 overflow-hidden rounded-md border border-current/10 bg-black/5'>
            {imageUrl ? (
              <img
                src={imageUrl}
                alt={productName || 'product'}
                className='h-full w-full object-cover'
              />
            ) : (
              <div className='flex h-full w-full items-center justify-center opacity-60'>
                <Package className='h-5 w-5 stroke-[1.5]' />
              </div>
            )}
            {isOutOfStock && (
              <div className='absolute inset-0 flex items-center justify-center bg-black/60 p-0.5 text-center'>
                <span className='text-[7px] leading-tight font-bold tracking-wider text-white uppercase'>
                  Out of stock
                </span>
              </div>
            )}
          </div>
          <div className='min-w-0 flex-1'>
            <p
              className='truncate text-xs leading-tight font-medium group-hover:underline'
              title={productName}
            >
              {productName}
            </p>
            {price !== null && (
              <p className='mt-0.5 text-xs font-semibold'>
                {new Intl.NumberFormat('vi-VN', {
                  style: 'currency',
                  currency: 'VND',
                }).format(price)}
              </p>
            )}
          </div>
        </div>
      </>
    )

    if (productUuid) {
      return (
        <Link
          to='/products/$uuid'
          params={{ uuid: productUuid }}
          preload='intent'
          className='group flex max-w-[250px] min-w-[200px] cursor-pointer flex-col gap-1.5 p-0.5 text-inherit hover:opacity-95'
        >
          {cardContent}
        </Link>
      )
    }

    return (
      <div className='flex max-w-[250px] min-w-[200px] flex-col gap-1.5 p-0.5 text-inherit'>
        {cardContent}
      </div>
    )
  }
  return <>{content}</>
}

/** Fullscreen Gallery Lightbox Modal with Next / Prev Navigation and Thumbnails */
function MediaLightbox({
  mediaGroup,
  onClose,
}: {
  mediaGroup: { items: MediaItem[]; initialIndex: number } | null
  onClose: () => void
}) {
  const [currentIndex, setCurrentIndex] = useState(0)

  useEffect(() => {
    if (mediaGroup) {
      setCurrentIndex(mediaGroup.initialIndex || 0)
    }
  }, [mediaGroup])

  const items = mediaGroup?.items || []
  const currentItem = items[currentIndex]

  const handlePrev = useCallback(() => {
    if (items.length <= 1) return
    setCurrentIndex((prev) => (prev - 1 + items.length) % items.length)
  }, [items.length])

  const handleNext = useCallback(() => {
    if (items.length <= 1) return
    setCurrentIndex((prev) => (prev + 1) % items.length)
  }, [items.length])

  useEffect(() => {
    if (!mediaGroup) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      if (e.key === 'ArrowLeft') handlePrev()
      if (e.key === 'ArrowRight') handleNext()
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [mediaGroup, onClose, handlePrev, handleNext])

  if (!mediaGroup || !currentItem) return null

  return (
    <div
      className='fixed inset-0 z-50 flex animate-in flex-col items-center justify-between bg-slate-950/92 p-4 backdrop-blur-sm duration-150 select-none fade-in sm:p-6'
      onClick={onClose}
    >
      {/* Top action toolbar */}
      <div
        className='z-20 flex w-full shrink-0 items-center justify-between text-white'
        onClick={(e) => e.stopPropagation()}
      >
        <div className='flex items-center gap-2'>
          {items.length > 1 && (
            <span className='rounded-full bg-white/15 px-2.5 py-1 font-mono text-xs font-medium backdrop-blur-xs'>
              {currentIndex + 1} / {items.length}
            </span>
          )}
          {currentItem.name && (
            <span className='max-w-[180px] truncate text-xs text-slate-300 sm:max-w-md'>
              {currentItem.name}
            </span>
          )}
        </div>

        <div className='flex items-center gap-2'>
          <a
            href={currentItem.url}
            target='_blank'
            rel='noopener noreferrer'
            download={currentItem.name || 'media'}
            className='cursor-pointer rounded-full bg-white/10 p-2.5 text-white shadow-md transition-colors hover:bg-white/20 active:bg-white/30'
            title='Download / Open original'
          >
            <Download className='h-4 w-4' />
          </a>
          <button
            type='button'
            onClick={onClose}
            className='cursor-pointer rounded-full bg-white/10 p-2.5 text-white shadow-md transition-colors hover:bg-white/20 active:bg-white/30'
            title='Close (Esc)'
            aria-label='Close image preview'
          >
            <X className='h-4 w-4' />
          </button>
        </div>
      </div>

      {/* Main Preview with Navigation Arrows */}
      <div className='relative my-2 flex min-h-0 w-full flex-1 items-center justify-center'>
        {/* Left Arrow Button */}
        {items.length > 1 && (
          <button
            type='button'
            onClick={(e) => {
              e.stopPropagation()
              handlePrev()
            }}
            className='absolute left-2 z-20 cursor-pointer rounded-full border border-white/10 bg-black/50 p-3 text-white shadow-xl backdrop-blur-xs transition-all hover:bg-black/80 active:scale-95 sm:left-6'
            title='Previous (Left Arrow)'
            aria-label='Previous image'
          >
            <ChevronLeft className='h-6 w-6' />
          </button>
        )}

        {/* Media Content */}
        <div
          className='relative z-10 flex max-h-full max-w-5xl cursor-default items-center justify-center'
          onClick={(e) => e.stopPropagation()}
        >
          {currentItem.type === 'video' ? (
            <video
              key={currentItem.url}
              src={currentItem.url}
              controls
              autoPlay
              className='max-h-[75vh] max-w-[85vw] rounded-xl object-contain shadow-2xl'
            />
          ) : (
            <img
              key={currentItem.url}
              src={currentItem.url}
              alt={currentItem.name || 'Preview'}
              className='max-h-[75vh] max-w-[85vw] animate-in rounded-xl object-contain shadow-2xl duration-150 select-none zoom-in-95'
            />
          )}
        </div>

        {/* Right Arrow Button */}
        {items.length > 1 && (
          <button
            type='button'
            onClick={(e) => {
              e.stopPropagation()
              handleNext()
            }}
            className='absolute right-2 z-20 cursor-pointer rounded-full border border-white/10 bg-black/50 p-3 text-white shadow-xl backdrop-blur-xs transition-all hover:bg-black/80 active:scale-95 sm:right-6'
            title='Next (Right Arrow)'
            aria-label='Next image'
          >
            <ChevronRight className='h-6 w-6' />
          </button>
        )}
      </div>

      {/* Bottom Thumbnail Navigation Strip (if > 1 item) */}
      {items.length > 1 && (
        <div
          className='z-20 flex max-w-xl shrink-0 items-center gap-1.5 overflow-x-auto rounded-xl bg-black/45 px-2.5 py-1.5 backdrop-blur-xs'
          onClick={(e) => e.stopPropagation()}
        >
          {items.map((it, idx) => (
            <button
              key={idx}
              type='button'
              onClick={() => setCurrentIndex(idx)}
              className={`relative h-11 w-11 shrink-0 cursor-pointer overflow-hidden rounded-lg border-2 transition-all ${
                idx === currentIndex
                  ? 'scale-105 border-primary opacity-100 shadow-md ring-1 ring-primary/50'
                  : 'border-transparent opacity-50 hover:opacity-80'
              }`}
            >
              {it.type === 'video' ? (
                <div className='flex h-full w-full items-center justify-center bg-slate-800 text-[9px] font-bold text-white'>
                  VID
                </div>
              ) : (
                <img
                  src={it.url}
                  alt={it.name || 'thumb'}
                  className='h-full w-full object-cover'
                />
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

const route = getRouteApi('/_authenticated/chats/')

export function Chats() {
  const routeSearch = route.useSearch()
  const [search, setSearch] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')
  const [isSearchOpen, setIsSearchOpen] = useState(false)
  const searchContainerRef = useRef<HTMLDivElement | null>(null)

  const [notifPermission, setNotifPermission] =
    useState<NotificationPermission>(() => {
      if (typeof window !== 'undefined' && 'Notification' in window) {
        return Notification.permission
      }
      return 'default'
    })

  const handleToggleNotification = async () => {
    const perm = await requestNotificationPermission()
    setNotifPermission(perm)
    if (perm === 'granted') {
      playNotificationSound()
    }
  }

  const [selectedConvUUID, setSelectedConvUUID] = useState<string | null>(
    () => routeSearch?.id || null
  )

  useEffect(() => {
    if (routeSearch?.id) {
      setSelectedConvUUID(routeSearch.id)
    }
  }, [routeSearch?.id])

  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(search.trim())
    }, 300)
    return () => clearTimeout(timer)
  }, [search])

  // Close search dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        searchContainerRef.current &&
        !searchContainerRef.current.contains(e.target as Node)
      ) {
        setIsSearchOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const [statusFilter, setStatusFilter] = useState<'open' | 'closed'>('open')

  // 1. Fetch conversations list with debounced search and status filter
  const { data: conversations = [], isLoading: isConversationsLoading } =
    useAdminConversations({
      status: statusFilter,
      search: debouncedSearch || undefined,
    })

  // 1b. Search across all system users when searching
  const { data: usersData, isLoading: isSearchingUsers } = useAdminUsers({
    search: debouncedSearch || undefined,
    limit: 6,
  })

  const searchResultsUsers =
    (usersData?.data as Array<{
      uuid?: string
      full_name?: string
      email?: string
      status?: string
      level?: string
      created_at?: string
    }>) || []

  // 1c. Get or create conversation mutation
  const { mutateAsync: getOrCreateConversation } =
    useAdminGetOrCreateConversation()
  const [selectedUserLoading, setSelectedUserLoading] = useState<string | null>(
    null
  )

  const [inputText, setInputText] = useState('')
  const [replyingTo, setReplyingTo] = useState<{
    uuid: string
    sender: string
    content: string
  } | null>(null)
  const [pendingFiles, setPendingFiles] = useState<File[]>([])
  const [isUploading, setIsUploading] = useState(false)
  const [previewMediaGroup, setPreviewMediaGroup] = useState<{
    items: MediaItem[]
    initialIndex: number
  } | null>(null)

  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const scrollContainerRef = useRef<HTMLDivElement | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const typingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const prevConvUUIDRef = useRef<string | null>(null)
  const prevMessagesLengthRef = useRef(0)
  const isAtBottomRef = useRef(true)

  const handleSelectUser = async (userUuid?: string) => {
    if (!userUuid) return
    try {
      setSelectedUserLoading(userUuid)
      const conv = await getOrCreateConversation(userUuid)
      if (conv?.uuid) {
        setSelectedConvUUID(conv.uuid)
        setStatusFilter('open')
        setIsSearchOpen(false)
        setSearch('')
      }
    } catch {
      // Ignored
    } finally {
      setSelectedUserLoading(null)
    }
  }

  // 2. Fetch active conversation details
  const { data: activeConversation } = useAdminConversationDetails(
    selectedConvUUID || undefined
  )

  // 3. Fetch active conversation messages
  const { data: messages = [], isLoading: isMessagesLoading } =
    useAdminMessages(selectedConvUUID || undefined)

  // 4. Real-time WebSocket connection for active chat room & typing
  const { sendTyping } = useAdminChatRoom(selectedConvUUID)

  // 5. Mutations & Typing state
  const { mutateAsync: sendMessage, isPending: isSending } =
    useAdminSendMessage()
  const { mutate: markRead } = useAdminMarkRead()
  const { mutateAsync: closeConversation, isPending: isClosing } =
    useAdminCloseConversation()
  const { mutateAsync: reopenConversation, isPending: isReopening } =
    useAdminReopenConversation()
  const { mutateAsync: recallMessage, isPending: isRecalling } =
    useAdminRecallMessage()

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
      sendTyping(true)
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current)
      }
      typingTimeoutRef.current = setTimeout(() => {
        sendTyping(false)
      }, 2000)
    } else {
      if (typingTimeoutRef.current) {
        clearTimeout(typingTimeoutRef.current)
      }
      sendTyping(false)
    }
  }

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files ? Array.from(e.target.files) : []
    if (files.length > 0) {
      setPendingFiles((prev) => [...prev, ...files].slice(0, 10))
    }
    // Reset so same files can be selected again
    e.target.value = ''
  }

  const handleRemoveFile = (index: number) => {
    setPendingFiles((prev) => prev.filter((_, i) => i !== index))
  }

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault()
    const content = inputText.trim()
    if (
      (!content && pendingFiles.length === 0) ||
      !selectedConvUUID ||
      isSending ||
      isUploading
    )
      return

    if (typingTimeoutRef.current) {
      clearTimeout(typingTimeoutRef.current)
    }
    sendTyping(false)

    setInputText('')
    const filesToSend = [...pendingFiles]
    setPendingFiles([])

    try {
      if (filesToSend.length > 0) {
        setIsUploading(true)
        const getToken = () => useAuthStore.getState().auth.accessToken

        if (filesToSend.length === 1) {
          const file = filesToSend[0]
          const uploaded = await uploadChatFile(file, getToken)
          const isVideo = file.type.startsWith('video/')
          await sendMessage({
            conversationUUID: selectedConvUUID,
            // Caption only — never leak the filename as message content.
            content,
            content_type: isVideo ? 'video' : 'image',
            attachment_url: uploaded.url,
            attachment_name: uploaded.name,
            attachment_size: uploaded.size,
          })
        } else {
          // Parallel upload for multiple files
          const uploadPromises = filesToSend.map((file) =>
            uploadChatFile(file, getToken).then((uploaded) => ({
              url: uploaded.url,
              name: uploaded.name || file.name,
              type: file.type.startsWith('video/') ? 'video' : 'image',
              size: uploaded.size,
            }))
          )
          const uploadedItems = await Promise.all(uploadPromises)

          await sendMessage({
            conversationUUID: selectedConvUUID,
            content: content || `${filesToSend.length} attachments`,
            content_type: 'media_group',
            attachment_url: JSON.stringify(uploadedItems),
          })
        }
        setIsUploading(false)
      } else if (content) {
        let textToSend = content
        if (replyingTo) {
          const quoteSnippet =
            replyingTo.content.length > 80
              ? replyingTo.content.slice(0, 80) + '...'
              : replyingTo.content
          textToSend = `> ${quoteSnippet}\n\n${content}`
        }
        await sendMessage({
          conversationUUID: selectedConvUUID,
          content: textToSend,
        })
        setReplyingTo(null)
      }
    } catch {
      setInputText(content)
      setPendingFiles(filesToSend)
      setIsUploading(false)
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
      <DashboardHeader fixed={false} />
      <Main
        fixed
        fluid
        className='flex min-h-0 flex-1 flex-col overflow-hidden !p-0'
      >
        <div className='flex h-full min-h-0 w-full flex-1 overflow-hidden border-t'>
          {/* Left Pane: Conversation List */}
          <div
            className={cn(
              'flex h-full min-h-0 w-full shrink-0 flex-col overflow-hidden border-r bg-card md:w-80 lg:w-96',
              selectedConvUUID ? 'hidden md:flex' : 'flex'
            )}
          >
            {/* Search & Notification Permission Toggle */}
            <div className='flex shrink-0 items-center gap-2 border-b p-3'>
              <div ref={searchContainerRef} className='relative flex-1'>
                <SearchIcon className='absolute top-2.5 left-2.5 h-4 w-4 text-muted-foreground' />
                <Input
                  placeholder='Search user, email or customer...'
                  value={search}
                  onFocus={() => {
                    if (search.trim()) setIsSearchOpen(true)
                  }}
                  onChange={(e) => {
                    setSearch(e.target.value)
                    setIsSearchOpen(true)
                  }}
                  className='h-9 pr-7 pl-8 text-xs'
                />
                {search && (
                  <button
                    type='button'
                    onClick={() => {
                      setSearch('')
                      setIsSearchOpen(false)
                    }}
                    className='absolute top-2.5 right-2 rounded p-0.5 text-muted-foreground hover:text-foreground'
                    title='Clear search'
                  >
                    <X className='h-3.5 w-3.5' />
                  </button>
                )}

                {/* Debounced User Search Dropdown */}
                {isSearchOpen && debouncedSearch && (
                  <div className='absolute top-full right-0 left-0 z-50 mt-1.5 animate-in overflow-hidden rounded-lg border bg-popover text-popover-foreground shadow-xl fade-in-50 zoom-in-95'>
                    <div className='flex items-center justify-between border-b bg-muted/40 px-3 py-1.5'>
                      <span className='text-[11px] font-medium text-muted-foreground'>
                        Global User Search
                      </span>
                      {isSearchingUsers && (
                        <Loader2 className='h-3 w-3 animate-spin text-muted-foreground' />
                      )}
                    </div>

                    <div className='max-h-60 overflow-y-auto p-1'>
                      {isSearchingUsers ? (
                        <div className='flex items-center justify-center gap-2 py-4 text-xs text-muted-foreground'>
                          <Loader2 className='h-3.5 w-3.5 animate-spin' />
                          Searching users...
                        </div>
                      ) : searchResultsUsers.length === 0 ? (
                        <div className='py-4 text-center text-xs text-muted-foreground'>
                          No users found matching &ldquo;{debouncedSearch}
                          &rdquo;
                        </div>
                      ) : (
                        <div className='flex flex-col gap-0.5'>
                          {searchResultsUsers.map((u) => {
                            const isSelectedLoading =
                              selectedUserLoading === u.uuid
                            const initials = getDisplayNameInitials(
                              u.full_name || u.email || 'User'
                            )
                            return (
                              <button
                                key={u.uuid}
                                type='button'
                                disabled={isSelectedLoading}
                                onClick={() => handleSelectUser(u.uuid)}
                                className='flex w-full items-center justify-between gap-2 rounded-md p-2 text-left transition-colors hover:bg-accent hover:text-accent-foreground disabled:opacity-50'
                              >
                                <div className='flex min-w-0 items-center gap-2'>
                                  <Avatar className='h-7 w-7 shrink-0 text-[10px]'>
                                    <AvatarFallback>{initials}</AvatarFallback>
                                  </Avatar>
                                  <div className='min-w-0 flex-1'>
                                    <div className='truncate text-xs font-medium'>
                                      {u.full_name || 'Unnamed User'}
                                    </div>
                                    <div className='truncate text-[11px] text-muted-foreground'>
                                      {u.email}
                                    </div>
                                  </div>
                                </div>
                                <div className='shrink-0'>
                                  {isSelectedLoading ? (
                                    <Loader2 className='h-3.5 w-3.5 animate-spin text-primary' />
                                  ) : (
                                    <span className='inline-flex items-center gap-1 rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary hover:bg-primary/20'>
                                      <MessageSquarePlus className='h-3 w-3' />
                                      Chat
                                    </span>
                                  )}
                                </div>
                              </button>
                            )
                          })}
                        </div>
                      )}
                    </div>
                  </div>
                )}
              </div>
              <Button
                variant={notifPermission === 'granted' ? 'ghost' : 'outline'}
                size='icon'
                className='h-9 w-9 shrink-0'
                onClick={handleToggleNotification}
                title={
                  notifPermission === 'granted'
                    ? 'Notifications enabled (Click to test sound)'
                    : 'Click to enable audio chime & browser notifications'
                }
                aria-label='Toggle chat notifications'
              >
                {notifPermission === 'granted' ? (
                  <Bell className='h-4 w-4 text-primary' />
                ) : (
                  <BellOff className='h-4 w-4 text-muted-foreground' />
                )}
              </Button>
            </div>

            {/* Status Filter Tabs (Active / Archived) */}
            <div className='flex shrink-0 gap-1 border-b bg-muted/30 p-1.5'>
              <button
                type='button'
                onClick={() => setStatusFilter('open')}
                className={cn(
                  'flex-1 rounded py-1 text-center text-xs font-medium transition-all',
                  statusFilter === 'open'
                    ? 'bg-background font-semibold text-foreground shadow-xs'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                Active
              </button>
              <button
                type='button'
                onClick={() => setStatusFilter('closed')}
                className={cn(
                  'flex-1 rounded py-1 text-center text-xs font-medium transition-all',
                  statusFilter === 'closed'
                    ? 'bg-background font-semibold text-foreground shadow-xs'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                Archived
              </button>
            </div>

            {/* Conversation Items - Direct scrollable container */}
            <div className='min-h-0 flex-1 overflow-y-auto'>
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
                        <div className='relative shrink-0'>
                          <Avatar className='h-10 w-10 border'>
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
                          {conv.is_online && (
                            <span
                              className='absolute right-0 bottom-0 h-3 w-3 rounded-full border-2 border-card bg-emerald-500 shadow-xs'
                              title='Active now'
                            />
                          )}
                        </div>

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
                                <span className='font-medium text-emerald-600 italic dark:text-emerald-400'>
                                  Customer is typing...
                                </span>
                              ) : (
                                formatLastMessage(conv.last_message_content)
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
            </div>
          </div>

          {/* Right Pane: Active Conversation Room */}
          <div
            className={cn(
              'flex h-full min-h-0 flex-1 flex-col overflow-hidden bg-background',
              !selectedConvUUID ? 'hidden md:flex' : 'flex'
            )}
          >
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
                    <div className='relative shrink-0'>
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
                      {activeConversation.is_online && (
                        <span
                          className='absolute right-0 bottom-0 h-2.5 w-2.5 rounded-full border-2 border-card bg-emerald-500 shadow-xs'
                          title='Active now'
                        />
                      )}
                    </div>
                    <div>
                      <div className='flex items-center gap-2'>
                        <h4 className='text-xs leading-tight font-semibold text-foreground'>
                          {activeConversation.user_fullname ||
                            activeConversation.user_email}
                        </h4>
                        {activeConversation.is_online ? (
                          <span className='inline-flex items-center gap-1 rounded-full bg-emerald-50 px-1.5 py-0.5 text-[10px] font-medium text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400'>
                            <span className='h-1.5 w-1.5 rounded-full bg-emerald-500' />
                            Active now
                          </span>
                        ) : (
                          <span className='inline-flex items-center gap-1 rounded-full bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground'>
                            <span className='h-1.5 w-1.5 rounded-full bg-muted-foreground/50' />
                            Offline
                          </span>
                        )}
                      </div>
                      <p className='text-[11px] text-muted-foreground'>
                        {activeConversation.user_email}
                      </p>
                    </div>
                  </div>

                  {/* Header Actions: Archive / Reopen */}
                  <div className='flex items-center gap-2'>
                    {activeConversation.status === 'closed' ? (
                      <Button
                        variant='outline'
                        size='sm'
                        disabled={isReopening}
                        onClick={() =>
                          reopenConversation(activeConversation.uuid)
                        }
                        className='h-8 gap-1.5 text-xs text-emerald-600 hover:text-emerald-700 dark:text-emerald-400'
                      >
                        {isReopening ? (
                          <Loader2 className='h-3.5 w-3.5 animate-spin' />
                        ) : (
                          <ArchiveRestore className='h-3.5 w-3.5' />
                        )}
                        Reopen Chat
                      </Button>
                    ) : (
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={isClosing}
                        onClick={() =>
                          closeConversation(activeConversation.uuid)
                        }
                        className='h-8 gap-1.5 text-xs text-muted-foreground hover:text-foreground'
                        title='Archive this conversation'
                      >
                        {isClosing ? (
                          <Loader2 className='h-3.5 w-3.5 animate-spin' />
                        ) : (
                          <Archive className='h-3.5 w-3.5' />
                        )}
                        Archive
                      </Button>
                    )}
                  </div>
                </div>

                {/* Messages Stream */}
                <div
                  ref={scrollContainerRef}
                  onScroll={handleMessagesScroll}
                  className='min-h-0 flex-1 overflow-y-auto bg-muted/20 p-4'
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
                        const prevMsg =
                          index > 0 ? messages[index - 1] : undefined
                        const showDivider = shouldShowTimeDivider(
                          msg.created_at,
                          prevMsg?.created_at
                        )
                        const isAdmin = msg.sender_role === 'admin'
                        return (
                          <div key={msg.uuid} className='flex flex-col'>
                            {showDivider && (
                              <div className='my-2 flex items-center justify-center'>
                                <span className='rounded-full border bg-muted px-2.5 py-0.5 text-[11px] font-medium text-muted-foreground shadow-xs'>
                                  {formatDividerTime(msg.created_at)}
                                </span>
                              </div>
                            )}
                            <div
                              className={cn(
                                'flex w-full flex-col',
                                isAdmin ? 'items-end' : 'items-start'
                              )}
                            >
                              {(() => {
                                const isMedia =
                                  msg.content_type === 'image' ||
                                  msg.content_type === 'video' ||
                                  msg.content_type === 'media_group'
                                const isRecalled =
                                  msg.content_type === 'recalled'
                                const canRecall =
                                  isAdmin &&
                                  !isRecalled &&
                                  new Date().getTime() -
                                    new Date(msg.created_at).getTime() <=
                                    60 * 60 * 1000
                                const { replyQuote, body: cleanContent } =
                                  !isRecalled
                                    ? parseReplyContent(msg.content)
                                    : { replyQuote: null, body: msg.content }

                                return (
                                  <div
                                    className={cn(
                                      'group/msg relative flex items-center gap-1',
                                      isAdmin
                                        ? 'justify-end self-end'
                                        : 'justify-start self-start'
                                    )}
                                  >
                                    {isAdmin && (
                                      <DropdownMenu>
                                        <DropdownMenuTrigger asChild>
                                          <button
                                            type='button'
                                            className='flex h-6 w-6 cursor-pointer items-center justify-center rounded-full text-muted-foreground opacity-0 shadow-2xs transition-opacity group-hover/msg:opacity-100 hover:bg-muted hover:text-foreground'
                                            title='More actions'
                                            aria-label='More actions'
                                          >
                                            <MoreHorizontal className='h-3.5 w-3.5' />
                                          </button>
                                        </DropdownMenuTrigger>
                                        <DropdownMenuContent
                                          align='end'
                                          className='w-36 text-xs'
                                        >
                                          <DropdownMenuItem
                                            onClick={() => {
                                              setReplyingTo({
                                                uuid: msg.uuid,
                                                sender: 'You',
                                                content:
                                                  msg.content_type === 'image'
                                                    ? '[Image]'
                                                    : msg.content_type ===
                                                        'video'
                                                      ? '[Video]'
                                                      : msg.content_type ===
                                                          'order_ref'
                                                        ? '[Order]'
                                                        : msg.content_type ===
                                                            'product_ref'
                                                          ? '[Product]'
                                                          : cleanContent,
                                              })
                                              inputRef.current?.focus()
                                            }}
                                            className='cursor-pointer gap-2'
                                          >
                                            <Reply className='h-3.5 w-3.5' />
                                            <span>Reply</span>
                                          </DropdownMenuItem>

                                          {!isRecalled && msg.content && (
                                            <DropdownMenuItem
                                              onClick={() => {
                                                navigator.clipboard.writeText(
                                                  cleanContent
                                                )
                                              }}
                                              className='cursor-pointer gap-2'
                                            >
                                              <Copy className='h-3.5 w-3.5' />
                                              <span>Copy text</span>
                                            </DropdownMenuItem>
                                          )}

                                          {canRecall && (
                                            <DropdownMenuItem
                                              disabled={isRecalling}
                                              onClick={() =>
                                                recallMessage({
                                                  messageUUID: msg.uuid,
                                                  conversationUUID:
                                                    selectedConvUUID ||
                                                    undefined,
                                                })
                                              }
                                              variant='destructive'
                                              className='cursor-pointer gap-2 text-destructive'
                                            >
                                              <Undo2 className='h-3.5 w-3.5' />
                                              <span>Recall</span>
                                            </DropdownMenuItem>
                                          )}
                                        </DropdownMenuContent>
                                      </DropdownMenu>
                                    )}

                                    <div
                                      className={cn(
                                        'relative flex max-w-[75%] flex-col overflow-hidden text-xs break-words shadow-sm',
                                        isRecalled
                                          ? isAdmin
                                            ? 'rounded-lg rounded-br-none border border-dashed bg-muted/40 px-3.5 py-2 text-muted-foreground shadow-none'
                                            : 'rounded-lg rounded-bl-none border border-dashed bg-muted/40 px-3.5 py-2 text-muted-foreground shadow-none'
                                          : isMedia
                                            ? 'overflow-hidden border-0 bg-transparent p-0 shadow-none'
                                            : isAdmin
                                              ? 'rounded-lg rounded-br-none bg-primary text-primary-foreground'
                                              : 'rounded-lg rounded-bl-none border bg-card text-card-foreground'
                                      )}
                                    >
                                      {/* Darker attached top sub-bubble for reply */}
                                      {replyQuote && (
                                        <div
                                          className={cn(
                                            'flex items-center gap-1.5 border-b px-3 py-1 text-[11px] select-none',
                                            isAdmin
                                              ? 'border-white/15 bg-black/25 text-primary-foreground/90'
                                              : 'border-border/50 bg-muted/90 text-muted-foreground'
                                          )}
                                        >
                                          <Reply className='h-3 w-3 shrink-0 opacity-75' />
                                          <span className='truncate italic'>
                                            {replyQuote}
                                          </span>
                                        </div>
                                      )}

                                      <div
                                        className={cn(
                                          !isRecalled &&
                                            !isMedia &&
                                            'px-3.5 py-2'
                                        )}
                                      >
                                        <MessageContent
                                          content={cleanContent}
                                          content_type={msg.content_type}
                                          attachment_url={msg.attachment_url}
                                          attachment_name={msg.attachment_name}
                                          attachment_size={msg.attachment_size}
                                          onImageClick={(items, idx) =>
                                            setPreviewMediaGroup({
                                              items,
                                              initialIndex: idx,
                                            })
                                          }
                                        />
                                      </div>
                                    </div>

                                    {!isAdmin && (
                                      <DropdownMenu>
                                        <DropdownMenuTrigger asChild>
                                          <button
                                            type='button'
                                            className='flex h-6 w-6 cursor-pointer items-center justify-center rounded-full text-muted-foreground opacity-0 shadow-2xs transition-opacity group-hover/msg:opacity-100 hover:bg-muted hover:text-foreground'
                                            title='More actions'
                                            aria-label='More actions'
                                          >
                                            <MoreHorizontal className='h-3.5 w-3.5' />
                                          </button>
                                        </DropdownMenuTrigger>
                                        <DropdownMenuContent
                                          align='start'
                                          className='w-36 text-xs'
                                        >
                                          <DropdownMenuItem
                                            onClick={() => {
                                              setReplyingTo({
                                                uuid: msg.uuid,
                                                sender:
                                                  activeConversation?.user_fullname ||
                                                  'Customer',
                                                content:
                                                  msg.content_type === 'image'
                                                    ? '[Image]'
                                                    : msg.content_type ===
                                                        'video'
                                                      ? '[Video]'
                                                      : msg.content_type ===
                                                          'order_ref'
                                                        ? '[Order]'
                                                        : msg.content_type ===
                                                            'product_ref'
                                                          ? '[Product]'
                                                          : cleanContent,
                                              })
                                              inputRef.current?.focus()
                                            }}
                                            className='cursor-pointer gap-2'
                                          >
                                            <Reply className='h-3.5 w-3.5' />
                                            <span>Reply</span>
                                          </DropdownMenuItem>

                                          {!isRecalled && msg.content && (
                                            <DropdownMenuItem
                                              onClick={() => {
                                                navigator.clipboard.writeText(
                                                  cleanContent
                                                )
                                              }}
                                              className='cursor-pointer gap-2'
                                            >
                                              <Copy className='h-3.5 w-3.5' />
                                              <span>Copy text</span>
                                            </DropdownMenuItem>
                                          )}
                                        </DropdownMenuContent>
                                      </DropdownMenu>
                                    )}
                                  </div>
                                )
                              })()}
                              <div
                                className={cn(
                                  'mt-1 flex items-center gap-1 px-1 text-[10px] text-muted-foreground',
                                  isAdmin ? 'justify-end' : 'justify-start'
                                )}
                              >
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
                        </div>
                      )}
                      <div ref={messagesEndRef} />
                    </div>
                  )}
                </div>

                {/* Message Input Bar */}
                <div className='shrink-0 border-t bg-card p-3'>
                  {/* Reply banner */}
                  {replyingTo && (
                    <div className='mb-2 flex items-center justify-between rounded-md border-l-2 border-l-primary bg-muted/60 px-3 py-1.5 text-xs text-foreground'>
                      <div className='flex min-w-0 items-center gap-2'>
                        <Reply className='h-3.5 w-3.5 shrink-0 text-primary' />
                        <div className='min-w-0'>
                          <span className='block text-[11px] font-semibold'>
                            {replyingTo.sender}
                          </span>
                          <span className='block max-w-md truncate text-[11px] text-muted-foreground'>
                            {replyingTo.content}
                          </span>
                        </div>
                      </div>
                      <button
                        type='button'
                        onClick={() => setReplyingTo(null)}
                        className='cursor-pointer rounded p-0.5 text-muted-foreground hover:text-foreground'
                        aria-label='Cancel reply'
                      >
                        <X className='h-3.5 w-3.5' />
                      </button>
                    </div>
                  )}

                  {/* Pending files preview strip */}
                  {pendingFiles.length > 0 && (
                    <div className='mb-2 flex items-center gap-2 overflow-x-auto rounded-lg border bg-muted/40 p-2'>
                      {pendingFiles.map((file, idx) => {
                        const isImg = file.type.startsWith('image/')
                        const isVid = file.type.startsWith('video/')
                        return (
                          <div
                            key={idx}
                            className='group relative flex h-13 w-13 shrink-0 items-center justify-center overflow-hidden rounded-md border bg-card text-muted-foreground shadow-2xs'
                          >
                            {isImg ? (
                              <img
                                src={URL.createObjectURL(file)}
                                alt={file.name}
                                className='h-full w-full object-cover'
                              />
                            ) : isVid ? (
                              <span className='rounded bg-muted px-1 py-0.5 text-[9px] font-bold text-foreground'>
                                VIDEO
                              </span>
                            ) : (
                              <Paperclip className='h-4 w-4 text-muted-foreground' />
                            )}
                            <button
                              type='button'
                              onClick={() => handleRemoveFile(idx)}
                              className='absolute top-0.5 right-0.5 cursor-pointer rounded-full bg-slate-900/80 p-0.5 text-white transition-colors hover:bg-slate-900'
                              title='Remove file'
                            >
                              <X className='h-3 w-3' />
                            </button>
                          </div>
                        )
                      })}
                      <div className='shrink-0 px-1 text-[11px] font-medium text-muted-foreground'>
                        {pendingFiles.length}{' '}
                        {pendingFiles.length === 1 ? 'file' : 'files'}
                      </div>
                    </div>
                  )}
                  <form
                    onSubmit={handleSend}
                    className='flex items-center gap-2'
                  >
                    {/* Hidden file input */}
                    <input
                      ref={fileInputRef}
                      type='file'
                      multiple
                      accept='image/*,video/*'
                      className='hidden'
                      onChange={handleFileSelect}
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='h-9 w-9 shrink-0'
                      onClick={() => fileInputRef.current?.click()}
                      disabled={isSending || isUploading}
                      title='Attach image or video'
                    >
                      <Paperclip className='h-4 w-4' />
                    </Button>
                    <Input
                      ref={inputRef}
                      type='text'
                      placeholder={
                        pendingFiles.length > 0
                          ? `Add a caption for ${pendingFiles.length} ${
                              pendingFiles.length === 1 ? 'file' : 'files'
                            }...`
                          : 'Type your reply to customer...'
                      }
                      value={inputText}
                      onChange={handleInputChange}
                      disabled={isSending || isUploading}
                      className='h-9 flex-1 text-xs'
                    />
                    <Button
                      type='submit'
                      size='sm'
                      disabled={
                        (!inputText.trim() && pendingFiles.length === 0) ||
                        isSending ||
                        isUploading
                      }
                      className='h-9 gap-1.5 px-3 text-xs'
                    >
                      <Send className='h-3.5 w-3.5' />
                      {isUploading ? 'Uploading...' : 'Send'}
                    </Button>
                  </form>
                </div>
              </>
            )}
          </div>
        </div>
      </Main>

      {/* Media Lightbox Gallery Modal */}
      <MediaLightbox
        mediaGroup={previewMediaGroup}
        onClose={() => setPreviewMediaGroup(null)}
      />
    </>
  )
}
export default Chats
