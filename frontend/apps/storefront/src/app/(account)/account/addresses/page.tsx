'use client'

import React, { useState, useEffect, useCallback } from 'react'
import { useAuthStore } from '@/stores/auth-store'
import { MapPin, Plus, Trash2, CheckCircle2, Home } from 'lucide-react'
import { toast } from 'sonner'

interface Address {
  uuid: string
  receiver_name: string
  phone_number: string
  street_address: string
  ward?: string
  district?: string
  city?: string
  is_default?: boolean
}

export default function AddressesPage() {
  const { accessToken } = useAuthStore()
  const [addresses, setAddresses] = useState<Address[]>([])
  const [loading, setLoading] = useState(true)
  const [showAddForm, setShowAddForm] = useState(false)

  // Form State
  const [receiverName, setReceiverName] = useState('')
  const [phone, setPhone] = useState('')
  const [streetAddress, setStreetAddress] = useState('')
  const [city, setCity] = useState('Ho Chi Minh City')
  const [district, setDistrict] = useState('District 1')
  const [ward, setWard] = useState('Ben Nghe Ward')
  const [isDefault, setIsDefault] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const fetchAddresses = useCallback(() => {
    if (!accessToken) return
    fetch('http://localhost:8080/api/v1/addresses', {
      headers: {
        'Authorization': `Bearer ${accessToken}`,
        'X-API-KEY': 'abcd-1234-1111',
      },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.data && Array.isArray(data.data)) {
          setAddresses(data.data)
        } else {
          setAddresses([])
        }
      })
      .catch(() => setAddresses([]))
      .finally(() => setLoading(false))
  }, [accessToken])

  useEffect(() => {
    fetchAddresses()
  }, [fetchAddresses])

  const handleCreateAddress = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)

    try {
      const res = await fetch('http://localhost:8080/api/v1/addresses', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${accessToken}`,
          'X-API-KEY': 'abcd-1234-1111',
        },
        body: JSON.stringify({
          receiver_name: receiverName,
          phone_number: phone,
          street_address: streetAddress,
          city,
          district,
          ward,
          is_default: isDefault,
        }),
      })

      if (!res.ok) {
        throw new Error('Failed to create address.')
      }

      toast.success('Address saved successfully!')
      setShowAddForm(false)
      // Reset form
      setReceiverName('')
      setPhone('')
      setStreetAddress('')
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

    try {
      const res = await fetch(`http://localhost:8080/api/v1/addresses/${uuid}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${accessToken}`,
          'X-API-KEY': 'abcd-1234-1111',
        },
      })

      if (!res.ok) throw new Error('Failed to delete address.')

      toast.success('Address removed.')
      setAddresses((prev) => prev.filter((a) => a.uuid !== uuid))
    } catch {
      toast.error('Could not remove address.')
    }
  }

  const handleSetDefault = async (uuid: string) => {
    try {
      const res = await fetch(
        `http://localhost:8080/api/v1/addresses/${uuid}/default`,
        {
          method: 'PATCH',
          headers: {
            'Authorization': `Bearer ${accessToken}`,
            'X-API-KEY': 'abcd-1234-1111',
          },
        }
      )

      if (!res.ok) throw new Error('Failed to update default address.')

      toast.success('Default address updated.')
      fetchAddresses()
    } catch {
      toast.error('Could not set default address.')
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
          className="inline-flex items-center gap-1.5 px-4 py-2.5 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-colors shadow-xs"
        >
          <Plus className="w-4 h-4" />
          <span>Add New Address</span>
        </button>
      </div>

      {/* Add New Address Form Modal/Panel */}
      {showAddForm && (
        <form
          onSubmit={handleCreateAddress}
          className="p-5 rounded-2xl bg-slate-50 border border-slate-200/80 space-y-4"
        >
          <h3 className="text-sm font-bold text-[#0D1B2A]">New Address Details</h3>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">
                Recipient Name <span className="text-rose-500">*</span>
              </label>
              <input
                type="text"
                required
                value={receiverName}
                onChange={(e) => setReceiverName(e.target.value)}
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

          <div className="space-y-1">
            <label className="text-xs font-semibold text-slate-700">
              Street Address <span className="text-rose-500">*</span>
            </label>
            <input
              type="text"
              required
              value={streetAddress}
              onChange={(e) => setStreetAddress(e.target.value)}
              placeholder="Apartment, house number, street name"
              className="w-full px-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">City</label>
              <input
                type="text"
                value={city}
                onChange={(e) => setCity(e.target.value)}
                className="w-full px-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A]"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">District</label>
              <input
                type="text"
                value={district}
                onChange={(e) => setDistrict(e.target.value)}
                className="w-full px-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A]"
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold text-slate-700">Ward</label>
              <input
                type="text"
                value={ward}
                onChange={(e) => setWard(e.target.value)}
                className="w-full px-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A]"
              />
            </div>
          </div>

          <div className="flex items-center gap-2 pt-1">
            <input
              type="checkbox"
              id="isDefault"
              checked={isDefault}
              onChange={(e) => setIsDefault(e.target.checked)}
              className="rounded text-[#2196F3]"
            />
            <label htmlFor="isDefault" className="text-xs text-slate-700 cursor-pointer">
              Set as default shipping address
            </label>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setShowAddForm(false)}
              className="px-4 py-2 rounded-xl border border-slate-200 bg-white text-xs font-semibold text-slate-600 hover:bg-slate-100"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="px-5 py-2 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-colors"
            >
              {submitting ? 'Saving...' : 'Save Address'}
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
                    <Home className="w-4 h-4 text-[#2196F3]" />
                    <span className="text-sm font-bold text-[#0D1B2A]">
                      {addr.receiver_name}
                    </span>
                  </div>
                  {addr.is_default && (
                    <span className="px-2 py-0.5 rounded-full bg-[#2196F3] text-white text-[10px] font-bold">
                      Default
                    </span>
                  )}
                </div>

                <p className="text-xs text-slate-600 font-medium">
                  {addr.phone_number}
                </p>
                <p className="text-xs text-slate-500 mt-1 leading-relaxed">
                  {addr.street_address}
                  {addr.ward && `, ${addr.ward}`}
                  {addr.district && `, ${addr.district}`}
                  {addr.city && `, ${addr.city}`}
                </p>
              </div>

              <div className="pt-4 mt-3 border-t border-slate-100 flex items-center justify-between text-xs">
                {!addr.is_default ? (
                  <button
                    onClick={() => handleSetDefault(addr.uuid)}
                    className="text-[#2196F3] font-bold hover:underline"
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
                  className="text-slate-400 hover:text-rose-500 p-1 transition-colors"
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
