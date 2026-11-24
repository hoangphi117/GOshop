import { TypedUseSelectorHook, useDispatch, useSelector } from 'react-redux'
import type { RootState, AppDispatch } from '@/stores/redux-store'

/** Typed dispatch hook — use everywhere instead of plain `useDispatch`. */
export const useAppDispatch = () => useDispatch<AppDispatch>()

/** Typed selector hook — use everywhere instead of plain `useSelector`. */
export const useAppSelector: TypedUseSelectorHook<RootState> = useSelector
