import { createFileRoute } from '@tanstack/react-router'
import { Coupons } from '@/features/coupons'

export const Route = createFileRoute('/_authenticated/coupons/')({
  component: Coupons,
})
