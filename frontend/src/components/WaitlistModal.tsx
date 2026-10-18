import React, { useState } from 'react';
import { EventItem, User, WaitlistEntry } from '../types';
import { joinWaitlist } from '../services/api';
import {
  XIcon,
  ClockIcon,
  UserPlusIcon,
  CheckCircleIcon,
  AlertCircleIcon,
  MapPinIcon,
  CalendarIcon,
  PassbookIcon,
} from './Icons';

interface WaitlistModalProps {
  event: EventItem;
  isOpen: boolean;
  currentUser: User | null;
  onClose: () => void;
  onOpenPassbook?: () => void;
  onWaitlistJoined?: (entry: WaitlistEntry) => void;
}

export const WaitlistModal: React.FC<WaitlistModalProps> = ({
  event,
  isOpen,
  currentUser,
  onClose,
  onOpenPassbook,
  onWaitlistJoined,
}) => {
  const [tierId, setTierId] = useState<string>(event.tiers[0]?.id || '');
  const [quantity, setQuantity] = useState<number>(1);
  const [name, setName] = useState<string>(currentUser?.name || '');
  const [email, setEmail] = useState<string>(currentUser?.email || '');
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [joinedEntry, setJoinedEntry] = useState<WaitlistEntry | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !email.trim()) {
      setError('Please provide your name and email address.');
      return;
    }
    setError(null);
    setSubmitting(true);

    try {
      const entry = await joinWaitlist({
        eventId: event.id,
        tierId: tierId || undefined,
        userId: currentUser?.id,
        customerName: name.trim(),
        customerEmail: email.trim().toLowerCase(),
        quantity,
      });
      setJoinedEntry(entry);
      if (onWaitlistJoined) {
        onWaitlistJoined(entry);
      }
    } catch (err: unknown) {
      const msg = (err as { message?: string })?.message || 'Failed to join waitlist';
      setError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200"
      role="dialog"
      aria-modal="true"
      aria-labelledby="waitlist-modal-title"
    >
      <div className="relative w-full max-w-lg bg-[#141424] border border-cyan-500/40 rounded-2xl shadow-2xl overflow-hidden text-slate-100">
        {/* Header */}
        <div className="bg-gradient-to-r from-cyan-950/80 via-slate-900 to-indigo-950/80 px-6 py-5 border-b border-cyan-800/40 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-400">
              <ClockIcon size={22} />
            </div>
            <div>
              <span className="text-[11px] font-semibold tracking-wider text-cyan-400 uppercase">
                Priority Waitlist
              </span>
              <h3 id="waitlist-modal-title" className="text-lg font-bold text-slate-100 font-display line-clamp-1">
                {event.title}
              </h3>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors cursor-pointer"
            aria-label="Close Waitlist Modal"
          >
            <XIcon size={20} />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6">
          {joinedEntry ? (
            /* Success State */
            <div className="text-center space-y-5 py-4 animate-in zoom-in-95 duration-200">
              <div className="w-16 h-16 mx-auto rounded-full bg-cyan-500/10 border border-cyan-400/30 flex items-center justify-center text-cyan-400 shadow-lg shadow-cyan-500/10">
                <CheckCircleIcon size={36} />
              </div>

              <div className="space-y-1">
                <h4 className="text-xl font-bold text-slate-100">You're on the Waitlist!</h4>
                <p className="text-sm text-slate-400">
                  We'll automatically allocate tickets in order when seats become available.
                </p>
              </div>

              {/* Queue Position Card */}
              <div className="p-4 rounded-xl bg-gradient-to-br from-cyan-950/60 to-slate-900 border border-cyan-500/30 flex items-center justify-around">
                <div className="text-center">
                  <span className="text-xs uppercase tracking-wider text-slate-400">Queue Position</span>
                  <div className="text-3xl font-extrabold text-cyan-300 font-mono mt-0.5">
                    #{joinedEntry.position || 1}
                  </div>
                </div>
                <div className="w-px h-10 bg-slate-700"></div>
                <div className="text-center">
                  <span className="text-xs uppercase tracking-wider text-slate-400">Seats Requested</span>
                  <div className="text-3xl font-extrabold text-slate-200 font-mono mt-0.5">
                    {joinedEntry.quantity}
                  </div>
                </div>
              </div>

              <div className="text-xs text-slate-400 bg-slate-800/60 p-3 rounded-lg border border-slate-700/60 text-left space-y-1">
                <p className="font-semibold text-slate-300">⚡ How it works:</p>
                <p>• If an attendee cancels or a hold expires, tickets are promoted in FIFO order.</p>
                <p>• Promoted attendees get a 15-minute exclusive hold window to complete checkout.</p>
                <p>• You can check live queue progress in your Passbook anytime.</p>
              </div>

              <div className="flex gap-3 pt-2">
                {onOpenPassbook && (
                  <button
                    type="button"
                    onClick={() => {
                      onClose();
                      onOpenPassbook();
                    }}
                    className="flex-1 py-2.5 px-4 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white font-medium text-sm transition-colors flex items-center justify-center gap-2 cursor-pointer shadow-lg shadow-cyan-900/30"
                  >
                    <PassbookIcon size={18} />
                    View in Passbook
                  </button>
                )}
                <button
                  type="button"
                  onClick={onClose}
                  className="flex-1 py-2.5 px-4 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium text-sm transition-colors cursor-pointer border border-slate-700"
                >
                  Done
                </button>
              </div>
            </div>
          ) : (
            /* Waitlist Form */
            <form onSubmit={handleSubmit} className="space-y-4">
              {/* Event Metadata Banner */}
              <div className="p-3.5 rounded-xl bg-slate-800/50 border border-slate-700/60 space-y-1.5 text-xs">
                <div className="flex items-center gap-2 text-slate-300">
                  <MapPinIcon size={14} className="text-cyan-400" />
                  <span>{event.venueName} • {event.venueLocation}</span>
                </div>
                <div className="flex items-center gap-2 text-slate-400">
                  <CalendarIcon size={14} className="text-cyan-400" />
                  <span>{new Date(event.startTime).toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                </div>
                <div className="pt-1 flex items-center gap-2">
                  <span className="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-red-950/80 border border-red-500/40 text-red-300">
                    Event Currently Sold Out
                  </span>
                  <span className="text-slate-400 text-[11px]">
                    Automatic promotion queue active
                  </span>
                </div>
              </div>

              {error && (
                <div className="p-3 rounded-xl bg-red-950/70 border border-red-500/50 flex items-center gap-2.5 text-xs text-red-200">
                  <AlertCircleIcon size={16} className="text-red-400 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              {/* Tiers & Quantity */}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-400 mb-1">
                    Preferred Tier
                  </label>
                  <select
                    value={tierId}
                    onChange={(e) => setTierId(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-xl text-slate-100 text-sm focus:outline-none focus:border-cyan-500"
                  >
                    {event.tiers.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.name} (${t.price.toFixed(2)})
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-400 mb-1">
                    Seats Needed
                  </label>
                  <select
                    value={quantity}
                    onChange={(e) => setQuantity(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-xl text-slate-100 text-sm focus:outline-none focus:border-cyan-500"
                  >
                    <option value={1}>1 Seat</option>
                    <option value={2}>2 Seats</option>
                    <option value={3}>3 Seats</option>
                    <option value={4}>4 Seats</option>
                  </select>
                </div>
              </div>

              {/* Contact Information */}
              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1">
                  Full Name
                </label>
                <input
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Alex Morgan"
                  className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-xl text-slate-100 text-sm focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1">
                  Notification Email
                </label>
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="e.g. alex@example.com"
                  className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-xl text-slate-100 text-sm focus:outline-none focus:border-cyan-500"
                />
                <p className="text-[11px] text-slate-400 mt-1">
                  We will reserve your hold and notify you when a seat is freed.
                </p>
              </div>

              {/* Submit Buttons */}
              <div className="pt-2 flex gap-3">
                <button
                  type="button"
                  onClick={onClose}
                  className="flex-1 py-2.5 px-4 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-sm transition-colors cursor-pointer border border-slate-700"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="flex-1 py-2.5 px-4 rounded-xl bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-medium text-sm transition-colors flex items-center justify-center gap-2 cursor-pointer shadow-lg shadow-cyan-900/30"
                >
                  <UserPlusIcon size={18} />
                  {submitting ? 'Joining Queue...' : 'Join Waitlist'}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};
