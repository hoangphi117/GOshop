export type BannerPosition =
  | 'home_main'
  | 'home_sub'
  | 'home_middle'
  | 'home_popup'
  | 'category_top'
  | 'sidebar'
  | 'common'

export interface Banner {
  uuid?: string
  title?: string
  image_url?: string
  link_url?: string
  position?: string
  order?: number
  is_active?: boolean
  created_at?: string
}

export interface BannerFormData {
  title: string
  image_url: string
  link_url?: string
  position: BannerPosition
  order: number
  is_active: boolean
}
