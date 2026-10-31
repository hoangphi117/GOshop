import { Badge } from '@/components/ui/badge'
import {
  ShoppingBag,
  RotateCcw,
  Undo2,
  PlusCircle,
  Sliders,
  Sparkles,
  AlertTriangle,
} from 'lucide-react'

interface MovementTypeBadgeProps {
  type: string
}

export function MovementTypeBadge({ type }: MovementTypeBadgeProps) {
  switch (type) {
    case 'ORDER_PLACED':
      return (
        <Badge
          variant='outline'
          className='gap-1 bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20 text-[11px] font-medium'
        >
          <ShoppingBag className='size-3' /> Order Placed
        </Badge>
      )
    case 'ORDER_CANCELLED':
      return (
        <Badge
          variant='outline'
          className='gap-1 bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20 text-[11px] font-medium'
        >
          <RotateCcw className='size-3' /> Order Cancelled
        </Badge>
      )
    case 'ORDER_RETURNED':
      return (
        <Badge
          variant='outline'
          className='gap-1 bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20 text-[11px] font-medium'
        >
          <Undo2 className='size-3' /> Order Returned
        </Badge>
      )
    case 'RESTOCK':
      return (
        <Badge
          variant='outline'
          className='gap-1 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 text-[11px] font-medium'
        >
          <PlusCircle className='size-3' /> Restock
        </Badge>
      )
    case 'MANUAL_ADJUSTMENT':
      return (
        <Badge
          variant='outline'
          className='gap-1 bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20 text-[11px] font-medium'
        >
          <Sliders className='size-3' /> Manual Adjust
        </Badge>
      )
    case 'INITIAL_STOCK':
      return (
        <Badge
          variant='outline'
          className='gap-1 bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/20 text-[11px] font-medium'
        >
          <Sparkles className='size-3' /> Initial Stock
        </Badge>
      )
    case 'DAMAGED_DISCARD':
      return (
        <Badge
          variant='outline'
          className='gap-1 bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20 text-[11px] font-medium'
        >
          <AlertTriangle className='size-3' /> Damaged / Write-off
        </Badge>
      )
    default:
      return (
        <Badge variant='outline' className='text-[11px] font-medium'>
          {type}
        </Badge>
      )
  }
}
