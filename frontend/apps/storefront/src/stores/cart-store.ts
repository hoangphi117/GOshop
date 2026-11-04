import { create } from 'zustand'
import { useAuthStore } from '@/stores/auth-store'
import { authFetch } from '@/lib/api'
import { toast } from 'sonner'

export interface CartItem {
  id: string // compound key: variantUuid || productUuid || itemUuid
  itemUuid?: string // Server cart_item UUID
  productUuid: string
  variantUuid?: string
  name: string
  image: string
  price: number
  compareAtPrice?: number
  quantity: number
  selectedSize?: string
  selectedColor?: string
  sku?: string
  stockQuantity: number
  selected?: boolean
  isAvailable?: boolean
}

export interface AppliedCoupon {
  code: string
  name: string
  discountAmount: number
}

interface ServerCartItemDTO {
  uuid: string
  product_uuid: string
  product_name: string
  product_slug: string
  product_image?: string
  price: number
  quantity: number
  subtotal: number
  stock_available: number
  is_available: boolean
}

interface ServerCartDTO {
  uuid: string
  total_items: number
  total_amount: number
  items: ServerCartItemDTO[]
}

interface CartState {
  items: CartItem[]
  buyNowItem: CartItem | null
  isDrawerOpen: boolean
  isSyncing: boolean
  appliedCoupon: AppliedCoupon | null
  currentUserKey: string | null

  setAppliedCoupon: (coupon: AppliedCoupon | null) => void
  initUserCart: (userKey: string | null) => void
  fetchServerCart: () => Promise<void>
  addItem: (item: Omit<CartItem, 'id'>) => boolean
  setBuyNowItem: (item: CartItem | null) => void
  clearBuyNowItem: () => void
  removeItem: (id: string) => Promise<void>
  updateQuantity: (id: string, quantity: number) => Promise<void>
  toggleSelectItem: (id: string) => void
  selectAllItems: (selected: boolean) => void
  removeSelectedItems: () => Promise<void>
  removeUnavailableItems: () => Promise<void>
  clearCart: () => Promise<void>
  openDrawer: () => void
  closeDrawer: () => void
  toggleDrawer: () => void
  getTotalItems: () => number
  getSubtotal: () => number
  getSelectedItems: () => CartItem[]
  getSelectedSubtotal: () => number
  getSelectedTotalItems: () => number
}

const getCartStorageKey = (userKey: string) => `goshop_cart_${userKey}`

const loadPersistedBuyNowItem = (): CartItem | null => {
  if (typeof window === 'undefined') return null
  try {
    const raw = sessionStorage.getItem('goshop_buy_now_item')
    if (raw) {
      return JSON.parse(raw)
    }
  } catch {
    // ignore
  }
  return null
}

const persistUserCart = (
  userKey: string | null,
  items: CartItem[],
  appliedCoupon: AppliedCoupon | null
) => {
  if (typeof window === 'undefined' || !userKey) return
  try {
    localStorage.setItem(
      getCartStorageKey(userKey),
      JSON.stringify({ items, appliedCoupon })
    )
  } catch (err) {
    console.error('Failed to persist cart:', err)
  }
}

const loadPersistedUserCart = (
  userKey: string
): { items: CartItem[]; appliedCoupon: AppliedCoupon | null } => {
  if (typeof window === 'undefined' || !userKey) {
    return { items: [], appliedCoupon: null }
  }
  try {
    const raw = localStorage.getItem(getCartStorageKey(userKey))
    if (raw) {
      const parsed = JSON.parse(raw)
      return {
        items: Array.isArray(parsed?.items) ? parsed.items : [],
        appliedCoupon: parsed?.appliedCoupon || null,
      }
    }
  } catch (err) {
    console.error('Failed to load user cart:', err)
  }
  return { items: [], appliedCoupon: null }
}

