import { useState, useRef } from 'react'
import { UploadCloud, X, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { getImageUrl } from '@/lib/utils'
import { useUploadImage } from '../hooks/use-products'

interface ImageUploadProps {
  value?: string
  onChange: (url: string) => void
  folder?: string
}

export function ImageUpload({ value, onChange, folder = 'products' }: ImageUploadProps) {
  const [isDragging, setIsDragging] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const uploadMutation = useUploadImage()

  const handleFile = async (file: File) => {
    if (!file.type.startsWith('image/')) {
      toast.error('Please select an image file (PNG, JPG, WEBP, etc.)')
      return
    }

    if (file.size > 5 * 1024 * 1024) {
      toast.error('Image size must be less than 5MB')
      return
    }

    try {
      const data = await uploadMutation.mutateAsync({ file, folder })
      onChange(data.url)
      toast.success('Image uploaded successfully!')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to upload image'
      toast.error(msg)
    }
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    if (e.dataTransfer.files?.[0]) {
      handleFile(e.dataTransfer.files[0])
    }
  }

  return (
    <div className='w-full'>
      {value ? (
        <div className='relative inline-block border rounded-lg overflow-hidden bg-muted/20 group'>
          <img
            src={getImageUrl(value)}
            alt='Product'
            className='size-36 object-cover'
            onError={(e) => {
              ;(e.target as HTMLImageElement).src =
                'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect fill="%23eee" width="100" height="100"/><text fill="%23aaa" x="50%" y="50%" text-anchor="middle" dominant-baseline="middle">No Image</text></svg>'
            }}
          />
          <Button
            type='button'
            variant='destructive'
            size='icon'
            className='absolute top-1 right-1 size-6 opacity-80 group-hover:opacity-100 transition-opacity'
            onClick={() => onChange('')}
          >
            <X className='size-3' />
          </Button>
        </div>
      ) : (
        <div
          onDragOver={(e) => {
            e.preventDefault()
            setIsDragging(true)
          }}
          onDragLeave={() => setIsDragging(false)}
          onDrop={handleDrop}
          onClick={() => fileInputRef.current?.click()}
          className={`flex flex-col items-center justify-center gap-2 p-6 rounded-lg border-2 border-dashed cursor-pointer transition-colors ${
            isDragging
              ? 'border-primary bg-primary/5'
              : 'border-muted-foreground/25 hover:border-primary/50'
          }`}
        >
          {uploadMutation.isPending ? (
            <Loader2 className='size-8 animate-spin text-primary' />
          ) : (
            <UploadCloud className='size-8 text-muted-foreground' />
          )}
          <div className='text-center'>
            <p className='text-sm font-medium'>
              {uploadMutation.isPending ? 'Uploading image...' : 'Drop image here or click to browse'}
            </p>
            <p className='text-xs text-muted-foreground'>PNG, JPG, WEBP up to 5MB</p>
          </div>
        </div>
      )}

      <input
        ref={fileInputRef}
        type='file'
        accept='image/*'
        className='hidden'
        onChange={(e) => {
          if (e.target.files?.[0]) {
            handleFile(e.target.files[0])
          }
        }}
      />
    </div>
  )
}
