import { createFileRoute } from '@tanstack/react-router'
import { Inventory } from '@/features/inventory'

export const Route = createFileRoute('/_authenticated/inventory/')({
  component: Inventory,
})
