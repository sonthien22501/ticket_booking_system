import React, { useState, useEffect } from 'react';
import {
  fetchAdminAnalytics,
  createAdminEvent,
  updateEventStatus,
} from '../services/api';
import { AdminAnalyticsData, CreateEventPayload, User } from '../types';
import {
  XIcon,
  ChartBarIcon,
  PlusCircleIcon,
  CurrencyDollarIcon,
  TicketIcon,
  SparklesIcon,
  CalendarIcon,
  MapPinIcon,
  PlayIcon,
  PauseIcon,
} from './Icons';

interface AdminDashboardModalProps {
  isOpen: boolean;
  onClose: () => void;
  user: User | null;
  onEventCreated?: () => void;
}

export const AdminDashboardModal: React.FC<AdminDashboardModalProps> = ({
  isOpen,
  onClose,
  user,
  onEventCreated,
}) => {
  const [activeTab, setActiveTab] = useState<'analytics' | 'create'>('analytics');
  const [analytics, setAnalytics] = useState<AdminAnalyticsData | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  // Create Event Form state
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState('Music & Concerts');
  const [venueName, setVenueName] = useState('Metropolitan Grand Hall');
  const [venueLocation, setVenueLocation] = useState('Berlin, Germany');
  const [startDate, setStartDate] = useState('2026-11-20T19:00');
  const [endDate, setEndDate] = useState('2026-11-20T22:30');
  const [imageUrl, setImageUrl] = useState(
    'https://images.unsplash.com/photo-1501386761578-eac5c94b800a?auto=format&fit=crop&w=1200&q=80'
  );
  const [tierName, setTierName] = useState('VIP Premiere Pass');
  const [tierPrice, setTierPrice] = useState('120');
  const [tierCapacity, setTierCapacity] = useState('50');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const loadAnalytics = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await fetchAdminAnalytics();
      setAnalytics(data);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load telemetry';
      setError(msg);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      loadAnalytics();
      setSuccessMsg(null);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleToggleEventStatus = async (eventId: string, currentStatus: string) => {
    try {
      const nextStatus = currentStatus === 'PAUSED' ? 'ACTIVE' : 'PAUSED';
      await updateEventStatus(eventId, nextStatus);
      await loadAnalytics();
      if (onEventCreated) onEventCreated();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to toggle event status';
      setError(msg);
    }
  };

  const handleCreateEvent = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) {
      setError('Event title is required');
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);

      const payload: CreateEventPayload = {
        title: title.trim(),
        description: description.trim(),
        category,
        venueName: venueName.trim(),
        venueLocation: venueLocation.trim(),
        startTime: new Date(startDate).toISOString(),
        endTime: new Date(endDate).toISOString(),
        imageUrl: imageUrl.trim() || undefined,
        tiers: [
          {
            name: tierName.trim() || 'General Admission',
            price: parseFloat(tierPrice) || 50,
            capacity: parseInt(tierCapacity, 10) || 50,
          },
        ],
      };

      await createAdminEvent(payload);
      setSuccessMsg(`Event "${title}" published successfully with provisioned 2D seating!`);
      setTitle('');
      setDescription('');
      setActiveTab('analytics');
      await loadAnalytics();
      if (onEventCreated) onEventCreated();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create event';
      setError(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-4 bg-black/80 backdrop-blur-md"
      role="dialog"
      aria-modal="true"
      aria-label="Organizer Studio & Live Telemetry"
    >
      <div className="bg-slate-900 border border-slate-700/80 rounded-2xl shadow-2xl text-slate-100 max-w-4xl w-full max-h-[92vh] flex flex-col overflow-hidden animate-in fade-in zoom-in-95 duration-200">
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-950/60">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-500/20 to-purple-600/30 border border-indigo-500/30 flex items-center justify-center text-indigo-400">
              <SparklesIcon size={22} />
            </div>
            <div>
              <div className="flex items-center space-x-2">
                <h2 className="text-lg font-bold text-white tracking-wide">Organizer Studio & Telemetry</h2>
                <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 mr-1.5 animate-pulse" />
                  Live Sync
                </span>
              </div>
              <p className="text-xs text-slate-400">
                Admin Console • {user ? `${user.name} (${user.role})` : 'Authorized Operator'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            aria-label="Close modal"
            className="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800/80 transition-colors"
          >
            <XIcon size={20} />
          </button>
        </div>

        {/* Tab Navigation */}
        <div className="flex border-b border-slate-800 bg-slate-900/90 px-6 pt-3 space-x-4">
          <button
            onClick={() => { setActiveTab('analytics'); setError(null); }}
            className={`flex items-center space-x-2 pb-3 px-1 text-sm font-medium border-b-2 transition-all ${
              activeTab === 'analytics'
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <ChartBarIcon size={18} />
            <span>Overview & Live Telemetry</span>
          </button>
          <button
            onClick={() => { setActiveTab('create'); setError(null); }}
            className={`flex items-center space-x-2 pb-3 px-1 text-sm font-medium border-b-2 transition-all ${
              activeTab === 'create'
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <PlusCircleIcon size={18} />
            <span>Create New Event</span>
          </button>
        </div>

        {/* Notifications */}
        {error && (
          <div className="mx-6 mt-4 p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex justify-between items-center">
            <span>{error}</span>
            <button onClick={() => setError(null)} className="text-rose-400 hover:underline">Dismiss</button>
          </div>
        )}
        {successMsg && (
          <div className="mx-6 mt-4 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex justify-between items-center">
            <span>{successMsg}</span>
            <button onClick={() => setSuccessMsg(null)} className="text-emerald-400 hover:underline">Dismiss</button>
          </div>
        )}

        {/* Content Area */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {activeTab === 'analytics' ? (
            <>
              {/* 4 KPI Metrics Cards */}
              <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
                <div className="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex flex-col justify-between">
                  <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
                    <span>Total Revenue</span>
                    <CurrencyDollarIcon size={18} className="text-emerald-400" />
                  </div>
                  <div className="mt-3">
                    <span className="text-2xl font-extrabold text-white">
                      ${analytics ? analytics.totalRevenue.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '0.00'}
                    </span>
                    <p className="text-[11px] text-emerald-400 mt-1">Confirmed gross receipts</p>
                  </div>
                </div>

                <div className="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex flex-col justify-between">
                  <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
                    <span>Tickets Issued</span>
                    <TicketIcon size={18} className="text-indigo-400" />
                  </div>
                  <div className="mt-3">
                    <span className="text-2xl font-extrabold text-white">
                      {analytics ? analytics.totalTicketsSold : 0}
                    </span>
                    <p className="text-[11px] text-slate-400 mt-1">
                      From {analytics ? analytics.confirmedBookings : 0} orders
                    </p>
                  </div>
                </div>

                <div className="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex flex-col justify-between">
                  <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
                    <span>Avg Occupancy</span>
                    <ChartBarIcon size={18} className="text-amber-400" />
                  </div>
                  <div className="mt-3">
                    <span className="text-2xl font-extrabold text-white">
                      {analytics ? analytics.averageOccupancy.toFixed(1) : '0.0'}%
                    </span>
                    <div className="w-full bg-slate-700/60 h-1.5 rounded-full mt-2 overflow-hidden">
                      <div
                        className="bg-amber-400 h-full rounded-full transition-all duration-500"
                        style={{ width: `${Math.min(analytics ? analytics.averageOccupancy : 0, 100)}%` }}
                      />
                    </div>
                  </div>
                </div>

                <div className="bg-slate-800/60 border border-slate-700/50 p-4 rounded-xl flex flex-col justify-between">
                  <div className="flex items-center justify-between text-slate-400 text-xs font-medium">
                    <span>Active Events</span>
                    <CalendarIcon size={18} className="text-sky-400" />
                  </div>
                  <div className="mt-3">
                    <span className="text-2xl font-extrabold text-white">
                      {analytics ? analytics.activeEventsCount : 0}
                    </span>
                    <p className="text-[11px] text-slate-400 mt-1">Ready for ticket booking</p>
                  </div>
                </div>
              </div>

              {/* Event Performance & Status Toggle Table */}
              <div className="bg-slate-800/40 border border-slate-700/50 rounded-xl p-4">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-semibold text-slate-200">Catalog Inventory & Venue Capacities</h3>
                  <button
                    onClick={loadAnalytics}
                    disabled={loading}
                    className="text-xs text-indigo-400 hover:text-indigo-300 font-medium transition-colors"
                  >
                    {loading ? 'Refreshing...' : 'Refresh Data'}
                  </button>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs text-slate-300">
                    <thead className="bg-slate-900/60 text-slate-400 uppercase font-semibold text-[10px] tracking-wider border-b border-slate-700/60">
                      <tr>
                        <th className="py-2.5 px-3">Event</th>
                        <th className="py-2.5 px-3">Category</th>
                        <th className="py-2.5 px-3">Venue</th>
                        <th className="py-2.5 px-3">Status</th>
                        <th className="py-2.5 px-3">Capacity</th>
                        <th className="py-2.5 px-3">Revenue</th>
                        <th className="py-2.5 px-3 text-right">Action</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800">
                      {analytics?.eventsSummary && analytics.eventsSummary.length > 0 ? (
                        analytics.eventsSummary.map((ev) => (
                          <tr key={ev.eventId} className="hover:bg-slate-800/30 transition-colors">
                            <td className="py-2.5 px-3 font-medium text-white">{ev.title}</td>
                            <td className="py-2.5 px-3 text-slate-400">{ev.category}</td>
                            <td className="py-2.5 px-3 text-slate-400">{ev.venueName}</td>
                            <td className="py-2.5 px-3">
                              <span
                                className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold ${
                                  ev.status === 'PAUSED'
                                    ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                                    : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                                }`}
                              >
                                {ev.status}
                              </span>
                            </td>
                            <td className="py-2.5 px-3">
                              <div className="flex items-center space-x-2">
                                <span>{ev.bookedSeats} / {ev.totalSeats}</span>
                                <span className="text-[10px] text-slate-500">
                                  ({ev.occupancyPercent.toFixed(0)}%)
                                </span>
                              </div>
                            </td>
                            <td className="py-2.5 px-3 text-emerald-400 font-medium">
                              ${ev.grossRevenue.toFixed(2)}
                            </td>
                            <td className="py-2.5 px-3 text-right">
                              <button
                                onClick={() => handleToggleEventStatus(ev.eventId, ev.status)}
                                title={ev.status === 'PAUSED' ? 'Resume ticket sales' : 'Pause ticket sales'}
                                className={`inline-flex items-center px-2.5 py-1 rounded text-[11px] font-medium transition-colors ${
                                  ev.status === 'PAUSED'
                                    ? 'bg-emerald-600/30 text-emerald-300 hover:bg-emerald-600/50'
                                    : 'bg-amber-600/30 text-amber-300 hover:bg-amber-600/50'
                                }`}
                              >
                                {ev.status === 'PAUSED' ? (
                                  <>
                                    <PlayIcon size={12} className="mr-1" /> Resume
                                  </>
                                ) : (
                                  <>
                                    <PauseIcon size={12} className="mr-1" /> Pause
                                  </>
                                )}
                              </button>
                            </td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={7} className="py-6 text-center text-slate-500">
                            No events currently published in catalog
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Recent Bookings Feed */}
              <div className="bg-slate-800/40 border border-slate-700/50 rounded-xl p-4">
                <h3 className="text-sm font-semibold text-slate-200 mb-3">Recent Ticket Purchases</h3>
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs text-slate-300">
                    <thead className="bg-slate-900/60 text-slate-400 uppercase font-semibold text-[10px] tracking-wider border-b border-slate-700/60">
                      <tr>
                        <th className="py-2.5 px-3">Reference</th>
                        <th className="py-2.5 px-3">Customer</th>
                        <th className="py-2.5 px-3">Event</th>
                        <th className="py-2.5 px-3">Tickets</th>
                        <th className="py-2.5 px-3">Amount</th>
                        <th className="py-2.5 px-3">Status</th>
                        <th className="py-2.5 px-3 text-right">Time</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800">
                      {analytics?.recentBookings && analytics.recentBookings.length > 0 ? (
                        analytics.recentBookings.map((b) => (
                          <tr key={b.id} className="hover:bg-slate-800/30 transition-colors">
                            <td className="py-2.5 px-3 font-mono text-indigo-400 font-medium">
                              {b.bookingReference}
                            </td>
                            <td className="py-2.5 px-3 font-medium text-white">
                              {b.customerName}
                              <div className="text-[10px] text-slate-500 font-normal">{b.customerEmail}</div>
                            </td>
                            <td className="py-2.5 px-3 text-slate-300">{b.eventTitle}</td>
                            <td className="py-2.5 px-3 text-slate-300">{b.quantity}</td>
                            <td className="py-2.5 px-3 text-emerald-400 font-semibold">
                              ${b.totalAmount.toFixed(2)}
                            </td>
                            <td className="py-2.5 px-3">
                              <span
                                className={`inline-flex items-center px-2 py-0.5 rounded text-[10px] font-bold ${
                                  b.status === 'CONFIRMED'
                                    ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                                    : b.status === 'CANCELLED'
                                    ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                                    : 'bg-slate-700 text-slate-300'
                                }`}
                              >
                                {b.status}
                              </span>
                            </td>
                            <td className="py-2.5 px-3 text-right text-slate-500 font-mono text-[11px]">
                              {new Date(b.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                            </td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={7} className="py-6 text-center text-slate-500">
                            No recent bookings recorded
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            </>
          ) : (
            /* Tab 2: Create New Event Studio */
            <form onSubmit={handleCreateEvent} className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    Event Title *
                  </label>
                  <input
                    type="text"
                    required
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder="e.g. Cyberpunk Synthwave Live 2026"
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    Category
                  </label>
                  <select
                    value={category}
                    onChange={(e) => setCategory(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  >
                    <option value="Music & Concerts">Music & Concerts</option>
                    <option value="Technology & Conferences">Technology & Conferences</option>
                    <option value="Cinema & Film Premiere">Cinema & Film Premiere</option>
                    <option value="Theater & Arts">Theater & Arts</option>
                    <option value="Sports & Live Events">Sports & Live Events</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  Description
                </label>
                <textarea
                  rows={2}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="Describe the experience, performers, keynote speakers..."
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    <MapPinIcon size={14} className="inline mr-1 text-slate-400" />
                    Venue Name
                  </label>
                  <input
                    type="text"
                    required
                    value={venueName}
                    onChange={(e) => setVenueName(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    City / Location
                  </label>
                  <input
                    type="text"
                    required
                    value={venueLocation}
                    onChange={(e) => setVenueLocation(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    Start Date & Time
                  </label>
                  <input
                    type="datetime-local"
                    required
                    value={startDate}
                    onChange={(e) => setStartDate(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    End Date & Time
                  </label>
                  <input
                    type="datetime-local"
                    required
                    value={endDate}
                    onChange={(e) => setEndDate(e.target.value)}
                    className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  Poster Image URL
                </label>
                <input
                  type="url"
                  value={imageUrl}
                  onChange={(e) => setImageUrl(e.target.value)}
                  placeholder="https://..."
                  className="w-full bg-slate-800 border border-slate-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-indigo-500"
                />
              </div>

              {/* Ticket Tier & Auto Seating Provisioning */}
              <div className="bg-slate-800/60 border border-slate-700/60 rounded-xl p-4">
                <div className="flex items-center space-x-2 text-xs font-semibold text-indigo-400 mb-3">
                  <TicketIcon size={16} />
                  <span>Ticket Tier & Automated 2D Seating Grid</span>
                </div>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Tier Name</label>
                    <input
                      type="text"
                      value={tierName}
                      onChange={(e) => setTierName(e.target.value)}
                      className="w-full bg-slate-900 border border-slate-700 rounded-lg px-2.5 py-1.5 text-xs text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Price ($)</label>
                    <input
                      type="number"
                      min="1"
                      step="0.01"
                      value={tierPrice}
                      onChange={(e) => setTierPrice(e.target.value)}
                      className="w-full bg-slate-900 border border-slate-700 rounded-lg px-2.5 py-1.5 text-xs text-white"
                    />
                  </div>
                  <div>
                    <label className="block text-[11px] text-slate-400 mb-1">Seat Capacity (Rows auto-created)</label>
                    <input
                      type="number"
                      min="10"
                      max="100"
                      value={tierCapacity}
                      onChange={(e) => setTierCapacity(e.target.value)}
                      className="w-full bg-slate-900 border border-slate-700 rounded-lg px-2.5 py-1.5 text-xs text-white"
                    />
                  </div>
                </div>
                <p className="text-[10px] text-slate-500 mt-2">
                  ⚡ Automatic Provisioning: Catalog service generates rows A-E with 10 seats per row for the interactive 2D seat map upon publishing.
                </p>
              </div>

              <div className="flex justify-end space-x-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setActiveTab('analytics')}
                  className="px-4 py-2 text-sm text-slate-400 hover:text-white transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="px-5 py-2 rounded-xl bg-gradient-to-r from-indigo-500 to-purple-600 hover:from-indigo-400 hover:to-purple-500 text-white font-medium text-sm shadow-lg shadow-indigo-500/25 transition-all disabled:opacity-50"
                >
                  {isSubmitting ? 'Publishing Event...' : 'Publish Event to Live Catalog'}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};
