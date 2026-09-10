'use client'

import React, { useState, useEffect, useCallback } from 'react'
import { useAuthStore } from '@/stores/auth-store'
import { authFetch } from '@/lib/api'
import { extractErrorMessage } from '@/lib/utils'
import { MapPin, Plus, Trash2, CheckCircle2, Home, Building2, Tag, Loader2, X } from 'lucide-react'
import { toast } from 'sonner'

interface Province {
  province_id: number
  province_name: string
  code: string
}

interface District {
  district_id: number
  province_id: number
  district_name: string
  code: string
}

interface Ward {
  ward_code: string
  district_id: number
  ward_name: string
}

interface UserAddress {
  uuid: string
  label: string
  name: string
  phone: string
  address: string
  province_id?: number
  province_name?: string
  district_id?: number
  district_name?: string
  ward_code?: string
  ward_name?: string
  full_address: string
  is_default?: boolean
}

export default function AddressesPage() {
  const { accessToken, user } = useAuthStore()
  const [addresses, setAddresses] = useState<UserAddress[]>([])
  const [loading, setLoading] = useState(true)
  const [showAddForm, setShowAddForm] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [deletingUuid, setDeletingUuid] = useState<string | null>(null)

  // Form State
  const [label, setLabel] = useState('Home')
  const [name, setName] = useState(user?.fullName || '')
  const [phone, setPhone] = useState('')
  const [address, setAddress] = useState('')
  const [isDefault, setIsDefault] = useState(false)

  // GHN Cascading Address State
  const [provinces, setProvinces] = useState<Province[]>([])
  const [districts, setDistricts] = useState<District[]>([])
  const [wards, setWards] = useState<Ward[]>([])

  const [provinceId, setProvinceId] = useState<number | null>(null)
  const [provinceName, setProvinceName] = useState('')
  const [districtId, setDistrictId] = useState<number | null>(null)
  const [districtName, setDistrictName] = useState('')
  const [wardCode, setWardCode] = useState('')
  const [wardName, setWardName] = useState('')

  const [isLoadingProvinces, setIsLoadingProvinces] = useState(false)
  const [isLoadingDistricts, setIsLoadingDistricts] = useState(false)
  const [isLoadingWards, setIsLoadingWards] = useState(false)

  const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const apiKey = process.env.NEXT_PUBLIC_API_KEY || 'abcd-1234-1111'

  const fetchAddresses = useCallback(async () => {
    if (!accessToken) return
    try {
      const res = await authFetch(`${baseUrl}/api/v1/addresses`)
      if (res.ok) {
        const json = await res.json()
        if (json?.data && Array.isArray(json.data)) {
          setAddresses(json.data)
        } else {
          setAddresses([])
        }
      }
    } catch {
      setAddresses([])
    } finally {
      setLoading(false)
    }
  }, [accessToken, baseUrl])

  useEffect(() => {
    fetchAddresses()
  }, [fetchAddresses])

  // Load provinces on mount
  useEffect(() => {
    setIsLoadingProvinces(true)
    fetch(`${baseUrl}/api/v1/shipping/provinces`, {
      headers: { 'x-api-key': apiKey },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.data && Array.isArray(data.data)) {
          const sorted = [...data.data].sort((a: Province, b: Province) =>
            a.province_name.localeCompare(b.province_name)
          )
          setProvinces(sorted)
        }
      })
      .catch(() => {})
      .finally(() => setIsLoadingProvinces(false))
  }, [baseUrl, apiKey])

  const loadDistricts = async (pId: number) => {
    setIsLoadingDistricts(true)
    setDistricts([])
    setWards([])
    setDistrictId(null)
    setDistrictName('')
    setWardCode('')
    setWardName('')

    try {
      const res = await fetch(`${baseUrl}/api/v1/shipping/districts?province_id=${pId}`, {
        headers: { 'x-api-key': apiKey },
      })
      if (res.ok) {
        const json = await res.json()
        if (json?.data && Array.isArray(json.data)) {
          const sorted = [...json.data].sort((a: District, b: District) =>
            a.district_name.localeCompare(b.district_name)
          )
          setDistricts(sorted)
        }
      }
    } catch {
      toast.error('Failed to load districts.')
    } finally {
      setIsLoadingDistricts(false)
    }
  }

  const loadWards = async (dId: number) => {
    setIsLoadingWards(true)
    setWards([])
    setWardCode('')
    setWardName('')

    try {
      const res = await fetch(`${baseUrl}/api/v1/shipping/wards?district_id=${dId}`, {
        headers: { 'x-api-key': apiKey },
      })
      if (res.ok) {
        const json = await res.json()
        if (json?.data && Array.isArray(json.data)) {
          const sorted = [...json.data].sort((a: Ward, b: Ward) =>
            a.ward_name.localeCompare(b.ward_name)
          )
          setWards(sorted)
        }
      }
    } catch {
      toast.error('Failed to load wards.')
    } finally {
      setIsLoadingWards(false)
    }
  }

  const handleProvinceChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const id = Number(e.target.value)
    if (!id) {
      setProvinceId(null)
      setProvinceName('')
      setDistricts([])
      setWards([])
      setDistrictId(null)
      setDistrictName('')
      setWardCode('')
      setWardName('')
      return
    }
    const found = provinces.find((p) => p.province_id === id)
    setProvinceId(id)
    setProvinceName(found?.province_name || '')
    loadDistricts(id)
  }

  const handleDistrictChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const id = Number(e.target.value)
    if (!id) {
      setDistrictId(null)
      setDistrictName('')
      setWards([])
      setWardCode('')
      setWardName('')
      return
    }
    const found = districts.find((d) => d.district_id === id)
    setDistrictId(id)
    setDistrictName(found?.district_name || '')
    loadWards(id)
  }

  const handleWardChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const code = e.target.value
    if (!code) {
      setWardCode('')
      setWardName('')
      return
    }
    const found = wards.find((w) => String(w.ward_code) === String(code))
    setWardCode(code)
    setWardName(found?.ward_name || '')
  }

  const handleCreateAddress = async (e: React.FormEvent) => {
    e.preventDefault()

    const trimmedName = name.trim()
    if (trimmedName.length < 2) {
      toast.error('Recipient Name must be at least 2 characters.')
      return
    }

    const cleanPhone = phone.trim().replace(/[\s.-]+/g, '')
    if (cleanPhone.length < 9) {
      toast.error('Phone number must be at least 9 digits.')
      return
    }

    const trimmedAddress = address.trim()
    if (trimmedAddress.length < 5) {
      toast.error('Street address must be at least 5 characters (e.g. house number, street name).')
      return
    }

    const finalProvName =
      provinceName || provinces.find((p) => p.province_id === provinceId)?.province_name || ''
    if (!provinceId || !finalProvName) {
      toast.error('Please select a City / Province.')
      return
    }

    const finalDistName =
      districtName || districts.find((d) => d.district_id === districtId)?.district_name || ''
    if (!districtId || !finalDistName) {
      toast.error('Please select a District.')
      return
    }

    const finalWardName =
      wardName || wards.find((w) => String(w.ward_code) === String(wardCode))?.ward_name || ''
    if (!wardCode || !finalWardName) {
      toast.error('Please select a Ward.')
      return
    }

    setSubmitting(true)

    const payload = {
      label: label || 'Home',
      name: trimmedName,
      phone: cleanPhone,
      address: trimmedAddress,
      province_id: Number(provinceId),
      province_name: finalProvName,
      district_id: Number(districtId),
      district_name: finalDistName,
      ward_code: String(wardCode),
      ward_name: finalWardName,
      is_default: isDefault,
    }

    try {
      const res = await authFetch(`${baseUrl}/api/v1/addresses`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      })

      if (!res.ok) {
        const errJson = await res.json().catch(() => null)
        throw new Error(extractErrorMessage(errJson, 'Failed to create address.'))
      }

      toast.success('Address saved successfully!')
      setShowAddForm(false)

      // Reset form
      setName(user?.fullName || '')
      setPhone('')
      setAddress('')
      setLabel('Home')
      setProvinceId(null)
      setProvinceName('')
      setDistrictId(null)
      setDistrictName('')
      setWardCode('')
      setWardName('')
      setDistricts([])
      setWards([])
      setIsDefault(false)

      fetchAddresses()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Error adding address'
      toast.error(msg)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (uuid: string) => {
    if (!window.confirm('Are you sure you want to remove this address?')) return

    setDeletingUuid(uuid)
    try {
      const res = await authFetch(`${baseUrl}/api/v1/addresses/${uuid}`, {
        method: 'DELETE',
      })

      if (!res.ok) {
        const errJson = await res.json().catch(() => null)
        throw new Error(extractErrorMessage(errJson, 'Failed to delete address.'))
      }

      toast.success('Address removed.')
      setAddresses((prev) => prev.filter((a) => a.uuid !== uuid))
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Could not remove address.'
      toast.error(msg)
    } finally {
      setDeletingUuid(null)
    }
  }

  const handleSetDefault = async (uuid: string) => {
    try {
      const res = await authFetch(`${baseUrl}/api/v1/addresses/${uuid}/default`, {
        method: 'PATCH',
      })

      if (!res.ok) {
        const errJson = await res.json().catch(() => null)
        throw new Error(extractErrorMessage(errJson, 'Failed to update default address.'))
      }

      toast.success('Default address updated.')
      fetchAddresses()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Could not set default address.'
      toast.error(msg)
    }
  }

  if (loading) {
    return (
      <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs flex items-center justify-center min-h-[300px]">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    )
  }

  return (
    <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-extrabold text-[#0D1B2A] tracking-tight">
            Shipping Addresses
          </h1>
          <p className="text-xs text-slate-500 mt-1">
            Manage your saved delivery destinations for quick and easy checkout.
          </p>
        </div>

        <button
          onClick={() => setShowAddForm(!showAddForm)}
          className="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-colors shadow-xs cursor-pointer"
        >
          {showAddForm ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
          <span>{showAddForm ? 'Close Form' : 'Add New Address'}</span>
        </button>
      </div>

      {/* Add New Address Form Modal/Panel */}
      {showAddForm && (
        <form
          onSubmit={handleCreateAddress}
          className="p-6 rounded-2xl bg-slate-50 border border-slate-200/80 space-y-5 transition-all"
        >
          <div className="flex items-center justify-between pb-3 border-b border-slate-200/80">
            <h3 className="text-sm font-bold text-[#0D1B2A]">New Address Details</h3>
            <span className="text-[10px] text-slate-400 font-medium">Standard GHN Address</span>
          </div>

          {/* Contact Information */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">
                Recipient Name <span className="text-rose-500">*</span>
              </label>
              <input
                type="text"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Jane Doe"
                className="w-full px-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
              />
            </div>

            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">
                Phone Number <span className="text-rose-500">*</span>
              </label>
              <input
                type="tel"
                required
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                placeholder="e.g. 0901234567"
                className="w-full px-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
              />
            </div>
          </div>

          {/* Address Label */}
          <div className="space-y-1">
            <label className="text-xs font-semibold text-slate-700">Address Label</label>
            <div className="flex items-center gap-2">
              {['Home', 'Office', 'Other'].map((l) => (
                <button
                  key={l}
                  type="button"
                  onClick={() => setLabel(l)}
                  className={`px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all cursor-pointer ${
                    label === l
                      ? 'border-[#2196F3] bg-[#E3F2FD] text-[#0D47A1]'
                      : 'border-slate-200 bg-white text-slate-600 hover:bg-slate-100'
                  }`}
                >
                  {l}
                </button>
              ))}
            </div>
          </div>

          {/* Cascading GHN Selectors */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-2">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">
                City / Province <span className="text-rose-500">*</span>
              </label>
              <select
                required
                value={provinceId || ''}
                onChange={handleProvinceChange}
                disabled={isLoadingProvinces}
                className="w-full px-2.5 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer"
              >
                <option value="">{isLoadingProvinces ? 'Loading...' : '-- Select --'}</option>
                {provinces.map((p) => (
                  <option key={p.province_id} value={p.province_id}>
                    {p.province_name}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">
                District <span className="text-rose-500">*</span>
              </label>
              <select
                required
                value={districtId || ''}
                onChange={handleDistrictChange}
                disabled={!provinceId || isLoadingDistricts}
                className="w-full px-2.5 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer"
              >
                <option value="">
                  {isLoadingDistricts
                    ? 'Loading...'
                    : !provinceId
                    ? 'Select province'
                    : '-- Select --'}
                </option>
                {districts.map((d) => (
                  <option key={d.district_id} value={d.district_id}>
                    {d.district_name}
                  </option>
                ))}
              </select>
            </div>

            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">
                Ward <span className="text-rose-500">*</span>
              </label>
              <select
                required
                value={wardCode || ''}
                onChange={handleWardChange}
                disabled={!districtId || isLoadingWards}
                className="w-full px-2.5 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer"
              >
                <option value="">
                  {isLoadingWards ? 'Loading...' : !districtId ? 'Select district' : '-- Select --'}
                </option>
                {wards.map((w) => (
                  <option key={w.ward_code} value={w.ward_code}>
                    {w.ward_name}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Street Address */}
          <div className="space-y-1">
            <label className="text-xs font-semibold text-slate-700">
              Street Address <span className="text-rose-500">*</span>
            </label>
            <input
              type="text"
              required
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="Apartment, suite, house number, street name"
              className="w-full px-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
            />
          </div>

          {/* Default Address Checkbox */}
          <div className="flex items-center gap-2 pt-1">
            <input
              type="checkbox"
              id="isDefault"
              checked={isDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
              className="rounded text-[#2196F3] focus:ring-[#2196F3] cursor-pointer"
            />
            <label htmlFor="isDefault" className="text-xs text-slate-700 cursor-pointer select-none">
              Set as default shipping address
            </label>
          </div>

          {/* Action Buttons */}
          <div className="flex justify-end gap-2 pt-2 border-t border-slate-200/80">
            <button
              type="button"
              onClick={() => setShowAddForm(false)}
              className="px-4 py-2 rounded-xl border border-slate-200 bg-white text-xs font-semibold text-slate-600 hover:bg-slate-100 cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-5 py-2 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-colors cursor-pointer disabled:opacity-60 flex items-center gap-1.5"
            >
              {submitting ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Saving...</span>
                </>
              ) : (
                <span>Save Address</span>
              )}
            </button>
          </div>
        </form>
      )}

      {/* Address Cards List */}
      {addresses.length === 0 ? (
        <div className="p-12 text-center border border-dashed border-slate-200 rounded-2xl space-y-3">
          <div className="w-14 h-14 rounded-full bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto">
            <MapPin className="w-7 h-7" />
          </div>
          <h3 className="text-sm font-bold text-[#0D1B2A]">No saved addresses</h3>
          <p className="text-xs text-slate-500 max-w-sm mx-auto">
            Add a shipping address to speed up your future checkout process.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {addresses.map((addr) => (
            <div
              key={addr.uuid}
              className={`p-5 rounded-2xl border transition-all relative flex flex-col justify-between ${
                addr.is_default
                  ? 'bg-[#E3F2FD]/30 border-[#2196F3] shadow-xs'
                  : 'bg-white border-slate-200/80 hover:border-slate-300'
              }`}
            >
              <div>
                <div className="flex items-center justify-between gap-2 mb-2">
                  <div className="flex items-center gap-2">
                    {addr.label === 'Office' ? (
                      <Building2 className="w-4 h-4 text-[#2196F3]" />
                    ) : (
                      <Home className="w-4 h-4 text-[#2196F3]" />
                    )}
                    <span className="text-sm font-bold text-[#0D1B2A]">{addr.name}</span>
                    <span className="text-[10px] px-2 py-0.5 rounded-full bg-slate-100 text-slate-600 font-semibold">
                      {addr.label || 'Home'}
                    </span>
                  </div>
                  {addr.is_default && (
                    <span className="px-2 py-0.5 rounded-full bg-[#2196F3] text-white text-[10px] font-bold">
                      Default
                    </span>
                  )}
                </div>

                <p className="text-xs text-slate-600 font-medium">{addr.phone}</p>
                <p className="text-xs text-slate-500 mt-1 leading-relaxed">
                  {addr.full_address || addr.address}
                </p>
              </div>

              <div className="pt-4 mt-3 border-t border-slate-100 flex items-center justify-between text-xs">
                {!addr.is_default ? (
                  <button
                    onClick={() => handleSetDefault(addr.uuid)}
                    className="text-[#2196F3] font-bold hover:underline cursor-pointer"
                  >
                    Set as Default
                  </button>
                ) : (
                  <span className="text-slate-400 text-[11px] flex items-center gap-1">
                    <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" /> Default Address
                  </span>
                )}

                <button
                  onClick={() => handleDelete(addr.uuid)}
                  disabled={deletingUuid === addr.uuid}
                  className="text-slate-400 hover:text-rose-500 p-1 transition-colors cursor-pointer disabled:opacity-50"
                  title="Delete address"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
