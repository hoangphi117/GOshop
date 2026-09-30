import { createFileRoute } from '@tanstack/react-router'
import { ProductDetail } from '@/features/products/components/product-detail'

export const Route = createFileRoute('/_authenticated/products/$uuid')({
  component: ProductDetailRoute,
})

// eslint-disable-next-line react-refresh/only-export-components
function ProductDetailRoute() {
  const { uuid } = Route.useParams()
  return <ProductDetail uuid={uuid} />
}
