import { createFileRoute } from '@tanstack/react-router'
import { Banners } from '@/features/banners'

export const Route = createFileRoute('/_authenticated/banners/')({
  component: Banners,
})
