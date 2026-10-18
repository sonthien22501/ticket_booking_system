import React, { useState } from 'react';
import { BookingResponse, TicketItem, VerifyTicketResult } from '../types';
import { QRCodeDisplay } from './QRCodeDisplay';
import { verifyTicket } from '../services/api';
import {
  XIcon,
  TicketIcon,
  CheckCircleIcon,
  AlertCircleIcon,
  ScanIcon,
  Share2Icon,
} from './Icons';

interface TicketPassModalProps {
  booking: BookingResponse;
  ticket: TicketItem;
  isOpen: boolean;
  onClose: () => void;
  onTicketStatusUpdated?: (ticketCode: string, newStatus: string) => void;
  onTransferTicket?: (booking: BookingResponse, ticket: TicketItem) => void;
}

export const TicketPassModal: React.FC<TicketPassModalProps> = ({
  booking,
  ticket,
  isOpen,
  onClose,
  onTicketStatusUpdated,
  onTransferTicket,
}) => {
  const [currentStatus, setCurrentStatus] = useState<string>(ticket.status || 'VALID');
  const [checkingIn, setCheckingIn] = useState<boolean>(false);
  const [scanResult, setScanResult] = useState<VerifyTicketResult | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSimulateScan = async (action: 'VERIFY' | 'CHECK_IN') => {
    setCheckingIn(true);
    setScanResult(null);
    setScanError(null);
    try {
      const result = await verifyTicket({
        ticketCode: ticket.ticketCode,
        eventId: booking.eventId,
        action,
      });
      setScanResult(result);
      if (result.status && result.status !== currentStatus) {
        setCurrentStatus(result.status);
        if (onTicketStatusUpdated) {
          onTicketStatusUpdated(ticket.ticketCode, result.status);
        }
      }
    } catch (err: unknown) {
      const msg = (err as { message?: string })?.message || 'Verification failed';
      setScanError(msg);
    } finally {
      setCheckingIn(false);
    }
  };

  const isCancelled = booking.status === 'CANCELLED' || currentStatus === 'CANCELLED';
  const isUsed = currentStatus === 'USED';

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md"
      role="dialog"
      aria-modal="true"
      aria-labelledby="ticket-pass-title"
    >
      <div className="relative w-full max-w-md bg-[#1B1B30] border border-indigo-700/60 rounded-2xl shadow-2xl overflow-hidden text-slate-100 animate-in fade-in zoom-in-95 duration-200">
        {/* Pass Header Banner */}
        <div className="bg-gradient-to-r from-indigo-950 via-slate-900 to-indigo-900 px-6 py-5 border-b border-indigo-800/40 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-400">
              <TicketIcon size={22} />
            </div>
            <div>
              <span className="text-[11px] font-semibold tracking-widest text-amber-400 uppercase">
                Digital Boarding Pass
              </span>
              <h3 id="ticket-pass-title" className="text-lg font-bold text-slate-100 font-display line-clamp-1">
                {booking.eventTitle || 'Cinema & Concert Event'}
              </h3>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors cursor-pointer"
            aria-label="Close Ticket Pass"
          >
            <XIcon size={20} />
          </button>
        </div>

        {/* Pass Body Content */}
        <div className="p-6 space-y-6">
          {/* Status Badge */}
          <div className="flex items-center justify-between">
            <span className="text-xs uppercase font-medium text-slate-400 tracking-wider">
              Admission Status
            </span>
            <span
              className={`px-3 py-1 rounded-full text-xs font-bold tracking-wide uppercase ${
                isCancelled
                  ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40'
                  : isUsed
                  ? 'bg-purple-500/20 text-purple-300 border border-purple-500/40'
                  : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
              }`}
            >
              {isCancelled ? 'CANCELLED' : isUsed ? 'CHECKED IN / USED' : 'VALID ENTRY'}
            </span>
          </div>

          {/* Seat & Passenger Info */}
          <div className="grid grid-cols-2 gap-4 bg-slate-900/60 p-4 rounded-xl border border-slate-800">
            <div>
              <span className="block text-[11px] uppercase text-slate-400">Assigned Seat</span>
              <span className="text-lg font-bold font-mono text-amber-400">
                {ticket.seatLabel}
              </span>
            </div>
            <div>
              <span className="block text-[11px] uppercase text-slate-400">Tier / Section</span>
              <span className="text-sm font-semibold text-slate-200">
                {booking.tierName || 'Standard Admission'}
              </span>
            </div>
            <div>
              <span className="block text-[11px] uppercase text-slate-400">Attendee</span>
              <span className="text-sm font-semibold text-slate-200 line-clamp-1">
                {booking.customerName}
              </span>
            </div>
            <div>
              <span className="block text-[11px] uppercase text-slate-400">Booking Ref</span>
              <span className="text-xs font-mono font-semibold text-slate-300">
                {booking.bookingReference}
              </span>
            </div>
          </div>

          {/* Perforated Divider */}
          <div className="relative flex items-center justify-center my-4">
            <div className="absolute -left-9 w-6 h-6 rounded-full bg-[#0F0F23] border-r border-indigo-700/60" />
            <div className="w-full border-t-2 border-dashed border-slate-700/60" />
            <div className="absolute -right-9 w-6 h-6 rounded-full bg-[#0F0F23] border-l border-indigo-700/60" />
          </div>

          {/* QR Code & Scannable Code Area */}
          <div className="flex flex-col items-center justify-center space-y-3">
            <QRCodeDisplay value={ticket.ticketCode} size={150} />
            <div className="text-center space-y-1">
              <span className="text-xs uppercase tracking-widest text-slate-400">
                Gate Entry Code
              </span>
              <p className="font-mono text-sm tracking-wider font-bold text-amber-300">
                {ticket.ticketCode}
              </p>
            </div>
          </div>

          {/* Scan result banner if present */}
          {scanResult && (
            <div
              className={`p-3 rounded-xl border flex items-center gap-2 text-xs ${
                scanResult.valid
                  ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-200'
                  : 'bg-rose-950/40 border-rose-500/40 text-rose-200'
              }`}
            >
              {scanResult.valid ? <CheckCircleIcon size={16} /> : <AlertCircleIcon size={16} />}
              <span>{scanResult.message}</span>
            </div>
          )}

          {scanError && (
            <div className="p-3 rounded-xl border border-rose-500/40 bg-rose-950/40 text-rose-200 flex items-center gap-2 text-xs">
              <AlertCircleIcon size={16} />
              <span>{scanError}</span>
            </div>
          )}

          {/* Action Buttons */}
          <div className="pt-2 flex flex-col sm:flex-row gap-2">
            {!isCancelled && !isUsed && (
              <button
                type="button"
                onClick={() => handleSimulateScan('CHECK_IN')}
                disabled={checkingIn}
                className="flex-1 py-2.5 px-4 rounded-xl bg-amber-600 hover:bg-amber-500 text-slate-950 font-bold text-xs uppercase tracking-wide flex items-center justify-center gap-2 transition-all shadow-lg shadow-amber-600/20 cursor-pointer disabled:opacity-50"
              >
                <ScanIcon size={16} />
                {checkingIn ? 'Verifying...' : 'Simulate Gate Check-In'}
              </button>
            )}
            {!isCancelled && !isUsed && currentStatus === 'VALID' && onTransferTicket && (
              <button
                type="button"
                onClick={() => {
                  onClose();
                  onTransferTicket(booking, ticket);
                }}
                className="py-2.5 px-4 rounded-xl bg-indigo-900/60 hover:bg-indigo-800 text-amber-300 font-semibold text-xs flex items-center justify-center gap-1.5 transition-colors cursor-pointer border border-amber-500/30"
              >
                <Share2Icon size={14} />
                Transfer
              </button>
            )}
            <button
              type="button"
              onClick={() => window.print()}
              className="py-2.5 px-4 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold text-xs transition-colors cursor-pointer"
            >
              Print Pass
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
