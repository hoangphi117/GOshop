import { getRouteApi } from '@tanstack/react-router'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import { UsersDialogs } from './components/users-dialogs'
import { UsersPrimaryButtons } from './components/users-primary-buttons'
import { UsersProvider } from './components/users-provider'
import { UsersTable } from './components/users-table'
import { useAdminUsers } from './hooks/use-users'
import type { User } from './data/schema'

const route = getRouteApi('/_authenticated/users/')

export function Users() {
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const { data: apiData, isLoading } = useAdminUsers({
    page: search.page,
    limit: search.pageSize,
    search: search.username || undefined,
  })

  const rawUsers = (apiData?.data as Array<{
    uuid?: string
    full_name?: string
    email?: string
    status?: string
    level?: string
    created_at?: string
  }>) || []

  const apiPagination = (
    apiData as unknown as {
      pagination?: { total_pages?: number; total_records?: number }
    }
  )?.pagination
  const pageCount = apiPagination?.total_pages ?? 1

  const users: User[] = rawUsers.map((u) => {
    const names = (u.full_name || 'User').split(' ')
    const firstName = names[0] || 'User'
    const lastName = names.slice(1).join(' ') || ''
    const statusLower = u.status?.toLowerCase()
    const status =
      statusLower === 'active' || statusLower === 'show' || statusLower === '1'
        ? ('active' as const)
        : statusLower === 'banned' || statusLower === 'suspended' || statusLower === '3'
        ? ('suspended' as const)
        : ('inactive' as const)
    const levelLower = u.level?.toLowerCase()
    const role =
      levelLower === 'admin'
        ? ('admin' as const)
        : levelLower === 'superadmin'
        ? ('superadmin' as const)
        : ('cashier' as const)

    return {
      id: u.uuid || '',
      firstName,
      lastName,
      username: u.email ? u.email.split('@')[0] : 'user',
      email: u.email || '',
      phoneNumber: '',
      status,
      role,
      createdAt: u.created_at ? new Date(u.created_at) : new Date(),
      updatedAt: new Date(),
    }
  })

  return (
    <UsersProvider>
        <DashboardHeader fixed />

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Users</h2>
            <p className='text-muted-foreground'>
              Manage user accounts, roles, and account statuses.
            </p>
          </div>
          <UsersPrimaryButtons />
        </div>
        <UsersTable data={users} search={search} navigate={navigate} isLoading={isLoading} pageCount={pageCount} />
      </Main>

      <UsersDialogs />
    </UsersProvider>
  )
}
