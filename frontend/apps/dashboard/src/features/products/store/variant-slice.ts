import { createSlice, createAsyncThunk, type PayloadAction } from '@reduxjs/toolkit'
import { fetchWithAuth } from '@/lib/api'
import type { ProductVariant, CreateVariantInput, UpdateVariantInput } from '../types'

interface VariantState {
  variants: ProductVariant[]
  isLoading: boolean
  error: string | null
  selectedVariant: ProductVariant | null
}

const initialState: VariantState = {
  variants: [],
  isLoading: false,
  error: null,
  selectedVariant: null,
}

export const fetchVariants = createAsyncThunk<
  ProductVariant[],
  string,
  { rejectValue: string }
>('variants/fetchVariants', async (productUuid, { rejectWithValue }) => {
  try {
    const res = await fetchWithAuth(
      `/api/v1/admin/products/${productUuid}/variants?active_only=false`,
      { method: 'GET' }
    )
    const json = await res.json()
    if (!res.ok) {
      return rejectWithValue(json.error || json.message || 'Failed to fetch variants')
    }
    return (json.data ?? []) as ProductVariant[]
  } catch (err) {
    return rejectWithValue(err instanceof Error ? err.message : 'Network error')
  }
})

export const createVariant = createAsyncThunk<
  ProductVariant,
  { productUuid: string; data: CreateVariantInput },
  { rejectValue: string }
>('variants/createVariant', async ({ productUuid, data }, { rejectWithValue }) => {
  try {
    const res = await fetchWithAuth(
      `/api/v1/admin/products/${productUuid}/variants`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      }
    )
    const json = await res.json()
    if (!res.ok) {
      return rejectWithValue(json.error || json.message || 'Failed to create variant')
    }
    return json.data as ProductVariant
  } catch (err) {
    return rejectWithValue(err instanceof Error ? err.message : 'Network error')
  }
})

export const updateVariant = createAsyncThunk<
  ProductVariant,
  { productUuid: string; variantUuid: string; data: UpdateVariantInput },
  { rejectValue: string }
>(
  'variants/updateVariant',
  async ({ productUuid, variantUuid, data }, { rejectWithValue }) => {
    try {
      const res = await fetchWithAuth(
        `/api/v1/admin/products/${productUuid}/variants/${variantUuid}`,
        {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(data),
        }
      )
      const json = await res.json()
      if (!res.ok) {
        return rejectWithValue(json.error || json.message || 'Failed to update variant')
      }
      return json.data as ProductVariant
    } catch (err) {
      return rejectWithValue(err instanceof Error ? err.message : 'Network error')
    }
  }
)

export const deleteVariant = createAsyncThunk<
  string,
  { productUuid: string; variantUuid: string },
  { rejectValue: string }
>(
  'variants/deleteVariant',
  async ({ productUuid, variantUuid }, { rejectWithValue }) => {
    try {
      const res = await fetchWithAuth(
        `/api/v1/admin/products/${productUuid}/variants/${variantUuid}`,
        { method: 'DELETE' }
      )
      const json = await res.json()
      if (!res.ok) {
        return rejectWithValue(json.error || json.message || 'Failed to delete variant')
      }
      return variantUuid
    } catch (err) {
      return rejectWithValue(err instanceof Error ? err.message : 'Network error')
    }
  }
)

export const variantSlice = createSlice({
  name: 'variants',
  initialState,
  reducers: {
    setSelectedVariant: (state, action: PayloadAction<ProductVariant | null>) => {
      state.selectedVariant = action.payload
    },
    clearVariants: (state) => {
      state.variants = []
      state.selectedVariant = null
      state.error = null
    },
    clearError: (state) => {
      state.error = null
    },
  },
  extraReducers: (builder) => {
    // fetchVariants
    builder
      .addCase(fetchVariants.pending, (state) => {
        state.isLoading = true
        state.error = null
      })
      .addCase(fetchVariants.fulfilled, (state, action) => {
        state.isLoading = false
        state.variants = action.payload
      })
      .addCase(fetchVariants.rejected, (state, action) => {
        state.isLoading = false
        state.error = action.payload ?? 'Failed to load variants'
      })

    // createVariant
    builder
      .addCase(createVariant.pending, (state) => {
        state.isLoading = true
        state.error = null
      })
      .addCase(createVariant.fulfilled, (state, action) => {
        state.isLoading = false
        state.variants.push(action.payload)
      })
      .addCase(createVariant.rejected, (state, action) => {
        state.isLoading = false
        state.error = action.payload ?? 'Failed to create variant'
      })

    // updateVariant
    builder
      .addCase(updateVariant.pending, (state) => {
        state.isLoading = true
        state.error = null
      })
      .addCase(updateVariant.fulfilled, (state, action) => {
        state.isLoading = false
        const index = state.variants.findIndex((v) => v.uuid === action.payload.uuid)
        if (index !== -1) {
          state.variants[index] = action.payload
        }
        if (state.selectedVariant?.uuid === action.payload.uuid) {
          state.selectedVariant = action.payload
        }
      })
      .addCase(updateVariant.rejected, (state, action) => {
        state.isLoading = false
        state.error = action.payload ?? 'Failed to update variant'
      })

    // deleteVariant
    builder
      .addCase(deleteVariant.pending, (state) => {
        state.isLoading = true
        state.error = null
      })
      .addCase(deleteVariant.fulfilled, (state, action) => {
        state.isLoading = false
        state.variants = state.variants.filter((v) => v.uuid !== action.payload)
        if (state.selectedVariant?.uuid === action.payload) {
          state.selectedVariant = null
        }
      })
      .addCase(deleteVariant.rejected, (state, action) => {
        state.isLoading = false
        state.error = action.payload ?? 'Failed to delete variant'
      })
  },
})

export const { setSelectedVariant, clearVariants, clearError } = variantSlice.actions
export default variantSlice.reducer
