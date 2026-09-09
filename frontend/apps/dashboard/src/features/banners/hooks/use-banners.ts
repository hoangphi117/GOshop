import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { BannerFormData } from '../types'

export function useBanners() {
  return useQuery({
    queryKey: ['adminBanners'],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/banners', {
        params: {
          query: {
            deleted: false,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch banners')
      }
      return data
    },
  })
}

export function useBannerMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['adminBanners'] })
  }

  const createBanner = useMutation({
    mutationFn: async (payload: BannerFormData) => {
      const { data, error } = await api.POST('/admin/banners', {
        body: {
          title: payload.title,
          image_url: payload.image_url,
          link_url: payload.link_url,
          position: payload.position,
          order: payload.order,
          is_active: payload.is_active,
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to create banner')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const updateBanner = useMutation({
    mutationFn: async ({ uuid, payload }: { uuid: string; payload: BannerFormData }) => {
      const { data, error } = await api.PUT('/admin/banners/{uuid}', {
        params: { path: { uuid } },
        body: {
          title: payload.title,
          image_url: payload.image_url,
          link_url: payload.link_url,
          position: payload.position,
          order: payload.order,
          is_active: payload.is_active,
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to update banner')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const updateStatus = useMutation({
    mutationFn: async ({ uuid, is_active }: { uuid: string; is_active: boolean }) => {
      const { data, error } = await api.PATCH('/admin/banners/{uuid}/status', {
        params: { path: { uuid } },
        body: { is_active },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to update banner status')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const deleteBanner = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/admin/banners/{uuid}', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to delete banner')
      }
      return data
    },
    onSuccess: invalidate,
  })

  return {
    createBanner,
    updateBanner,
    updateStatus,
    deleteBanner,
  }
}
