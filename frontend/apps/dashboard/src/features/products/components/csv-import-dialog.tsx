import { useState, useRef } from 'react'
import {
  AlertCircle,
  CheckCircle2,
  Download,
  FileSpreadsheet,
  Info,
  Loader2,
  Upload,
} from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { useCategories } from '@/features/categories/hooks/use-categories'
import { useImportProductsCSV } from '../hooks/use-products'
import type { ProductCSVImportResponse } from '../types'

interface CSVImportDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CSVImportDialog({ open, onOpenChange }: CSVImportDialogProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [importResult, setImportResult] = useState<ProductCSVImportResponse | null>(null)
  const [showCategoryReference, setShowCategoryReference] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data: categoriesData, isLoading: isLoadingCategories } = useCategories({ limit: 100 })
  const categories = (categoriesData?.data as Array<{ category_id?: number; category_name?: string }>) || []

  const importMutation = useImportProductsCSV()

  const handleCopyCategoryId = (id?: number) => {
    if (id === undefined) return
    navigator.clipboard.writeText(String(id))
    toast.success(`Copied category ID ${id} to clipboard`)
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    if (!file.name.toLowerCase().endsWith('.csv')) {
      toast.error('Only .csv files are supported')
      return
    }

    if (file.size > 5 * 1024 * 1024) {
      toast.error('File size exceeds the 5MB limit')
      return
    }

    setSelectedFile(file)
    setImportResult(null)
  }

  const handleDownloadTemplate = () => {
    const firstCatId = categories[0]?.category_id ?? 1
    const secondCatId = categories[1]?.category_id ?? firstCatId
    const header = 'name,price,description,imageUrl,category_id,stock,sku\n'
    const sampleRow1 = `"Sample T-Shirt, 100% Cotton",19.99,"Soft, breathable casual tee",https://images.unsplash.com/photo-1521572267360-ee0c2909d518,${firstCatId},50,TSHIRT-001\n`
    const sampleRow2 = `"Wireless Gaming Mouse",49.50,"Ergonomic wireless mouse with RGB",https://images.unsplash.com/photo-1527864550417-7fd91fc51a46,${secondCatId},25,MOUSE-002\n`
    const csvContent = header + sampleRow1 + sampleRow2

    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', 'products_import_template.csv')
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  const handleSubmit = async () => {
    if (!selectedFile) {
      toast.error('Please select a CSV file first')
      return
    }

    try {
      const res = await importMutation.mutateAsync(selectedFile)
      setImportResult(res)

      if (res.error_count === 0 && res.success_count > 0) {
        toast.success(`Successfully imported ${res.success_count} products`)
      } else {
        toast.error(`Import failed with ${res.error_count} validation error(s)`)
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to import CSV'
      toast.error(msg)
    }
  }

  const handleClose = () => {
    setSelectedFile(null)
    setImportResult(null)
    setShowCategoryReference(false)
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-[650px]'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <FileSpreadsheet className='h-5 w-5 text-primary' />
            Import Products from CSV
          </DialogTitle>
          <DialogDescription>
            Upload a CSV file to batch import products. Supports RFC 4180 quotes with commas and atomic all-or-nothing transactions.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-2'>
          {/* Action Row: Download Template & Category Reference */}
          <div className='flex flex-wrap items-center justify-between gap-2 rounded-lg border bg-muted/30 p-3'>
            <div className='space-y-0.5 text-xs text-muted-foreground'>
              <p className='font-semibold text-foreground'>Required Headers:</p>
              <code className='rounded bg-muted px-1 py-0.5 text-[11px] font-mono'>
                name,price,description,imageUrl,category_id,stock,sku
              </code>
            </div>
            <div className='flex items-center gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => setShowCategoryReference(!showCategoryReference)}
                className='h-7 gap-1 text-xs'
              >
                <Info className='h-3.5 w-3.5' />
                <span>Categories</span>
              </Button>
              <Button
                type='button'
                variant='secondary'
                size='sm'
                onClick={handleDownloadTemplate}
                className='h-7 gap-1 text-xs'
              >
                <Download className='h-3.5 w-3.5' />
                <span>Template</span>
              </Button>
            </div>
          </div>

          {/* Collapsible Category ID Reference */}
          {showCategoryReference && (
            <div className='rounded-lg border bg-card p-3'>
              <div className='mb-2 flex items-center justify-between'>
                <span className='text-xs font-semibold text-foreground'>Category ID Reference List</span>
                <span className='text-[10px] text-muted-foreground'>Click any item to copy its ID</span>
              </div>
              {isLoadingCategories ? (
                <div className='py-4 text-center text-xs text-muted-foreground'>
                  Loading categories...
                </div>
              ) : categories.length === 0 ? (
                <div className='py-4 text-center text-xs text-muted-foreground'>
                  No categories found.
                </div>
              ) : (
                <div className='max-h-36 overflow-y-auto pr-1'>
                  <div className='grid grid-cols-2 gap-1.5 sm:grid-cols-3'>
                    {categories.map((c) => (
                      <button
                        type='button'
                        key={c.category_id ?? c.category_name}
                        onClick={() => handleCopyCategoryId(c.category_id)}
                        className='group flex items-center justify-between rounded border bg-muted/20 px-2 py-1 text-xs text-left transition-colors hover:bg-primary/10 hover:border-primary/40 cursor-pointer'
                        title={`Click to copy category_id ${c.category_id}`}
                      >
                        <span className='truncate pr-1 text-foreground'>{c.category_name}</span>
                        <Badge variant='outline' className='font-mono text-[10px] px-1 py-0 shrink-0 group-hover:bg-primary group-hover:text-primary-foreground'>
                          ID: {c.category_id ?? 'N/A'}
                        </Badge>
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* File Picker / Dropzone */}
          <div
            onClick={() => fileInputRef.current?.click()}
            className='flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed border-muted-foreground/25 p-6 transition-colors hover:border-primary/50 hover:bg-muted/20'
          >
            <input
              ref={fileInputRef}
              type='file'
              accept='.csv'
              className='hidden'
              onChange={handleFileChange}
            />
            <Upload className='h-8 w-8 text-muted-foreground' />
            <p className='mt-2 text-sm font-semibold text-foreground'>
              {selectedFile ? selectedFile.name : 'Click to select CSV file'}
            </p>
            <p className='mt-1 text-xs text-muted-foreground'>
              {selectedFile
                ? `${(selectedFile.size / 1024).toFixed(1)} KB`
                : 'Accepts .csv up to 5MB'}
            </p>
          </div>

          {/* Import Result Feedback */}
          {importResult && (
            <div className='space-y-3 pt-1'>
              {importResult.success_count > 0 && importResult.error_count === 0 ? (
                <div className='flex items-center gap-3 rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-4 text-emerald-600 dark:text-emerald-400'>
                  <CheckCircle2 className='h-5 w-5 shrink-0' />
                  <div>
                    <p className='text-sm font-bold'>Import Successful</p>
                    <p className='text-xs'>
                      All {importResult.success_count} products were validated and inserted into the catalog.
                    </p>
                  </div>
                </div>
              ) : (
                <div className='space-y-3'>
                  <div className='flex items-start gap-3 rounded-xl border border-rose-500/30 bg-rose-500/10 p-4 text-rose-600 dark:text-rose-400'>
                    <AlertCircle className='h-5 w-5 shrink-0 mt-0.5' />
                    <div>
                      <p className='text-sm font-bold'>Import Failed (All-or-Nothing)</p>
                      <p className='text-xs'>
                        Found {importResult.error_count} error(s). No records were saved to the database. Fix the errors below and try again.
                      </p>
                    </div>
                  </div>

                  {/* Errors Table */}
                  <div className='max-h-56 overflow-y-auto rounded-lg border'>
                    <table className='w-full text-left text-xs'>
                      <thead className='sticky top-0 bg-muted border-b text-muted-foreground'>
                        <tr>
                          <th className='p-2 w-16'>Row</th>
                          <th className='p-2 w-28'>Field</th>
                          <th className='p-2'>Reason</th>
                        </tr>
                      </thead>
                      <tbody className='divide-y divide-border'>
                        {importResult.errors.map((err, i) => (
                          <tr key={i} className='hover:bg-muted/30'>
                            <td className='p-2 font-mono font-bold text-foreground'>
                              #{err.row}
                            </td>
                            <td className='p-2'>
                              <Badge variant='outline' className='text-[10px] font-mono'>
                                {err.field || 'General'}
                              </Badge>
                            </td>
                            <td className='p-2 text-destructive font-medium'>
                              {err.reason}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        <DialogFooter className='gap-2 sm:gap-0'>
          <Button type='button' variant='outline' onClick={handleClose}>
            {importResult?.success_count ? 'Done' : 'Cancel'}
          </Button>
          {(!importResult || importResult.error_count > 0) && (
            <Button
              type='button'
              onClick={handleSubmit}
              disabled={!selectedFile || importMutation.isPending}
            >
              {importMutation.isPending && (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              )}
              Upload & Import
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
