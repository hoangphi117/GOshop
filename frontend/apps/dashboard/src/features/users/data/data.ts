import { Shield, UserCheck, Users, CreditCard } from 'lucide-react'
import { type UserStatus } from './schema'

export const callTypes = new Map<UserStatus, string>([
  ['active', 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'],
  ['inactive', 'bg-secondary text-secondary-foreground border-transparent'],
  ['invited', 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/20'],
  [
    'suspended',
    'bg-destructive/10 text-destructive border-destructive/20',
  ],
])

export const roles = [
  {
    label: 'Superadmin',
    value: 'superadmin',
    icon: Shield,
  },
  {
    label: 'Admin',
    value: 'admin',
    icon: UserCheck,
  },
  {
    label: 'Manager',
    value: 'manager',
    icon: Users,
  },
  {
    label: 'Cashier',
    value: 'cashier',
    icon: CreditCard,
  },
] as const
