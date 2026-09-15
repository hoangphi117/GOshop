import { createFileRoute } from '@tanstack/react-router'
import { OrderDetail } from '@/features/orders/components/order-detail'

export const Route = createFileRoute('/_authenticated/orders/$uuid')({
  component: OrderDetailRoute,
})

// eslint-disable-next-line react-refresh/only-export-components
function OrderDetailRoute() {
  const { uuid } = Route.useParams()
  return <OrderDetail uuid={uuid} />
}
