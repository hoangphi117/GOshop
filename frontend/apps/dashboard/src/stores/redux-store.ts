import { configureStore } from '@reduxjs/toolkit'
import variantReducer from '@/features/products/store/variant-slice'

export const store = configureStore({
  reducer: {
    variants: variantReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch
