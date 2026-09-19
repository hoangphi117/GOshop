import { Download, FileSpreadsheet } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type {
  OrderStatusDistributionItem,
  OverviewStats,
  PaymentMethodBreakdownItem,
  RevenueChartPoint,
  TopProduct,
} from '../types'
import {
  exportFullAnalyticsReport,
  exportOrderStatus,
  exportOverviewKPI,
  exportPaymentMethods,
  exportRevenueDaily,
  exportTopProducts,
} from '../utils/export-utils'

interface DashboardExportMenuProps {
  stats?: OverviewStats
  revenuePoints?: RevenueChartPoint[]
  topProducts?: TopProduct[]
  orderStatus?: OrderStatusDistributionItem[]
  paymentMethods?: PaymentMethodBreakdownItem[]
  dateRangeLabel: string
  disabled?: boolean
}

export function DashboardExportMenu({
  stats,
  revenuePoints,
  topProducts,
  orderStatus,
  paymentMethods,
  dateRangeLabel,
  disabled = false,
}: DashboardExportMenuProps) {
  const handleExportFull = () => {
    exportFullAnalyticsReport(
      {
        stats,
        revenuePoints,
        topProducts,
        orderStatus,
        paymentMethods,
      },
      dateRangeLabel
    )
    toast.success('Consolidated analytics report exported')
  }

  const handleExportKPI = () => {
    exportOverviewKPI(stats, dateRangeLabel)
    toast.success('KPI summary exported')
  }

  const handleExportRevenue = () => {
    exportRevenueDaily(revenuePoints, dateRangeLabel)
    toast.success('Daily revenue data exported')
  }

  const handleExportTopProducts = () => {
    exportTopProducts(topProducts, dateRangeLabel)
    toast.success('Top products exported')
  }

  const handleExportOrderStatus = () => {
    exportOrderStatus(orderStatus, dateRangeLabel)
    toast.success('Order status distribution exported')
  }

  const handleExportPaymentMethods = () => {
    exportPaymentMethods(paymentMethods, dateRangeLabel)
    toast.success('Payment methods breakdown exported')
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant='outline'
          size='sm'
          disabled={disabled}
          className='h-8 gap-1.5 text-xs'
        >
          <Download className='h-3.5 w-3.5' />
          <span>Export</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-56'>
        <DropdownMenuLabel className='text-xs'>Export Reports (CSV / Excel)</DropdownMenuLabel>
        <DropdownMenuItem onClick={handleExportFull} className='cursor-pointer text-xs font-medium'>
          <FileSpreadsheet className='mr-2 h-4 w-4 text-primary' />
          <span>Full Analytics Report</span>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={handleExportKPI} className='cursor-pointer text-xs'>
          KPI Summary (CSV)
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleExportRevenue} className='cursor-pointer text-xs'>
          Daily Revenue Timeline (CSV)
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleExportTopProducts} className='cursor-pointer text-xs'>
          Top Selling Products (CSV)
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleExportOrderStatus} className='cursor-pointer text-xs'>
          Order Status Breakdown (CSV)
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleExportPaymentMethods} className='cursor-pointer text-xs'>
          Payment Methods Breakdown (CSV)
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
