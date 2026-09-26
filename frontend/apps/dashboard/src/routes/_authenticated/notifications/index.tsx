import { createFileRoute } from '@tanstack/react-router'
import { NotificationsPage } from '@/features/notifications'

export const Route = createFileRoute('/_authenticated/notifications/')({
  component: NotificationsPage,
})
