import { useState } from 'react'
import { Plus, Edit2, Trash2, Image as ImageIcon, ExternalLink } from 'lucide-react'
import { toast } from 'sonner'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useBanners, useBannerMutations } from './hooks/use-banners'
import { BannerDialog } from './components/banner-dialog'
import type { Banner } from './types'

export function Banners() {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [selectedBanner, setSelectedBanner] = useState<Banner | null>(null)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [bannerToDelete, setBannerToDelete] = useState<Banner | null>(null)

  const { data, isLoading } = useBanners()
  const { updateStatus, deleteBanner } = useBannerMutations()

  const banners = (data?.data as Banner[]) || []

  const handleOpenCreate = () => {
    setSelectedBanner(null)
    setDialogOpen(true)
  }

  const handleOpenEdit = (b: Banner) => {
    setSelectedBanner(b)
    setDialogOpen(true)
  }

  const handleOpenDelete = (b: Banner) => {
    setBannerToDelete(b)
    setDeleteConfirmOpen(true)
  }

  const handleToggleStatus = async (banner: Banner) => {
    if (!banner.uuid) return
    const newStatus = !banner.is_active
    try {
      await updateStatus.mutateAsync({
        uuid: banner.uuid,
        is_active: newStatus,
      })
      toast.success(newStatus ? 'Banner activated' : 'Banner deactivated')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update status'
      toast.error(msg)
    }
  }

  const handleConfirmDelete = async () => {
    if (!bannerToDelete?.uuid) return
    try {
      await deleteBanner.mutateAsync(bannerToDelete.uuid)
      toast.success(`Banner "${bannerToDelete.title}" deleted successfully`)
      setDeleteConfirmOpen(false)
      setBannerToDelete(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete banner'
      toast.error(msg)
    }
  }

  const formatPosition = (pos?: string) => {
    switch (pos) {
      case 'home_main':
        return 'Home - Hero Banner'
      case 'home_sub':
        return 'Home - Sub Banner'
      case 'home_middle':
        return 'Home - Middle'
      case 'home_popup':
        return 'Home - Popup'
      case 'category_top':
        return 'Category - Top'
      case 'sidebar':
        return 'Sidebar'
      default:
        return pos || 'General'
    }
  }

  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Banners</h2>
            <p className='text-muted-foreground'>
              Manage promotional banners, hero sliders, and storefront placements.
            </p>
          </div>
          <Button onClick={handleOpenCreate} className='gap-2'>
            <Plus className='size-4' /> Add Banner
          </Button>
        </div>

        {/* Banners Table */}
        <div className='rounded-md border bg-card'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-[160px]'>Image</TableHead>
                <TableHead>Title</TableHead>
                <TableHead>Position</TableHead>
                <TableHead>Link URL</TableHead>
                <TableHead>Order</TableHead>
                <TableHead>Active</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={7} className='h-32 text-center text-muted-foreground'>
                    Loading banners...
                  </TableCell>
                </TableRow>
              ) : banners.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className='h-32 text-center text-muted-foreground'>
                    No banners found. Create your first banner!
                  </TableCell>
                </TableRow>
              ) : (
                banners.map((b) => (
                  <TableRow key={b.uuid}>
                    <TableCell>
                      {b.image_url ? (
                        <img
                          src={b.image_url}
                          alt={b.title}
                          className='h-12 w-28 rounded object-cover border'
                          onError={(e) => {
                            ;(e.currentTarget as HTMLImageElement).src =
                              'https://placehold.co/112x48?text=Banner'
                          }}
                        />
                      ) : (
                        <div className='h-12 w-28 rounded border bg-muted flex items-center justify-center text-muted-foreground'>
                          <ImageIcon className='size-5' />
                        </div>
                      )}
                    </TableCell>
                    <TableCell className='font-medium'>{b.title}</TableCell>
                    <TableCell>
                      <Badge variant='outline'>{formatPosition(b.position)}</Badge>
                    </TableCell>
                    <TableCell className='max-w-[200px] truncate text-muted-foreground text-xs font-mono'>
                      {b.link_url ? (
                        <a
                          href={b.link_url}
                          target='_blank'
                          rel='noopener noreferrer'
                          className='inline-flex items-center gap-1 hover:underline text-primary'
                        >
                          {b.link_url} <ExternalLink className='size-3' />
                        </a>
                      ) : (
                        '—'
                      )}
                    </TableCell>
                    <TableCell className='font-medium'>{b.order ?? 0}</TableCell>
                    <TableCell>
                      <Switch
                        checked={b.is_active}
                        onCheckedChange={() => handleToggleStatus(b)}
                        disabled={updateStatus.isPending}
                      />
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8'
                          onClick={() => handleOpenEdit(b)}
                          title='Edit'
                        >
                          <Edit2 className='size-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8 text-destructive hover:bg-destructive/10'
                          onClick={() => handleOpenDelete(b)}
                          title='Delete'
                        >
                          <Trash2 className='size-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </Main>

      <BannerDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        banner={selectedBanner}
      />

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title='Delete Banner'
        desc={`Are you sure you want to delete banner "${bannerToDelete?.title}"?`}
        confirmText='Delete'
        destructive
        isLoading={deleteBanner.isPending}
        handleConfirm={handleConfirmDelete}
      />
    </>
  )
}
