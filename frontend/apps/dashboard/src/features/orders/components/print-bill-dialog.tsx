import { useState } from 'react'
import { Printer, ExternalLink, Loader2, FileText } from 'lucide-react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { useShippingMutations } from '../hooks/use-shipping'
import type { PrintBillResponse } from '../types'

interface PrintBillDialogProps {
  orderUuid: string
  shippingCode: string
}

export function PrintBillDialog({ orderUuid, shippingCode }: PrintBillDialogProps) {
  const [open, setOpen] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [billData, setBillData] = useState<PrintBillResponse | null>(null)
  const { fetchPrintBill } = useShippingMutations()

  const handleOpen = async (isOpen: boolean) => {
    setOpen(isOpen)
    if (isOpen && !billData) {
      setIsLoading(true)
      try {
        const res = await fetchPrintBill(orderUuid)
        setBillData(res)
      } catch (err) {
        toast.error(
          err instanceof Error ? err.message : 'Failed to generate print bill'
        )
      } finally {
        setIsLoading(false)
      }
    }
  }

  const handlePrint = (url?: string) => {
    if (!url) {
      toast.error('Print URL is not available')
      return
    }
    window.open(url, '_blank', 'noopener,noreferrer')
  }

  return (
    <Dialog open={open} onOpenChange={handleOpen}>
      <DialogTrigger asChild>
        <Button size='sm' variant='outline' className='h-8 gap-1.5 text-xs'>
          <Printer className='size-3.5' /> Print Bill
        </Button>
      </DialogTrigger>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Printer className='size-4 text-primary' /> Print Shipping Bill
          </DialogTitle>
          <DialogDescription>
            Generate and print carrier shipping labels for tracking code{' '}
            <span className='font-mono font-semibold text-foreground'>
              {shippingCode}
            </span>
          </DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className='flex flex-col items-center justify-center py-8 gap-2 text-muted-foreground'>
            <Loader2 className='size-6 animate-spin text-primary' />
            <p className='text-xs'>Generating carrier print URLs...</p>
          </div>
        ) : billData ? (
          <div className='space-y-3 py-2'>
            <div className='grid grid-cols-1 gap-2.5'>
              {billData.print_a5 && (
                <div className='flex items-center justify-between p-3 rounded-lg border bg-card hover:bg-muted/50 transition-colors'>
                  <div className='space-y-0.5'>
                    <div className='text-xs font-semibold flex items-center gap-1.5'>
                      <FileText className='size-3.5 text-primary' /> Standard A5 Format
                    </div>
                    <p className='text-[11px] text-muted-foreground'>
                      Recommended for standard laser & inkjet printers (A5 / A4)
                    </p>
                  </div>
                  <Button
                    size='sm'
                    className='h-8 gap-1 text-xs shrink-0'
                    onClick={() => handlePrint(billData.print_a5)}
                  >
                    Print A5 <ExternalLink className='size-3' />
                  </Button>
                </div>
              )}

              {billData.print_80x80 && (
                <div className='flex items-center justify-between p-3 rounded-lg border bg-card hover:bg-muted/50 transition-colors'>
                  <div className='space-y-0.5'>
                    <div className='text-xs font-semibold flex items-center gap-1.5'>
                      <FileText className='size-3.5 text-primary' /> Thermal 80x80 mm
                    </div>
                    <p className='text-[11px] text-muted-foreground'>
                      Square label for 80mm thermal receipt & barcode printers
                    </p>
                  </div>
                  <Button
                    size='sm'
                    variant='outline'
                    className='h-8 gap-1 text-xs shrink-0'
                    onClick={() => handlePrint(billData.print_80x80)}
                  >
                    Print 80x80 <ExternalLink className='size-3' />
                  </Button>
                </div>
              )}

              {billData.print_52x70 && (
                <div className='flex items-center justify-between p-3 rounded-lg border bg-card hover:bg-muted/50 transition-colors'>
                  <div className='space-y-0.5'>
                    <div className='text-xs font-semibold flex items-center gap-1.5'>
                      <FileText className='size-3.5 text-primary' /> Compact 52x70 mm
                    </div>
                    <p className='text-[11px] text-muted-foreground'>
                      Mini barcode label for compact shipping stickers
                    </p>
                  </div>
                  <Button
                    size='sm'
                    variant='outline'
                    className='h-8 gap-1 text-xs shrink-0'
                    onClick={() => handlePrint(billData.print_52x70)}
                  >
                    Print 52x70 <ExternalLink className='size-3' />
                  </Button>
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className='py-6 text-center text-xs text-muted-foreground'>
            No print bill format available. Please check the carrier status.
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
