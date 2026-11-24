import { useState } from 'react'
import { Plus, Trash2, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'

interface SpecificationsEditorProps {
  value?: Record<string, string>
  onChange: (value: Record<string, string>) => void
}

const COMMON_SPEC_SUGGESTIONS = [
  'CPU',
  'RAM',
  'Storage',
  'Screen',
  'Battery',
  'Camera',
  'GPU',
  'OS',
  'Weight',
  'Color',
  'Connectivity',
  'Warranty',
]

interface SpecEntry {
  id: string
  key: string
  val: string
}

export function SpecificationsEditor({ value = {}, onChange }: SpecificationsEditorProps) {
  // Convert dict to list of entries for local state
  const [entries, setEntries] = useState<SpecEntry[]>(() => {
    return Object.entries(value || {}).map(([k, v], idx) => ({
      id: `${k}-${idx}-${Date.now()}`,
      key: k,
      val: v,
    }))
  })

  const syncToParent = (updatedEntries: SpecEntry[]) => {
    setEntries(updatedEntries)
    const result: Record<string, string> = {}
    for (const item of updatedEntries) {
      const cleanKey = item.key.trim()
      if (cleanKey) {
        result[cleanKey] = item.val.trim()
      }
    }
    onChange(result)
  }

  const handleAddRow = (initialKey = '') => {
    const newEntry: SpecEntry = {
      id: `spec-${Date.now()}-${Math.random().toString(36).substring(2, 7)}`,
      key: initialKey,
      val: '',
    }
    syncToParent([...entries, newEntry])
  }

  const handleRemoveRow = (id: string) => {
    syncToParent(entries.filter((e) => e.id !== id))
  }

  const handleKeyChange = (id: string, newKey: string) => {
    syncToParent(
      entries.map((e) => (e.id === id ? { ...e, key: newKey } : e))
    )
  }

  const handleValChange = (id: string, newVal: string) => {
    syncToParent(
      entries.map((e) => (e.id === id ? { ...e, val: newVal } : e))
    )
  }

  const existingKeys = new Set(entries.map((e) => e.key.toLowerCase().trim()))

  return (
    <div className='space-y-4'>
      {/* Quick Suggestions */}
      <div className='space-y-1.5'>
        <div className='flex items-center gap-1.5 text-xs text-muted-foreground'>
          <Sparkles className='size-3 text-amber-500' />
          <span>Quick Suggestions:</span>
        </div>
        <div className='flex flex-wrap gap-1.5'>
          {COMMON_SPEC_SUGGESTIONS.map((sug) => {
            const isAdded = existingKeys.has(sug.toLowerCase())
            return (
              <Badge
                key={sug}
                variant={isAdded ? 'secondary' : 'outline'}
                className={`cursor-pointer text-xs transition-colors ${
                  isAdded
                    ? 'opacity-50 cursor-not-allowed'
                    : 'hover:bg-primary hover:text-primary-foreground'
                }`}
                onClick={() => {
                  if (!isAdded) handleAddRow(sug)
                }}
              >
                + {sug}
              </Badge>
            )
          })}
        </div>
      </div>

      {/* Entries List */}
      <div className='space-y-2'>
        {entries.length === 0 ? (
          <div className='rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground'>
            No specifications added yet. Click a suggestion above or add a custom row below.
          </div>
        ) : (
          entries.map((entry) => (
            <div key={entry.id} className='flex items-center gap-2'>
              <Input
                placeholder='Specification (e.g. RAM)'
                value={entry.key}
                onChange={(e) => handleKeyChange(entry.id, e.target.value)}
                className='w-1/3 text-xs'
              />
              <Input
                placeholder='Value (e.g. 16GB LPDDR5)'
                value={entry.val}
                onChange={(e) => handleValChange(entry.id, e.target.value)}
                className='flex-1 text-xs'
              />
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='size-8 text-destructive hover:bg-destructive/10'
                onClick={() => handleRemoveRow(entry.id)}
                title='Remove specification'
              >
                <Trash2 className='size-4' />
              </Button>
            </div>
          ))
        )}
      </div>

      <Button
        type='button'
        variant='outline'
        size='sm'
        className='gap-1.5 text-xs'
        onClick={() => handleAddRow()}
      >
        <Plus className='size-3.5' /> Add Specification
      </Button>
    </div>
  )
}
