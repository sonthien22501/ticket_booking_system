import React, { useState, useEffect, useCallback } from 'react';
import { BookingResponse, User, TicketItem, WaitlistEntry } from '../types';
import {
  fetchUserBookings,
  cancelBooking,
  downloadInvoicePdf,
  fetchUserWaitlists,
  cancelWaitlistEntry,
  claimWaitlistReservation,
} from '../services/api';
import { formatCurrency } from '../services/currency';
import { TicketPassModal } from './TicketPassModal';
import { TicketTransferModal } from './TicketTransferModal';
import { ConfirmationDialog } from './ConfirmationDialog';
import {
  XIcon,
  TicketIcon,
  PassbookIcon,
  CopyIcon,
  CheckIcon,
  AlertCircleIcon,
  TrashIcon,
  SearchIcon,
  RefreshCwIcon,
  QrCodeIcon,
  FileTextIcon,
  ClockIcon,
  Share2Icon,
} from './Icons';

interface MyBookingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentUser: User | null;
  onBrowseEvents?: () => void;
  onBookingCancelled?: (bookingId: string) => void;
}

export const MyBookingsModal: React.FC<MyBookingsModalProps> = ({
  isOpen,
  onClose,
  currentUser,
  onBrowseEvents,
  onBookingCancelled,
}) => {
  const [bookings, setBookings] = useState<BookingResponse[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [filterTab, setFilterTab] = useState<'ALL' | 'CONFIRMED' | 'CANCELLED' | 'WAITLIST'>('ALL');
  const [guestEmail, setGuestEmail] = useState<string>('');
  const [copiedRef, setCopiedRef] = useState<string | null>(null);

  // Waitlist state
  const [waitlistEntries, setWaitlistEntries] = useState<WaitlistEntry[]>([]);
  const [waitlistLoading, setWaitlistLoading] = useState<boolean>(false);
  const [claimingWaitlistId, setClaimingWaitlistId] = useState<string | null>(null);

  // Ticket transfer modal state
  const [transferTicketState, setTransferTicketState] = useState<{
    booking: BookingResponse;
    ticket: TicketItem;
  } | null>(null);

  // Ticket pass modal state
  const [activeTicket, setActiveTicket] = useState<{ booking: BookingResponse; ticket: TicketItem } | null>(null);

  // Cancel dialog state
  const [cancellingBookingId, setCancellingBookingId] = useState<string | null>(null);
  const [isCancelConfirmOpen, setIsCancelConfirmOpen] = useState<boolean>(false);
  const [actionInProgress, setActionInProgress] = useState<boolean>(false);

  const loadBookings = useCallback(async (emailOverride?: string) => {
    setLoading(true);
    setError(null);
    try {
      const emailToUse = emailOverride !== undefined ? emailOverride : (currentUser?.email || guestEmail);
      const params: { userId?: string; email?: string } = {};
      if (currentUser?.id) {
        params.userId = currentUser.id;
      }
      if (emailToUse) {
        params.email = emailToUse;
      }

      if (!params.userId && !params.email) {
        setBookings([]);
        setLoading(false);
        return;
      }

      const list = await fetchUserBookings(params);
      setBookings(list);
    } catch (err: unknown) {
      const msg = (err as { message?: string })?.message || 'Failed to load bookings';
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, [currentUser, guestEmail]);

  const loadWaitlists = useCallback(async (emailOverride?: string) => {
    setWaitlistLoading(true);
    try {
      const emailToUse = emailOverride !== undefined ? emailOverride : (currentUser?.email || guestEmail);
      const list = await fetchUserWaitlists({
        userId: currentUser?.id,
        email: emailToUse || undefined,
      });
      setWaitlistEntries(list);
    } catch {
      // Ignore waitlist fetch errors silently
    } finally {
      setWaitlistLoading(false);
    }
  }, [currentUser, guestEmail]);

  useEffect(() => {
    if (isOpen) {
      if (currentUser?.email) {
        setGuestEmail(currentUser.email);
      }
      loadBookings();
      loadWaitlists();
    }
  }, [isOpen, currentUser, loadBookings, loadWaitlists]);

  const handleCancelWaitlist = async (id: string) => {
    try {
      await cancelWaitlistEntry(id, currentUser?.id);
      setWaitlistEntries((prev) => prev.filter((w) => w.id !== id));
    } catch (err: unknown) {
      const msg = (err as { message?: string })?.message || 'Failed to cancel waitlist entry';
      setError(msg);
    }
  };

  const handleClaimWaitlist = async (id: string) => {
    setClaimingWaitlistId(id);
    setError(null);
    try {
      await claimWaitlistReservation(id);
      await Promise.all([loadBookings(), loadWaitlists()]);
      setFilterTab('CONFIRMED');
    } catch (err: unknown) {
      const msg = (err as { message?: string })?.message || 'Failed to claim waitlist hold';
      setError(msg);
    } finally {
      setClaimingWaitlistId(null);
    }
  };

  if (!isOpen) return null;

  const handleGuestSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (guestEmail.trim()) {
      loadBookings(guestEmail.trim());
    }
  };

  const handleCopyReference = (ref: string) => {
    if (navigator?.clipboard?.writeText) {
      navigator.clipboard.writeText(ref);
      setCopiedRef(ref);
      setTimeout(() => setCopiedRef(null), 2000);
    }
  };

  const handleOpenCancelConfirm = (bookingId: string) => {
    setCancellingBookingId(bookingId);
    setIsCancelConfirmOpen(true);
  };

  const handleExecuteCancel = async () => {
    if (!cancellingBookingId) return;
    setActionInProgress(true);
    try {
      await cancelBooking(cancellingBookingId);
      // Update local booking status
      setBookings((prev) =>
        prev.map((b) =>
          b.bookingId === cancellingBookingId || b.bookingReference === cancellingBookingId
            ? { ...b, status: 'CANCELLED' }
            : b
        )
      );
      if (onBookingCancelled) {
        onBookingCancelled(cancellingBookingId);
      }
      setIsCancelConfirmOpen(false);
      setCancellingBookingId(null);
    } catch (err: unknown) {
      const msg = (err as { message?: string })?.message || 'Failed to cancel reservation';
      setError(msg);
    } finally {
      setActionInProgress(false);
    }
  };

  const filteredBookings = bookings.filter((b) => {
    if (filterTab === 'ALL') return true;
    return b.status === filterTab;
  });

  return (
    <>
      <div
        className="fixed inset-0 z-40 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md"
        role="dialog"
        aria-modal="true"
        aria-labelledby="my-bookings-title"
      >
        <div className="relative w-full max-w-2xl max-h-[90vh] bg-[#16162A] border border-indigo-700/60 rounded-2xl shadow-2xl flex flex-col text-slate-100 overflow-hidden animate-in fade-in zoom-in-95 duration-200">
          {/* Header */}
          <div className="px-6 py-4 bg-gradient-to-r from-indigo-950 via-slate-900 to-indigo-900 border-b border-indigo-800/40 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-400">
                <PassbookIcon size={22} />
              </div>
              <div>
                <h3 id="my-bookings-title" className="text-lg font-bold text-slate-100 font-display">
                  My Bookings & Passbook
                </h3>
                <p className="text-xs text-slate-400">
                  {currentUser ? `Signed in as ${currentUser.name}` : 'View your tickets and entry passes'}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => loadBookings()}
                disabled={loading}
                className="p-2 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors cursor-pointer"
                title="Refresh Bookings"
                aria-label="Refresh Bookings"
              >
                <RefreshCwIcon size={18} className={loading ? 'animate-spin' : ''} />
              </button>
              <button
                type="button"
                onClick={onClose}
                className="p-2 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors cursor-pointer"
                aria-label="Close My Bookings"
              >
                <XIcon size={20} />
              </button>
            </div>
          </div>

          {/* Subheader & Search for guest users */}
          {!currentUser && (
            <div className="px-6 py-3 bg-slate-900/60 border-b border-indigo-950">
              <form onSubmit={handleGuestSearch} className="flex gap-2">
                <input
                  type="email"
                  value={guestEmail}
                  onChange={(e) => setGuestEmail(e.target.value)}
                  placeholder="Enter email address to find bookings..."
                  className="flex-1 px-3 py-2 text-xs rounded-xl bg-slate-950 border border-slate-700 focus:border-amber-500 text-slate-100 outline-none"
                />
                <button
                  type="submit"
                  disabled={loading || !guestEmail.trim()}
                  className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-slate-950 font-bold text-xs flex items-center gap-1 transition-all cursor-pointer disabled:opacity-50"
                >
                  <SearchIcon size={14} />
                  Lookup
                </button>
              </form>
            </div>
          )}

          {/* Filter Tabs */}
          <div className="px-6 py-2.5 bg-slate-950/40 border-b border-indigo-950 flex items-center gap-2">
            {(['ALL', 'CONFIRMED', 'CANCELLED', 'WAITLIST'] as const).map((tab) => (
              <button
                key={tab}
                type="button"
                onClick={() => setFilterTab(tab)}
                className={`px-3 py-1 rounded-lg text-xs font-semibold tracking-wide transition-all cursor-pointer ${
                  filterTab === tab
                    ? 'bg-amber-500/20 text-amber-400 border border-amber-500/40'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {tab === 'ALL'
                  ? 'All Bookings'
                  : tab === 'WAITLIST'
                  ? `Waitlist Queue (${waitlistEntries.length})`
                  : tab.charAt(0) + tab.slice(1).toLowerCase()}
              </button>
            ))}
          </div>

          {/* Main Bookings List or Waitlist List */}
          <div className="p-6 overflow-y-auto space-y-4 flex-1">
            {error && (
              <div className="p-3 rounded-xl border border-rose-500/40 bg-rose-950/40 text-rose-200 flex items-center gap-2 text-xs">
                <AlertCircleIcon size={16} />
                <span>{error}</span>
              </div>
            )}

            {filterTab === 'WAITLIST' ? (
              waitlistLoading ? (
                <div className="py-12 text-center text-slate-400 text-sm">
                  <RefreshCwIcon size={24} className="animate-spin mx-auto mb-2 text-cyan-400" />
                  Checking waitlist queue status...
                </div>
              ) : waitlistEntries.length === 0 ? (
                <div className="py-12 text-center text-slate-400 space-y-3">
                  <div className="p-4 rounded-full bg-slate-800/40 w-16 h-16 mx-auto flex items-center justify-center text-slate-500">
                    <ClockIcon size={32} />
                  </div>
                  <h4 className="text-base font-semibold text-slate-200">No active waitlist requests</h4>
                  <p className="text-xs text-slate-400 max-w-sm mx-auto">
                    When high-demand events sell out, you can join the queue to be allocated tickets automatically.
                  </p>
                  {onBrowseEvents && (
                    <button
                      type="button"
                      onClick={() => {
                        onClose();
                        onBrowseEvents();
                      }}
                      className="mt-2 px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white font-bold text-xs transition-colors cursor-pointer"
                    >
                      Browse Events
                    </button>
                  )}
                </div>
              ) : (
                waitlistEntries.map((w) => {
                  const isPromoted = w.status === 'PROMOTED';
                  const isPending = w.status === 'PENDING';
                  const isExpired = w.status === 'EXPIRED';

                  return (
                    <div
                      key={w.id}
                      className={`p-5 rounded-2xl border transition-all ${
                        isPromoted
                          ? 'bg-gradient-to-br from-emerald-950/70 via-slate-900 to-cyan-950/70 border-emerald-500/60 shadow-xl shadow-emerald-950/50'
                          : 'bg-slate-950/60 border-slate-800'
                      }`}
                    >
                      <div className="flex items-start justify-between gap-3 mb-3">
                        <div>
                          <span className="text-[10px] uppercase font-bold tracking-wider text-cyan-400 block mb-0.5">
                            Priority Queue Request
                          </span>
                          <h4 className="text-base font-bold text-slate-100">{w.eventTitle || 'Event'}</h4>
                          <p className="text-xs text-slate-400">
                            Requested by: {w.customerName} ({w.customerEmail})
                          </p>
                        </div>
                        <span
                          className={`px-3 py-1 rounded-full text-xs font-bold tracking-wide uppercase ${
                            isPromoted
                              ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 animate-pulse'
                              : isExpired
                              ? 'bg-slate-800 text-slate-400 border border-slate-700'
                              : 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40'
                          }`}
                        >
                          {w.status}
                        </span>
                      </div>

                      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 text-xs bg-slate-900/60 p-3 rounded-xl border border-slate-800 mb-3">
                        <div>
                          <span className="block text-slate-400 text-[10px] uppercase">Queue Position</span>
                          <span className="font-mono font-bold text-sm text-cyan-300">
                            {isPromoted ? 'Allocated! 🎉' : `#${w.position || 1} in line`}
                          </span>
                        </div>
                        <div>
                          <span className="block text-slate-400 text-[10px] uppercase">Seats Needed</span>
                          <span className="font-bold text-slate-200">{w.quantity} seat{w.quantity > 1 ? 's' : ''}</span>
                        </div>
                        <div>
                          <span className="block text-slate-400 text-[10px] uppercase">Tier</span>
                          <span className="text-slate-200">{w.tierName || 'Standard'}</span>
                        </div>
                      </div>

                      {isPromoted && (
                        <div className="p-3.5 rounded-xl bg-emerald-950/60 border border-emerald-500/40 text-xs text-emerald-200 space-y-1.5 mb-3">
                          <div className="flex items-center gap-2 font-semibold">
                            <CheckIcon size={16} className="text-emerald-400" />
                            <span>Seats successfully reserved for you! 15-Minute Purchase Window.</span>
                          </div>
                          {w.expiresAt && (
                            <div className="flex items-center gap-1.5 text-slate-300 text-[11px]">
                              <ClockIcon size={13} className="text-amber-400" />
                              <span>Expires at: {new Date(w.expiresAt).toLocaleTimeString()}</span>
                            </div>
                          )}
                        </div>
                      )}

                      <div className="flex justify-end items-center gap-2">
                        {isPromoted && (
                          <button
                            type="button"
                            onClick={() => handleClaimWaitlist(w.id)}
                            disabled={claimingWaitlistId === w.id}
                            className="px-4 py-2 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs flex items-center gap-1.5 transition-all shadow-lg shadow-emerald-900/30 cursor-pointer disabled:opacity-50"
                          >
                            <CheckIcon size={14} />
                            {claimingWaitlistId === w.id ? 'Claiming Hold...' : 'Claim & Confirm Ticket'}
                          </button>
                        )}
                        {isPending && (
                          <button
                            type="button"
                            onClick={() => handleCancelWaitlist(w.id)}
                            className="px-3 py-1.5 rounded-xl border border-rose-500/30 hover:border-rose-500 bg-rose-950/20 hover:bg-rose-900/40 text-rose-300 text-xs font-semibold flex items-center gap-1.5 transition-colors cursor-pointer"
                          >
                            <TrashIcon size={14} />
                            Leave Waitlist
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })
              )
            ) : loading ? (
              <div className="py-12 text-center text-slate-400 text-sm">
                <RefreshCwIcon size={24} className="animate-spin mx-auto mb-2 text-amber-400" />
                Retrieving your reservations...
              </div>
            ) : filteredBookings.length === 0 ? (
              <div className="py-12 text-center text-slate-400 space-y-3">
                <div className="p-4 rounded-full bg-slate-800/40 w-16 h-16 mx-auto flex items-center justify-center text-slate-500">
                  <TicketIcon size={32} />
                </div>
                <h4 className="text-base font-semibold text-slate-200">No bookings found</h4>
                <p className="text-xs text-slate-400 max-w-sm mx-auto">
                  {currentUser
                    ? "You haven't reserved any tickets yet. Explore upcoming cinema, concerts, and live summits!"
                    : "Enter the email used during checkout above to retrieve your bookings."}
                </p>
                {onBrowseEvents && (
                  <button
                    type="button"
                    onClick={() => {
                      onClose();
                      onBrowseEvents();
                    }}
                    className="mt-2 px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-slate-950 font-bold text-xs transition-colors cursor-pointer"
                  >
                    Browse Events
                  </button>
                )}
              </div>
            ) : (
              filteredBookings.map((b) => {
                const isConfirmed = b.status === 'CONFIRMED';
                const isCancelled = b.status === 'CANCELLED';

                return (
                  <div
                    key={b.bookingId || b.bookingReference}
                    className="p-5 rounded-2xl bg-[#1B1B30] border border-indigo-900/60 hover:border-indigo-600/60 transition-all space-y-4 shadow-lg"
                  >
                    {/* Top Row: Title, Date, Status */}
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-indigo-950 pb-3">
                      <div>
                        <span className="text-[10px] font-mono uppercase tracking-wider text-amber-400">
                          {b.tierName || 'Standard Admission'}
                        </span>
                        <h4 className="text-base font-bold text-slate-100 font-display">
                          {b.eventTitle || 'Event Reservation'}
                        </h4>
                        <span className="text-xs text-slate-400">
                          Booked on {new Date(b.createdAt).toLocaleDateString()}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span
                          className={`px-2.5 py-0.5 rounded-full text-[11px] font-bold uppercase tracking-wide ${
                            isConfirmed
                              ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
                              : isCancelled
                              ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40'
                              : 'bg-amber-500/20 text-amber-300 border border-amber-500/40'
                          }`}
                        >
                          {b.status}
                        </span>
                      </div>
                    </div>

                    {/* Booking Meta Details */}
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs bg-slate-900/60 p-3 rounded-xl border border-slate-800">
                      <div>
                        <span className="block text-slate-400 text-[10px] uppercase">Reference</span>
                        <button
                          type="button"
                          onClick={() => handleCopyReference(b.bookingReference)}
                          className="font-mono font-bold text-amber-300 flex items-center gap-1 hover:text-amber-200 cursor-pointer"
                          title="Click to copy"
                        >
                          {b.bookingReference}
                          {copiedRef === b.bookingReference ? (
                            <CheckIcon size={12} className="text-emerald-400" />
                          ) : (
                            <CopyIcon size={12} className="text-slate-400" />
                          )}
                        </button>
                      </div>
                      <div>
                        <span className="block text-slate-400 text-[10px] uppercase">Total Paid</span>
                        <span className="font-bold text-slate-100">
                          {formatCurrency(b.totalPrice, b.currency || 'USD')}
                        </span>
                      </div>
                      <div>
                        <span className="block text-slate-400 text-[10px] uppercase">Seats ({b.seats.length})</span>
                        <span className="font-mono text-slate-200">
                          {b.seats.join(', ')}
                        </span>
                      </div>
                      <div>
                        <span className="block text-slate-400 text-[10px] uppercase">Recipient</span>
                        <span className="text-slate-200 truncate block" title={b.customerEmail}>
                          {b.customerName}
                        </span>
                      </div>
                    </div>

                    {/* Corporate Billing Details (Milestone 26) */}
                    {b.companyName && (
                      <div className="text-xs bg-indigo-950/40 px-3 py-1.5 rounded-lg border border-indigo-800/40 flex items-center justify-between">
                        <span className="text-slate-400">Billed To: <strong className="text-slate-200">{b.companyName}</strong></span>
                        {b.taxId && <span className="font-mono text-cyan-300 text-[11px]">Tax ID: {b.taxId}</span>}
                      </div>
                    )}

                    {/* Tickets / QR Passes Collection */}
                    {b.tickets && b.tickets.length > 0 && (
                      <div className="space-y-2 pt-1">
                        <span className="text-[11px] uppercase font-semibold text-slate-400 tracking-wider flex items-center gap-1">
                          <QrCodeIcon size={14} className="text-amber-400" />
                          Digital Entry Passes
                        </span>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                          {b.tickets.map((t) => (
                            <div
                              key={t.ticketId || t.ticketCode}
                              className="p-2.5 rounded-xl bg-slate-950/70 border border-slate-800 flex items-center justify-between"
                            >
                              <div>
                                <span className="font-mono font-bold text-xs text-amber-300 block">
                                  {t.seatLabel}
                                </span>
                                <span className="font-mono text-[10px] text-slate-400">
                                  {t.ticketCode}
                                </span>
                              </div>
                              <div className="flex items-center gap-1.5">
                                {t.status === 'VALID' && (
                                  <button
                                    type="button"
                                    onClick={() => setTransferTicketState({ booking: b, ticket: t })}
                                    className="px-2.5 py-1 rounded-lg bg-amber-900/40 hover:bg-amber-800 text-amber-300 hover:text-white text-xs font-semibold flex items-center gap-1 transition-colors cursor-pointer border border-amber-500/30"
                                    title="Transfer ticket to friend"
                                  >
                                    <Share2Icon size={12} />
                                    Transfer
                                  </button>
                                )}
                                {t.status === 'TRANSFERRED' && (
                                  <span className="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-800 text-slate-400 border border-slate-700">
                                    TRANSFERRED
                                  </span>
                                )}
                                <button
                                  type="button"
                                  onClick={() => setActiveTicket({ booking: b, ticket: t })}
                                  className="px-2.5 py-1 rounded-lg bg-indigo-900/60 hover:bg-indigo-800 text-indigo-200 hover:text-white text-xs font-semibold flex items-center gap-1 transition-colors cursor-pointer"
                                >
                                  <QrCodeIcon size={12} />
                                  View Pass
                                </button>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Booking Card Actions */}
                    {isConfirmed && (
                      <div className="flex justify-end items-center gap-2 pt-2">
                        <button
                          type="button"
                          onClick={() => downloadInvoicePdf(b.bookingId || b.bookingReference, b.bookingReference)}
                          className="px-3 py-1.5 rounded-xl border border-cyan-500/30 hover:border-cyan-400 bg-cyan-950/30 hover:bg-cyan-900/40 text-cyan-300 text-xs font-semibold flex items-center gap-1.5 transition-colors cursor-pointer"
                          data-testid="download-invoice-btn"
                          title="Download Official Tax Invoice & Admission Receipt (PDF)"
                        >
                          <FileTextIcon size={14} />
                          <span>Tax Invoice (PDF)</span>
                        </button>
                        <button
                          type="button"
                          onClick={() => handleOpenCancelConfirm(b.bookingId || b.bookingReference)}
                          className="px-3 py-1.5 rounded-xl border border-rose-500/30 hover:border-rose-500 bg-rose-950/20 hover:bg-rose-900/40 text-rose-300 text-xs font-semibold flex items-center gap-1.5 transition-colors cursor-pointer"
                        >
                          <TrashIcon size={14} />
                          Cancel Reservation
                        </button>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </div>
      </div>

      {/* Ticket Pass Modal */}
      {activeTicket && (
        <TicketPassModal
          booking={activeTicket.booking}
          ticket={activeTicket.ticket}
          isOpen={Boolean(activeTicket)}
          onClose={() => setActiveTicket(null)}
          onTransferTicket={(b, t) => setTransferTicketState({ booking: b, ticket: t })}
          onTicketStatusUpdated={(code, newStatus) => {
            setBookings((prev) =>
              prev.map((b) => ({
                ...b,
                tickets: b.tickets.map((t) =>
                  t.ticketCode === code ? { ...t, status: newStatus } : t
                ),
              }))
            );
          }}
        />
      )}

      {/* Ticket Transfer Modal */}
      {transferTicketState && (
        <TicketTransferModal
          isOpen={Boolean(transferTicketState)}
          booking={transferTicketState.booking}
          ticket={transferTicketState.ticket}
          currentUser={currentUser}
          onClose={() => setTransferTicketState(null)}
          onTicketTransferred={async () => {
            setTransferTicketState(null);
            await loadBookings();
          }}
        />
      )}

      {/* Cancel Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={isCancelConfirmOpen}
        title="Cancel Reservation?"
        message="Are you sure you want to cancel this booking? This will invoke the inventory compensation saga, immediately releasing your reserved seats back into the available pool."
        confirmLabel={actionInProgress ? 'Cancelling...' : 'Yes, Cancel Reservation'}
        cancelLabel="Keep Reservation"
        confirmVariant="danger"
        loading={actionInProgress}
        onConfirm={handleExecuteCancel}
        onCancel={() => {
          setIsCancelConfirmOpen(false);
          setCancellingBookingId(null);
        }}
      />
    </>
  );
};