const getInitialCartState = (): {
  currentUserKey: string | null
  items: CartItem[]
  appliedCoupon: AppliedCoupon | null
  buyNowItem: CartItem | null
} => {
  if (typeof window === 'undefined') {
    return { currentUserKey: null, items: [], appliedCoupon: null, buyNowItem: null }
  }
  const buyNow = loadPersistedBuyNowItem()
  try {
    const authRaw = localStorage.getItem('goshop_storefront_auth')
    if (authRaw) {
      const parsedAuth = JSON.parse(authRaw)
      const user = parsedAuth?.state?.user
      // Access token is memory-only now; the persisted profile is enough to
      // pick the right cart bucket. Auth itself is restored separately.
      if (user) {
        const userKey = user.uuid || user.accountNo || user.email
        if (userKey) {
          const loaded = loadPersistedUserCart(userKey)
          return {
            currentUserKey: userKey,
            items: loaded.items,
            appliedCoupon: loaded.appliedCoupon,
            buyNowItem: buyNow,
          }
        }
      }
    }
  } catch {
    // ignore
  }
  return { currentUserKey: null, items: [], appliedCoupon: null, buyNowItem: buyNow }
}

const initialState = getInitialCartState()

export const useCartStore = create<CartState>((set, get) => ({
  items: initialState.items,
  buyNowItem: initialState.buyNowItem,
  isDrawerOpen: false,
  isSyncing: false,
  appliedCoupon: initialState.appliedCoupon,
  currentUserKey: initialState.currentUserKey,

  setBuyNowItem: (item) => {
    set({ buyNowItem: item })
    if (typeof window !== 'undefined') {
      try {
        if (item) {
          sessionStorage.setItem('goshop_buy_now_item', JSON.stringify(item))
        } else {
          sessionStorage.removeItem('goshop_buy_now_item')
        }
      } catch {
        // ignore
      }
    }
  },

  clearBuyNowItem: () => {
    set({ buyNowItem: null })
    if (typeof window !== 'undefined') {
      try {
        sessionStorage.removeItem('goshop_buy_now_item')
      } catch {
        // ignore
      }
    }
  },

  setAppliedCoupon: (coupon) => {
    set({ appliedCoupon: coupon })
    const { currentUserKey, items } = get()
    persistUserCart(currentUserKey, items, coupon)
  },

  initUserCart: (userKey) => {
    if (!userKey) {
      set({
        currentUserKey: null,
        items: [],
        appliedCoupon: null,
        isDrawerOpen: false,
      })
      return
    }
    const loaded = loadPersistedUserCart(userKey)
    set({
      currentUserKey: userKey,
      items: loaded.items,
      appliedCoupon: loaded.appliedCoupon,
    })

    // Automatically trigger server cart synchronization
    get().fetchServerCart()
  },

  fetchServerCart: async () => {
    const { user, accessToken } = useAuthStore.getState()
    if (!user || !accessToken) return

    set({ isSyncing: true })
    try {
      const res = await authFetch('/api/v1/cart', { method: 'GET' })
      if (res.ok) {
        const json = await res.json()
        const serverCart: ServerCartDTO = json?.data
        if (serverCart && Array.isArray(serverCart.items)) {
          const currentItems = get().items
          const mappedItems: CartItem[] = serverCart.items.map((sItem) => {
            const existing = currentItems.find(
              (ci) =>
                ci.itemUuid === sItem.uuid ||
                ci.productUuid === sItem.product_uuid
            )
            return {
              id: existing?.id || sItem.product_uuid,
              itemUuid: sItem.uuid,
              productUuid: sItem.product_uuid,
              variantUuid: existing?.variantUuid,
              name: sItem.product_name,
              image: sItem.product_image || existing?.image || '',
              price: sItem.price,
              compareAtPrice: existing?.compareAtPrice,
              quantity: sItem.quantity,
              selectedSize: existing?.selectedSize,
              selectedColor: existing?.selectedColor,
              sku: existing?.sku,
              stockQuantity: sItem.stock_available,
              isAvailable: sItem.is_available,
              selected: existing ? existing.selected !== false : true,
            }
          })

          set({ items: mappedItems })
          persistUserCart(get().currentUserKey, mappedItems, get().appliedCoupon)
        }
      }
    } catch (err) {
      console.warn('Failed to fetch server cart:', err)
    } finally {
      set({ isSyncing: false })
    }
  },

  addItem: (newItem) => {
    const { user, accessToken } = useAuthStore.getState()
    if (!accessToken || !user) {
      toast.error('Please sign in to add items to your cart.')
      return false
    }

    const userKey = user.uuid || user.accountNo || user.email
    if (get().currentUserKey !== userKey) {
      get().initUserCart(userKey)
    }

    const id = newItem.variantUuid
      ? `${newItem.productUuid}-${newItem.variantUuid}`
      : `${newItem.productUuid}-${newItem.selectedSize || 'default'}-${newItem.selectedColor || 'default'}`

    const currentItems = get().items
    const existingIndex = currentItems.findIndex((item) => item.id === id)
    let updatedItems: CartItem[]

    if (existingIndex > -1) {
      updatedItems = [...currentItems]
      const existingItem = updatedItems[existingIndex]
      const newQty = Math.min(
        existingItem.quantity + newItem.quantity,
        newItem.stockQuantity || 999
      )
      updatedItems[existingIndex] = {
        ...existingItem,
        quantity: newQty,
        selected: true,
        isAvailable: true,
      }
    } else {
      updatedItems = [
        ...currentItems,
        {
          ...newItem,
          id,
          selected: true,
          isAvailable: true,
        },
      ]
    }

    set({ items: updatedItems, isDrawerOpen: true })
    persistUserCart(userKey, updatedItems, get().appliedCoupon)

    // Sync to backend server
    authFetch('/api/v1/cart/items', {
      method: 'POST',
      body: JSON.stringify({
        product_uuid: newItem.productUuid,
        quantity: newItem.quantity,
      }),
    })
      .then((res) => res.json())
      .then((data) => {
        if (data?.data?.items && Array.isArray(data.data.items)) {
          get().fetchServerCart()
        }
      })
      .catch((err) => {
        console.warn('Background cart sync error:', err)
      })

    return true
  },

  removeItem: async (id) => {
    const target = get().items.find((item) => item.id === id)
    const updatedItems = get().items.filter((item) => item.id !== id)
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)

    if (target?.itemUuid) {
      try {
        await authFetch(`/api/v1/cart/items/${target.itemUuid}`, {
          method: 'DELETE',
        })
      } catch (err) {
        console.warn('Failed to delete cart item on server:', err)
      }
    }
  },

  updateQuantity: async (id, quantity) => {
    const target = get().items.find((item) => item.id === id)
    if (quantity <= 0) {
      await get().removeItem(id)
      return
    }

    const maxStock = target?.stockQuantity || 999
    const clampedQty = Math.min(quantity, maxStock)

    const updatedItems = get().items.map((item) =>
      item.id === id
        ? {
            ...item,
            quantity: clampedQty,
          }
        : item
    )
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)

    if (target?.itemUuid) {
      try {
        const res = await authFetch(`/api/v1/cart/items/${target.itemUuid}`, {
          method: 'PUT',
          body: JSON.stringify({ quantity: clampedQty }),
        })
        if (!res.ok) {
          const errData = await res.json().catch(() => null)
          toast.error(errData?.detail || errData?.message || 'Failed to update item quantity')
          get().fetchServerCart()
        }
      } catch (err) {
        console.warn('Failed to update cart item quantity on server:', err)
      }
    }
  },

  toggleSelectItem: (id) => {
    const updatedItems = get().items.map((item) =>
      item.id === id
        ? { ...item, selected: item.selected === false ? true : false }
        : item
    )
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)
  },

  selectAllItems: (selected) => {
    const updatedItems = get().items.map((item) => ({
      ...item,
      selected: item.isAvailable !== false ? selected : false,
    }))
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)
  },

  removeSelectedItems: async () => {
    const selected = get().items.filter((item) => item.selected !== false)
    const updatedItems = get().items.filter((item) => item.selected === false)
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)

    // Sync deletes with server in parallel non-blocking background
    if (selected.length > 0) {
      Promise.allSettled(
        selected
          .filter((item) => item.itemUuid)
          .map((item) =>
            authFetch(`/api/v1/cart/items/${item.itemUuid}`, {
              method: 'DELETE',
            }).catch((err) =>
              console.warn('Failed to delete selected item on server:', err)
            )
          )
      )
    }
  },

  removeUnavailableItems: async () => {
    const unavailable = get().items.filter(
      (item) => item.isAvailable === false || item.stockQuantity <= 0
    )
    if (unavailable.length === 0) return

    const remaining = get().items.filter(
      (item) => item.isAvailable !== false && item.stockQuantity > 0
    )
    set({ items: remaining })
    persistUserCart(get().currentUserKey, remaining, get().appliedCoupon)

    for (const item of unavailable) {
      if (item.itemUuid) {
        try {
          await authFetch(`/api/v1/cart/items/${item.itemUuid}`, {
            method: 'DELETE',
          })
        } catch (err) {
          console.warn('Failed to delete unavailable item on server:', err)
        }
      }
    }
    toast.success(`Removed ${unavailable.length} unavailable item(s).`)
  },

  clearCart: async () => {
    const key = get().currentUserKey
    set({ items: [], appliedCoupon: null })
    if (key && typeof window !== 'undefined') {
      try {
        localStorage.removeItem(getCartStorageKey(key))
      } catch {
        // ignore
      }
    }
    try {
      await authFetch('/api/v1/cart', { method: 'DELETE' })
    } catch (err) {
      console.warn('Failed to clear cart on server:', err)
    }
  },

  openDrawer: () => {
    const { user, accessToken } = useAuthStore.getState()
    if (!user || !accessToken) {
      return
    }
    set({ isDrawerOpen: true })
    get().fetchServerCart()
  },
  closeDrawer: () => set({ isDrawerOpen: false }),
  toggleDrawer: () => {
    const { user, accessToken } = useAuthStore.getState()
    if (!user || !accessToken) {
      return
    }
    const nextState = !get().isDrawerOpen
    set({ isDrawerOpen: nextState })
    if (nextState) {
      get().fetchServerCart()
    }
  },

  getTotalItems: () => {
    return get().items.length
  },

  getSubtotal: () => {
    return get().items.reduce(
      (total, item) => total + item.price * item.quantity,
      0
    )
  },

  getSelectedItems: () => {
    return get().items.filter(
      (item) => item.selected !== false && item.isAvailable !== false
    )
  },

  getSelectedSubtotal: () => {
    return get()
      .items.filter(
        (item) => item.selected !== false && item.isAvailable !== false
      )
      .reduce((total, item) => total + item.price * item.quantity, 0)
  },

  getSelectedTotalItems: () => {
    return get().items.filter(
      (item) => item.selected !== false && item.isAvailable !== false
    ).length
  },
}))

// Auto-sync cart with auth store subscription. Keyed on the user only: the
// access token is memory-only and flips on every reload/refresh, which must
// not reset the cart bucket.
if (typeof window !== 'undefined') {
  useAuthStore.subscribe((state, prevState) => {
    const currentKey = state.user
      ? state.user.uuid || state.user.accountNo || state.user.email
      : null
    const prevKey = prevState?.user
      ? prevState.user.uuid || prevState.user.accountNo || prevState.user.email
      : null
    if (currentKey !== prevKey) {
      useCartStore.getState().initUserCart(currentKey)
    }
  })

  // Clean legacy shared storage key to eliminate cross-account leakage from past sessions
  try {
    localStorage.removeItem('goshop_storefront_cart')
  } catch {
    // ignore
  }
}
