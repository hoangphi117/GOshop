'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import {
  Mail,
  Phone,
  MapPin,
  Clock,
  Send,
  MessageSquare,
  CheckCircle2,
  HelpCircle,
  ArrowRight,
  Headphones,
} from 'lucide-react'
import { toast } from 'sonner'

export default function ContactPage() {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [subject, setSubject] = useState('')
  const [message, setMessage] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isSubmitted, setIsSubmitted] = useState(false)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!name.trim() || !email.trim() || !message.trim()) {
      toast.error('Please fill in all required fields.')
      return
    }

    setIsSubmitting(true)

    // Simulate sending message
    setTimeout(() => {
      setIsSubmitting(false)
      setIsSubmitted(true)
      toast.success('Thank you! Your message has been sent to our customer care team.')
    }, 800)
  }

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-12">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 space-y-12">
        {/* Header */}
        <div className="text-center max-w-2xl mx-auto space-y-3">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-semibold">
            <Headphones className="w-3.5 h-3.5 text-[#2196F3]" />
            <span>Customer Support & Assistance</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold text-[#0D1B2A] tracking-tight">
            We&apos;re Here to Help
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            Have a question about your order, return, product details, or partnership?
            Get in touch with our friendly support team anytime.
          </p>
        </div>

        {/* Info Cards Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-3">
            <div className="w-10 h-10 rounded-xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center">
              <Phone className="w-5 h-5" />
            </div>
            <h3 className="text-sm font-bold text-[#0D1B2A]">Call Us Directly</h3>
            <p className="text-xs text-slate-500">Toll-free customer hotline</p>
            <p className="text-sm font-black text-[#0D47A1]">1900-6868</p>
          </div>

          <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-3">
            <div className="w-10 h-10 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
              <Mail className="w-5 h-5" />
            </div>
            <h3 className="text-sm font-bold text-[#0D1B2A]">Email Support</h3>
            <p className="text-xs text-slate-500">Replies within 2-4 hours</p>
            <p className="text-sm font-bold text-slate-800 break-all">support@goshop.com</p>
          </div>

          <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-3">
            <div className="w-10 h-10 rounded-xl bg-amber-50 text-amber-600 flex items-center justify-center">
              <Clock className="w-5 h-5" />
            </div>
            <h3 className="text-sm font-bold text-[#0D1B2A]">Support Hours</h3>
            <p className="text-xs text-slate-500">Mon – Sun (7 days/week)</p>
            <p className="text-xs font-semibold text-slate-800">8:00 AM – 9:00 PM ICT</p>
          </div>

          <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-3">
            <div className="w-10 h-10 rounded-xl bg-purple-50 text-purple-600 flex items-center justify-center">
              <MapPin className="w-5 h-5" />
            </div>
            <h3 className="text-sm font-bold text-[#0D1B2A]">Headquarters</h3>
            <p className="text-xs text-slate-500">District 1, Ho Chi Minh City</p>
            <p className="text-xs font-semibold text-slate-800">Vietnam</p>
          </div>
        </div>

        {/* Contact Form & Side FAQ Card */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          {/* Contact Form (7 cols) */}
          <div className="lg:col-span-7 p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
            <div className="space-y-1">
              <h2 className="text-xl font-extrabold text-[#0D1B2A]">
                Send Us a Message
              </h2>
              <p className="text-xs text-slate-500">
                Fill out the form below and we will get back to you promptly.
              </p>
            </div>

            {isSubmitted ? (
              <div className="py-12 text-center space-y-4">
                <div className="w-16 h-16 rounded-full bg-emerald-100 text-emerald-600 flex items-center justify-center mx-auto">
                  <CheckCircle2 className="w-8 h-8" />
                </div>
                <h3 className="text-lg font-bold text-[#0D1B2A]">Message Received!</h3>
                <p className="text-xs text-slate-500 max-w-sm mx-auto leading-relaxed">
                  Thank you for reaching out, <strong>{name}</strong>. Our team has received
                  your message and will reply to <strong>{email}</strong> shortly.
                </p>
                <button
                  type="button"
                  onClick={() => {
                    setIsSubmitted(false)
                    setName('')
                    setEmail('')
                    setPhone('')
                    setSubject('')
                    setMessage('')
                  }}
                  className="px-5 py-2.5 rounded-xl border border-slate-200 text-xs font-semibold text-slate-700 hover:bg-slate-50 transition-colors cursor-pointer"
                >
                  Send Another Message
                </button>
              </div>
            ) : (
              <form onSubmit={handleSubmit} className="space-y-4">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Full Name <span className="text-rose-500">*</span>
                    </label>
                    <input
                      type="text"
                      required
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="e.g. John Doe"
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Email Address <span className="text-rose-500">*</span>
                    </label>
                    <input
                      type="email"
                      required
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="name@example.com"
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Phone Number (Optional)
                    </label>
                    <input
                      type="tel"
                      value={phone}
                      onChange={(e) => setPhone(e.target.value)}
                      placeholder="0912 345 678"
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Subject
                    </label>
                    <input
                      type="text"
                      value={subject}
                      onChange={(e) => setSubject(e.target.value)}
                      placeholder="Order inquiry, return, etc."
                      className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">
                    Your Message <span className="text-rose-500">*</span>
                  </label>
                  <textarea
                    required
                    rows={5}
                    value={message}
                    onChange={(e) => setMessage(e.target.value)}
                    placeholder="How can we help you with your order or questions?"
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] resize-none"
                  />
                </div>

                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="w-full sm:w-auto px-7 py-3 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-sm flex items-center justify-center gap-2 cursor-pointer disabled:opacity-60"
                >
                  <Send className="w-3.5 h-3.5" />
                  <span>{isSubmitting ? 'Sending Message...' : 'Send Message'}</span>
                </button>
              </form>
            )}
          </div>

          {/* Quick FAQ / Self-service (5 cols) */}
          <div className="lg:col-span-5 space-y-6">
            <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-4">
              <div className="flex items-center gap-2">
                <HelpCircle className="w-5 h-5 text-[#2196F3]" />
                <h3 className="text-lg font-bold text-[#0D1B2A]">Quick Answers</h3>
              </div>
              <p className="text-xs text-slate-500">
                Looking for immediate answers? Check these common topics before messaging:
              </p>

              <div className="space-y-3 pt-2">
                <Link
                  href="/faq"
                  className="block p-3.5 rounded-xl bg-slate-50 hover:bg-[#E3F2FD] border border-slate-100 transition-colors group"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-[#0D1B2A] group-hover:text-[#0D47A1]">
                      How do I track my delivery?
                    </span>
                    <ArrowRight className="w-3.5 h-3.5 text-slate-400 group-hover:text-[#0D47A1]" />
                  </div>
                  <p className="text-[11px] text-slate-500 mt-0.5">
                    Real-time timeline tracking with GHN Express integration.
                  </p>
                </Link>

                <Link
                  href="/return-policy"
                  className="block p-3.5 rounded-xl bg-slate-50 hover:bg-[#E3F2FD] border border-slate-100 transition-colors group"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-[#0D1B2A] group-hover:text-[#0D47A1]">
                      7-day Free Return Policy
                    </span>
                    <ArrowRight className="w-3.5 h-3.5 text-slate-400 group-hover:text-[#0D47A1]" />
                  </div>
                  <p className="text-[11px] text-slate-500 mt-0.5">
                    How to request a return and receive full refunds.
                  </p>
                </Link>

                <Link
                  href="/shipping-policy"
                  className="block p-3.5 rounded-xl bg-slate-50 hover:bg-[#E3F2FD] border border-slate-100 transition-colors group"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-[#0D1B2A] group-hover:text-[#0D47A1]">
                      Delivery Times & Rates
                    </span>
                    <ArrowRight className="w-3.5 h-3.5 text-slate-400 group-hover:text-[#0D47A1]" />
                  </div>
                  <p className="text-[11px] text-slate-500 mt-0.5">
                    Standard delivery timelines across all Vietnam regions.
                  </p>
                </Link>
              </div>

              <div className="pt-2">
                <Link
                  href="/faq"
                  className="inline-flex items-center gap-1.5 text-xs font-bold text-[#2196F3] hover:underline"
                >
                  <span>Visit Help Center / FAQ Hub</span>
                  <ArrowRight className="w-3.5 h-3.5" />
                </Link>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
