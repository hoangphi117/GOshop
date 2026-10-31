import type { MetadataRoute } from 'next'
import { API_BASE_URL, API_KEY } from '@/lib/api'

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || 'http://localhost:3001'

  const staticRoutes = [
    '',
    '/products',
    '/about',
    '/contact',
    '/faq',
    '/terms',
    '/privacy',
    '/shipping-policy',
    '/return-policy',
    '/cart',
    '/login',
    '/register',
    '/forgot-password',
  ]

  const staticEntries: MetadataRoute.Sitemap = staticRoutes.map((route) => ({
    url: `${siteUrl}${route}`,
    lastModified: new Date(),
    changeFrequency: route === '' || route === '/products' ? 'daily' : 'weekly',
    priority: route === '' ? 1.0 : route === '/products' ? 0.9 : 0.7,
  }))

  // Attempt to fetch dynamic products list for sitemap if available
  try {
    const res = await fetch(`${API_BASE_URL}/api/v1/products?page=1&limit=50`, {
      headers: { 'X-API-KEY': API_KEY },
      next: { revalidate: 3600 },
    })

    if (res.ok) {
      const data = await res.json()
      const products = data?.data?.items || data?.data || []
      if (Array.isArray(products)) {
        const productEntries: MetadataRoute.Sitemap = (
          products as { slug?: string; uuid?: string; updated_at?: string }[]
        ).map((p) => ({
          url: `${siteUrl}/products/${p.slug || p.uuid}`,
          lastModified: p.updated_at ? new Date(p.updated_at) : new Date(),
          changeFrequency: 'weekly',
          priority: 0.8,
        }))
        return [...staticEntries, ...productEntries]
      }
    }
  } catch {
    // Return static entries on API failure
  }

  return staticEntries
}
