import React, { useState } from 'react';
import { BookingResponse, TicketItem, TicketTransferResult, User } from '../types';
import { transferTicket } from '../services/api';
import {
  XIcon,
  Share2Icon,
  CheckCircleIcon,
  AlertCircleIcon,
  TicketIcon,
  MailIcon,
  UserIcon,
} from './Icons';

interface TicketTransferModalProps {
  isOpen: boolean;
  onClose: () => void;
  booking: BookingResponse;
  ticket: TicketItem;
  currentUser: User | null;
  onTicketTransferred?: (oldTicketCode: string, result: TicketTransferResult) => void;
}

export const TicketTransferModal: React.FC<TicketTransferModalProps> = ({
  isOpen,
  onClose,
  booking,
  ticket,
  currentUser,
  onTicketTransferred,
}) => {
  const [recipientName, setRecipientName] = useState<string>('');
  const [recipientEmail, setRecipientEmail] = useState<string>('');
  const [note, setNote] = useState<string>('');
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<TicketTransferResult | null>(null);

  if (!isOpen) return null;

  const handleTransfer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!recipientEmail.trim()) {
      setError('Recipient email is required.');
      return;
    }
    if (recipientEmail.trim().toLowerCase() === booking.customerEmail.toLowerCase()) {
      setError('You cannot transfer a ticket to your own email address.');
      return;
    }

    setError(null);
    setSubmitting(true);

    try {
      const res = await transferTicket({
        ticketCode: ticket.ticketCode,
        ticketId: ticket.ticketId,
        recipientEmail: recipientEmail.trim().toLowerCase(),
        recipientName: recipientName.trim() || recipientEmail.trim().split('@')[0],
        fromUserId: currentUser?.id,
        note: note.trim() || undefined,
      });

      setResult(res);
      if (onTicketTransferred) {
        onTicketTransferred(ticket.ticketCode, res);
      }
    } catch (err: unknown) {
      const msg = (err as { message?: string })?.message || 'Ticket transfer failed';
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
      aria-labelledby="ticket-transfer-title"
    >
      <div className="relative w-full max-w-md bg-[#161628] border border-amber-500/40 rounded-2xl shadow-2xl overflow-hidden text-slate-100">
        {/* Header */}
        <div className="bg-gradient-to-r from-amber-950/80 via-slate-900 to-indigo-950/80 px-6 py-5 border-b border-amber-800/40 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-400">
              <Share2Icon size={22} />
            </div>
            <div>
              <span className="text-[11px] font-semibold tracking-wider text-amber-400 uppercase">
                Digital Pass Transfer
              </span>
              <h3 id="ticket-transfer-title" className="text-lg font-bold text-slate-100 font-display line-clamp-1">
                Transfer Ticket
              </h3>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors cursor-pointer"
            aria-label="Close Transfer Modal"
          >
            <XIcon size={20} />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-5">
          {result ? (
            /* Success State */
            <div className="text-center space-y-5 py-3 animate-in zoom-in-95 duration-200">
              <div className="w-16 h-16 mx-auto rounded-full bg-emerald-500/10 border border-emerald-400/30 flex items-center justify-center text-emerald-400 shadow-lg shadow-emerald-500/10">
                <CheckCircleIcon size={36} />
              </div>

              <div className="space-y-1">
                <h4 className="text-xl font-bold text-slate-100">Ticket Transferred!</h4>
                <p className="text-sm text-slate-400">
                  Admission pass has been re-issued to <strong className="text-slate-200">{result.recipientName}</strong>.
                </p>
              </div>

              {/* Transfer Summary Pill */}
              <div className="p-4 rounded-xl bg-slate-800/60 border border-slate-700/60 text-left text-xs space-y-2">
                <div className="flex justify-between items-center text-slate-300">
                  <span className="text-slate-400">Event:</span>
                  <span className="font-semibold">{booking.eventTitle}</span>
                </div>
                <div className="flex justify-between items-center text-slate-300">
                  <span className="text-slate-400">Seat:</span>
                  <span className="font-mono text-cyan-300">{ticket.seatLabel}</span>
                </div>
                <div className="flex justify-between items-center text-slate-300">
                  <span className="text-slate-400">Recipient:</span>
                  <span className="font-medium text-amber-300">{result.recipientEmail}</span>
                </div>
                <div className="flex justify-between items-center text-slate-300">
                  <span className="text-slate-400">New Ticket Code:</span>
                  <span className="font-mono text-emerald-400">{result.newTicketCode}</span>
                </div>
                <div className="pt-2 border-t border-slate-700/60 text-[11px] text-amber-300/90">
                  🔒 Your original ticket code <code className="text-slate-400">({ticket.ticketCode})</code> has been permanently deactivated and will be rejected at the gate scanner.
                </div>
              </div>

              <button
                type="button"
                onClick={onClose}
                className="w-full py-2.5 px-4 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white font-medium text-sm transition-colors cursor-pointer shadow-lg shadow-emerald-900/30"
              >
                Close & Return to Passbook
              </button>
            </div>
          ) : (
            /* Transfer Form */
            <form onSubmit={handleTransfer} className="space-y-4">
              {/* Ticket Preview Card */}
              <div className="p-3.5 rounded-xl bg-slate-800/60 border border-slate-700/60 flex items-center justify-between text-xs">
                <div className="space-y-0.5">
                  <div className="font-bold text-slate-200 line-clamp-1">{booking.eventTitle}</div>
                  <div className="flex items-center gap-2 text-slate-400">
                    <TicketIcon size={14} className="text-amber-400" />
                    <span>{ticket.seatLabel}</span>
                    <span>•</span>
                    <span className="font-mono text-slate-300">{ticket.ticketCode}</span>
                  </div>
                </div>
                <span className="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                  VALID
                </span>
              </div>

              {/* Warning Notice */}
              <div className="p-3 rounded-xl bg-amber-950/40 border border-amber-500/40 text-amber-300 text-xs flex items-start gap-2.5">
                <AlertCircleIcon size={18} className="shrink-0 text-amber-400 mt-0.5" />
                <div className="space-y-0.5">
                  <span className="font-semibold block">Permanent Transfer:</span>
                  <p className="text-[11px] text-amber-200/80 leading-relaxed">
                    Once sent, your original QR code is invalidated immediately. The recipient will receive admission ownership under their email.
                  </p>
                </div>
              </div>

              {error && (
                <div className="p-3 rounded-xl bg-red-950/70 border border-red-500/50 flex items-center gap-2.5 text-xs text-red-200">
                  <AlertCircleIcon size={16} className="text-red-400 shrink-0" />
                  <span>{error}</span>
                </div>
              )}

              {/* Form Inputs */}
              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1">
                  Recipient Name
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
                    <UserIcon size={16} />
                  </div>
                  <input
                    type="text"
                    required
                    value={recipientName}
                    onChange={(e) => setRecipientName(e.target.value)}
                    placeholder="e.g. Sarah Connor"
                    className="w-full pl-9 pr-3 py-2 bg-slate-900 border border-slate-700 rounded-xl text-slate-100 text-sm focus:outline-none focus:border-amber-500"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1">
                  Recipient Email Address
                </label>
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
                    <MailIcon size={16} />
                  </div>
                  <input
                    type="email"
                    required
                    value={recipientEmail}
                    onChange={(e) => setRecipientEmail(e.target.value)}
                    placeholder="e.g. sarah.connor@cyberdyne.io"
                    className="w-full pl-9 pr-3 py-2 bg-slate-900 border border-slate-700 rounded-xl text-slate-100 text-sm focus:outline-none focus:border-amber-500"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1">
                  Gift / Transfer Note (Optional)
                </label>
                <input
                  type="text"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder="e.g. See you at Neon Symphony!"
                  className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-xl text-slate-100 text-sm focus:outline-none focus:border-amber-500"
                />
              </div>

              {/* Actions */}
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
                  className="flex-1 py-2.5 px-4 rounded-xl bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-white font-medium text-sm transition-colors flex items-center justify-center gap-2 cursor-pointer shadow-lg shadow-amber-900/30 font-semibold"
                >
                  <Share2Icon size={18} />
                  {submitting ? 'Transferring...' : 'Transfer Ticket'}
                </button>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};
