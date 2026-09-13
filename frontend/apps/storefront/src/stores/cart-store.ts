import { create } from 'zustand'
import { useAuthStore } from '@/stores/auth-store'
import { toast } from 'sonner'

export interface CartItem {
  id: string // compound key: variantUuid || productUuid
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
}

export interface AppliedCoupon {
  code: string
  name: string
  discountAmount: number
}

interface CartState {
  items: CartItem[]
  buyNowItem: CartItem | null
  isDrawerOpen: boolean
  appliedCoupon: AppliedCoupon | null
  currentUserKey: string | null
  setAppliedCoupon: (coupon: AppliedCoupon | null) => void
  initUserCart: (userKey: string | null) => void
  addItem: (item: Omit<CartItem, 'id'>) => boolean
  setBuyNowItem: (item: CartItem | null) => void
  clearBuyNowItem: () => void
  removeItem: (id: string) => void
  updateQuantity: (id: string, quantity: number) => void
  toggleSelectItem: (id: string) => void
  selectAllItems: (selected: boolean) => void
  removeSelectedItems: () => void
  clearCart: () => void
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
      const token = parsedAuth?.state?.accessToken
      if (user && token) {
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
      }
    } else {
      updatedItems = [...currentItems, { ...newItem, id, selected: true }]
    }

    set({ items: updatedItems, isDrawerOpen: true })
    persistUserCart(userKey, updatedItems, get().appliedCoupon)
    return true
  },

  removeItem: (id) => {
    const updatedItems = get().items.filter((item) => item.id !== id)
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)
  },

  updateQuantity: (id, quantity) => {
    if (quantity <= 0) {
      get().removeItem(id)
      return
    }
    const updatedItems = get().items.map((item) =>
      item.id === id
        ? {
            ...item,
            quantity: Math.min(quantity, item.stockQuantity || 999),
          }
        : item
    )
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)
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
    const updatedItems = get().items.map((item) => ({ ...item, selected }))
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)
  },

  removeSelectedItems: () => {
    const updatedItems = get().items.filter((item) => item.selected === false)
    set({ items: updatedItems })
    persistUserCart(get().currentUserKey, updatedItems, get().appliedCoupon)
  },

  clearCart: () => {
    const key = get().currentUserKey
    set({ items: [], appliedCoupon: null })
    if (key && typeof window !== 'undefined') {
      try {
        localStorage.removeItem(getCartStorageKey(key))
      } catch {
        // ignore
      }
    }
  },

  openDrawer: () => {
    const { user, accessToken } = useAuthStore.getState()
    if (!user || !accessToken) {
      return
    }
    set({ isDrawerOpen: true })
  },
  closeDrawer: () => set({ isDrawerOpen: false }),
  toggleDrawer: () => {
    const { user, accessToken } = useAuthStore.getState()
    if (!user || !accessToken) {
      return
    }
    set((state) => ({ isDrawerOpen: !state.isDrawerOpen }))
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
    return get().items.filter((item) => item.selected !== false)
  },

  getSelectedSubtotal: () => {
    return get()
      .items.filter((item) => item.selected !== false)
      .reduce((total, item) => total + item.price * item.quantity, 0)
  },

  getSelectedTotalItems: () => {
    return get().items.filter((item) => item.selected !== false).length
  },
}))

// Auto-sync cart with auth store subscription
if (typeof window !== 'undefined') {
  useAuthStore.subscribe((state, prevState) => {
    const currentKey =
      state.user && state.accessToken
        ? state.user.uuid || state.user.accountNo || state.user.email
        : null
    const prevKey =
      prevState?.user && prevState?.accessToken
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
