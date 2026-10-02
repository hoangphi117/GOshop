import z from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { Chats } from '@/features/chats'

const chatsSearchSchema = z.object({
  id: z.string().optional(),
})

export const Route = createFileRoute('/_authenticated/chats/')({
  validateSearch: chatsSearchSchema,
  component: Chats,
})

