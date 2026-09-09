import { create } from 'zustand'
import { persist, createJSONStorage } from 'zustand/middleware'

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
}

interface CartState {
  items: CartItem[]
  isDrawerOpen: boolean
  addItem: (item: Omit<CartItem, 'id'>) => void
  removeItem: (id: string) => void
  updateQuantity: (id: string, quantity: number) => void
  clearCart: () => void
  openDrawer: () => void
  closeDrawer: () => void
  toggleDrawer: () => void
  getTotalItems: () => number
  getSubtotal: () => number
}

export const useCartStore = create<CartState>()(
  persist(
    (set, get) => ({
      items: [],
      isDrawerOpen: false,

      addItem: (newItem) => {
        const id = newItem.variantUuid
          ? `${newItem.productUuid}-${newItem.variantUuid}`
          : `${newItem.productUuid}-${newItem.selectedSize || 'default'}-${newItem.selectedColor || 'default'}`

        set((state) => {
          const existingIndex = state.items.findIndex((item) => item.id === id)
          if (existingIndex > -1) {
            const updatedItems = [...state.items]
            const existingItem = updatedItems[existingIndex]
            const newQty = Math.min(
              existingItem.quantity + newItem.quantity,
              newItem.stockQuantity || 999
            )
            updatedItems[existingIndex] = {
              ...existingItem,
              quantity: newQty,
            }
            return { items: updatedItems, isDrawerOpen: true }
          } else {
            return {
              items: [...state.items, { ...newItem, id }],
              isDrawerOpen: true,
            }
          }
        })
      },

      removeItem: (id) => {
        set((state) => ({
          items: state.items.filter((item) => item.id !== id),
        }))
      },

      updateQuantity: (id, quantity) => {
        if (quantity <= 0) {
          get().removeItem(id)
          return
        }
        set((state) => ({
          items: state.items.map((item) =>
            item.id === id
              ? {
                  ...item,
                  quantity: Math.min(quantity, item.stockQuantity || 999),
                }
              : item
          ),
        }))
      },

      clearCart: () => set({ items: [] }),

      openDrawer: () => set({ isDrawerOpen: true }),
      closeDrawer: () => set({ isDrawerOpen: false }),
      toggleDrawer: () => set((state) => ({ isDrawerOpen: !state.isDrawerOpen })),

      getTotalItems: () => {
        return get().items.reduce((total, item) => total + item.quantity, 0)
      },

      getSubtotal: () => {
        return get().items.reduce(
          (total, item) => total + item.price * item.quantity,
          0
        )
      },
    }),
    {
      name: 'goshop_storefront_cart',
      storage: createJSONStorage(() =>
        typeof window !== 'undefined'
          ? localStorage
          : {
              getItem: () => null,
              setItem: () => {},
              removeItem: () => {},
            }
      ),
      partialize: (state) => ({ items: state.items }),
    }
  )
)
