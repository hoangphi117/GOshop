import { useState } from 'react'
import { Truck, Calculator, Loader2, Package, MapPin } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  useProvinces,
  useDistricts,
  useWards,
  useCalculateShippingFee,
  useShippingMutations,
} from '../hooks/use-shipping'
import { formatCurrency } from '../order-utils'

interface CreateShippingFormProps {
  orderUuid: string
  shippingAddress: string
  totalAmount?: number
  onSuccess?: () => void
}

export function CreateShippingForm({
  orderUuid,
  shippingAddress,
  totalAmount,
  onSuccess,
}: CreateShippingFormProps) {
  const [provider, setProvider] = useState<'ghn' | 'manual'>('ghn')
  const [selectedProvinceId, setSelectedProvinceId] = useState<number | null>(
    null
  )
  const [selectedDistrictId, setSelectedDistrictId] = useState<number | null>(
    null
  )
  const [selectedWardCode, setSelectedWardCode] = useState<string>('')
  const [weight, setWeight] = useState<number>(500)
  const [length, setLength] = useState<number>(20)
  const [width, setWidth] = useState<number>(15)
  const [height, setHeight] = useState<number>(10)
  const [requiredNote, setRequiredNote] = useState<
    'KHONGCHOXEMHANG' | 'CHOXEMHANGKHONGTHU' | 'CHOTHUHANG'
  >('KHONGCHOXEMHANG')
  const [note, setNote] = useState<string>('')
  const [estimatedFee, setEstimatedFee] = useState<number | null>(null)

  const { data: provinces = [], isLoading: isLoadingProvinces } = useProvinces()
  const { data: districts = [], isLoading: isLoadingDistricts } =
    useDistricts(selectedProvinceId)
  const { data: wards = [], isLoading: isLoadingWards } =
    useWards(selectedDistrictId)

  const calculateFeeMutation = useCalculateShippingFee()
  const { createShipping } = useShippingMutations()

  const handleProvinceChange = (val: string) => {
    const pId = Number(val)
    setSelectedProvinceId(pId)
    setSelectedDistrictId(null)
    setSelectedWardCode('')
    setEstimatedFee(null)
  }

  const handleDistrictChange = (val: string) => {
    const dId = Number(val)
    setSelectedDistrictId(dId)
    setSelectedWardCode('')
    setEstimatedFee(null)
  }

  const handleCalculateFee = async () => {
    if (!selectedDistrictId || !selectedWardCode) {
      toast.error('Please select both District and Ward to calculate fee')
      return
    }

    try {
      const res = await calculateFeeMutation.mutateAsync({
        provider,
        district_id: selectedDistrictId,
        ward_code: selectedWardCode,
        weight,
        length,
        width,
        height,
        insurance_value: totalAmount,
      })
      if (
        res &&
        typeof (res as { shipping_fee?: number }).shipping_fee === 'number'
      ) {
        setEstimatedFee((res as { shipping_fee: number }).shipping_fee)
        toast.success(
          `Estimated shipping fee: ${formatCurrency((res as { shipping_fee: number }).shipping_fee)}`
        )
      }
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to calculate shipping fee'
      )
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (provider === 'ghn' && (!selectedDistrictId || !selectedWardCode)) {
      toast.error('Please select delivery District and Ward for GHN shipment')
      return
    }

    if (weight <= 0) {
      toast.error('Parcel weight must be greater than 0g')
      return
    }

    try {
      await createShipping.mutateAsync({
        uuid: orderUuid,
        input: {
          provider,
          district_id: selectedDistrictId || 0,
          ward_code: selectedWardCode || '',
          weight,
          length,
          width,
          height,
          required_note: requiredNote,
          note: note.trim() || undefined,
        },
      })
      toast.success('Shipping order created successfully with carrier!')
      onSuccess?.()
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to create shipping order'
      )
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className='space-y-4 rounded-lg border bg-card p-4'
    >
      <div className='flex items-center justify-between border-b pb-3'>
        <div className='flex items-center gap-2'>
          <Truck className='size-4 text-primary' />
          <div>
            <h3 className='text-xs font-semibold tracking-wider text-foreground uppercase'>
              Create Shipping Order
            </h3>
            <p className='text-[11px] text-muted-foreground'>
              Dispatch parcel to delivery carrier (GHN Giao Hàng Nhanh / Manual)
            </p>
          </div>
        </div>
      </div>

      {/* Delivery Destination Reference */}
      <div className='flex items-start gap-2 rounded-md border bg-muted/40 p-2.5 text-xs'>
        <MapPin className='mt-0.5 size-3.5 shrink-0 text-muted-foreground' />
        <div className='space-y-0.5'>
          <span className='text-[11px] font-semibold text-muted-foreground uppercase'>
            Customer Delivery Address
          </span>
          <p className='text-foreground'>{shippingAddress || 'Not provided'}</p>
        </div>
      </div>

      <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
        {/* Carrier Selection */}
        <div className='space-y-1.5'>
          <label className='text-xs font-semibold text-muted-foreground'>
            Shipping Carrier
          </label>
          <Select
            value={provider}
            onValueChange={(val: 'ghn' | 'manual') => setProvider(val)}
          >
            <SelectTrigger className='h-9 text-xs'>
              <SelectValue placeholder='Select shipping carrier' />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='ghn' className='text-xs'>
                Giao Hàng Nhanh (GHN) — Automated API Integration
              </SelectItem>
              <SelectItem value='manual' className='text-xs'>
                Manual Delivery / In-House Courier
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* GHN Required Note */}
        <div className='space-y-1.5'>
          <label className='text-xs font-semibold text-muted-foreground'>
            Package Inspection Policy
          </label>
          <Select
            value={requiredNote}
            onValueChange={(
              val: 'KHONGCHOXEMHANG' | 'CHOXEMHANGKHONGTHU' | 'CHOTHUHANG'
            ) => setRequiredNote(val)}
          >
            <SelectTrigger className='h-9 text-xs'>
              <SelectValue placeholder='Select inspection rule' />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='KHONGCHOXEMHANG' className='text-xs'>
                Do Not Inspect
              </SelectItem>
              <SelectItem value='CHOXEMHANGKHONGTHU' className='text-xs'>
                Inspect Without Trial
              </SelectItem>
              <SelectItem value='CHOTHUHANG' className='text-xs'>
                Allow Trial
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Cascading GHN Location Selection */}
      <div className='grid grid-cols-1 gap-3 rounded-lg border bg-muted/20 p-3 sm:grid-cols-3'>
        <div className='space-y-1.5'>
          <label className='text-[11px] font-semibold text-muted-foreground'>
            1. Province / City
          </label>
          <Select
            value={selectedProvinceId ? String(selectedProvinceId) : ''}
            onValueChange={handleProvinceChange}
            disabled={isLoadingProvinces}
          >
            <SelectTrigger className='h-8 text-xs'>
              <SelectValue
                placeholder={
                  isLoadingProvinces ? 'Loading...' : 'Select Province'
                }
              />
            </SelectTrigger>
            <SelectContent className='max-h-56'>
              {provinces.map((p) => (
                <SelectItem
                  key={p.province_id}
                  value={String(p.province_id)}
                  className='text-xs'
                >
                  {p.province_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className='space-y-1.5'>
          <label className='text-[11px] font-semibold text-muted-foreground'>
            2. District
          </label>
          <Select
            value={selectedDistrictId ? String(selectedDistrictId) : ''}
            onValueChange={handleDistrictChange}
            disabled={!selectedProvinceId || isLoadingDistricts}
          >
            <SelectTrigger className='h-8 text-xs'>
              <SelectValue
                placeholder={
                  !selectedProvinceId
                    ? 'Select province first'
                    : isLoadingDistricts
                      ? 'Loading...'
                      : 'Select District'
                }
              />
            </SelectTrigger>
            <SelectContent className='max-h-56'>
              {districts.map((d) => (
                <SelectItem
                  key={d.district_id}
                  value={String(d.district_id)}
                  className='text-xs'
                >
                  {d.district_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className='space-y-1.5'>
          <label className='text-[11px] font-semibold text-muted-foreground'>
            3. Ward / Commune
          </label>
          <Select
            value={selectedWardCode}
            onValueChange={(val) => {
              setSelectedWardCode(val)
              setEstimatedFee(null)
            }}
            disabled={!selectedDistrictId || isLoadingWards}
          >
            <SelectTrigger className='h-8 text-xs'>
              <SelectValue
                placeholder={
                  !selectedDistrictId
                    ? 'Select district first'
                    : isLoadingWards
                      ? 'Loading...'
                      : 'Select Ward'
                }
              />
            </SelectTrigger>
            <SelectContent className='max-h-56'>
              {wards.map((w) => (
                <SelectItem
                  key={w.ward_code}
                  value={w.ward_code}
                  className='text-xs'
                >
                  {w.ward_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Package Dimensions & Weight */}
      <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
        <div className='space-y-1.5'>
          <label className='flex items-center gap-1 text-[11px] font-semibold text-muted-foreground'>
            <Package className='size-3' /> Weight (grams)
          </label>
          <Input
            type='number'
            min={1}
            value={weight}
            onChange={(e) => setWeight(Number(e.target.value))}
            className='h-8 text-xs'
            required
          />
        </div>
        <div className='space-y-1.5'>
          <label className='text-[11px] font-semibold text-muted-foreground'>
            Length (cm)
          </label>
          <Input
            type='number'
            min={1}
            value={length}
            onChange={(e) => setLength(Number(e.target.value))}
            className='h-8 text-xs'
            required
          />
        </div>
        <div className='space-y-1.5'>
          <label className='text-[11px] font-semibold text-muted-foreground'>
            Width (cm)
          </label>
          <Input
            type='number'
            min={1}
            value={width}
            onChange={(e) => setWidth(Number(e.target.value))}
            className='h-8 text-xs'
            required
          />
        </div>
        <div className='space-y-1.5'>
          <label className='text-[11px] font-semibold text-muted-foreground'>
            Height (cm)
          </label>
          <Input
            type='number'
            min={1}
            value={height}
            onChange={(e) => setHeight(Number(e.target.value))}
            className='h-8 text-xs'
            required
          />
        </div>
      </div>

      {/* Note */}
      <div className='space-y-1.5'>
        <label className='text-xs font-semibold text-muted-foreground'>
          Carrier Note / Delivery Instructions
        </label>
        <Textarea
          placeholder='e.g., Handle with care, fragile electronic goods, call before delivery...'
          value={note}
          onChange={(e) => setNote(e.target.value)}
          className='h-16 resize-none text-xs'
          maxLength={500}
        />
      </div>

      {/* Actions & Fee preview */}
      <div className='flex flex-wrap items-center justify-between gap-3 border-t pt-2'>
        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='h-8 gap-1.5 text-xs'
            onClick={handleCalculateFee}
            disabled={
              calculateFeeMutation.isPending ||
              !selectedDistrictId ||
              !selectedWardCode
            }
          >
            {calculateFeeMutation.isPending ? (
              <Loader2 className='size-3 animate-spin' />
            ) : (
              <Calculator className='size-3' />
            )}
            Estimate Fee
          </Button>

          {estimatedFee !== null && (
            <span className='text-xs font-semibold text-emerald-600 dark:text-emerald-400'>
              Fee: {formatCurrency(estimatedFee)}
            </span>
          )}
        </div>

        <Button
          type='submit'
          size='sm'
          className='h-8 gap-1.5 text-xs'
          disabled={createShipping.isPending}
        >
          {createShipping.isPending ? (
            <>
              <Loader2 className='size-3.5 animate-spin' /> Dispatching...
            </>
          ) : (
            <>
              <Truck className='size-3.5' /> Dispatch GHN Shipment
            </>
          )}
        </Button>
      </div>
    </form>
  )
}
