import React, { useState } from 'react';
import { BookingResponse, TicketItem } from '../types';
import { CheckCircleIcon, TicketIcon, QrCodeIcon, FileTextIcon, CalendarIcon } from './Icons';
import { TicketPassModal } from './TicketPassModal';
import { downloadInvoicePdf, downloadBookingCalendarICS } from '../services/api';
import { formatCurrency } from '../services/currency';

interface BookingConfirmationProps {
  booking: BookingResponse;
  onDone: () => void;
  onViewPasses?: () => void;
}

export const BookingConfirmation: React.FC<BookingConfirmationProps> = ({ booking, onDone, onViewPasses }) => {
  const [activeTicket, setActiveTicket] = useState<TicketItem | null>(null);
  const [downloadingPdf, setDownloadingPdf] = useState(false);

  const handleDownloadInvoice = async () => {
    setDownloadingPdf(true);
    try {
      await downloadInvoicePdf(booking.bookingId, booking.bookingReference);
    } catch (err) {
      console.error('Invoice download error:', err);
    } finally {
      setDownloadingPdf(false);
    }
  };

  return (
    <div className="booking-confirmation-view">
      <div className="confirmation-header">
        <div className="success-icon-badge">
          <CheckCircleIcon size={36} className="text-success" />
        </div>
        <h3 className="confirmation-title">Booking Confirmed!</h3>
        <p className="confirmation-subtitle">
          Your atomic seat reservation has been committed and verified.
        </p>
      </div>

      <div className="booking-ref-card">
        <span className="ref-label">Booking Reference</span>
        <span className="ref-code font-mono">{booking.bookingReference}</span>
        <span className="ref-status-badge status-confirmed">{booking.status}</span>
      </div>

      <div className="confirmation-details-card">
        <h4 className="card-section-title">Order Details</h4>
        <div className="details-grid">
          <div className="detail-item">
            <span className="detail-label">Customer Name</span>
            <span className="detail-value">{booking.customerName}</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Customer Email</span>
            <span className="detail-value">{booking.customerEmail}</span>
          </div>
          {booking.companyName && (
            <div className="detail-item">
              <span className="detail-label">Billed To (Company)</span>
              <span className="detail-value font-medium">
                {booking.companyName} {booking.taxId ? `(${booking.taxId})` : ''}
              </span>
            </div>
          )}
          <div className="detail-item">
            <span className="detail-label">Total Amount Paid</span>
            <span className="detail-value font-bold text-accent">
              {formatCurrency(booking.totalPrice, booking.currency || 'USD')}
            </span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Quantity</span>
            <span className="detail-value">{booking.quantity} Ticket(s)</span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Reserved Seats</span>
            <span className="detail-value font-mono">
              {booking.seats.join(', ')}
            </span>
          </div>
          <div className="detail-item">
            <span className="detail-label">Booking Date</span>
            <span className="detail-value">
              {new Date(booking.createdAt).toLocaleString()}
            </span>
          </div>
        </div>
      </div>

      <div className="digital-tickets-container">
        <h4 className="card-section-title">
          <TicketIcon size={18} className="mr-1 inline-icon" />
          Digital Tickets ({booking.tickets.length})
        </h4>

        <div className="tickets-list">
          {booking.tickets.map((ticket) => (
            <div key={ticket.ticketId} className="digital-ticket-card">
              <div className="ticket-card-header">
                <span className="ticket-seat-label">{ticket.seatLabel}</span>
                <span className="ticket-status-pill">{ticket.status}</span>
              </div>
              <div className="ticket-barcode-area">
                <div className="simulated-barcode">
                  || | ||| || |||| | || |||| | ||| ||
                </div>
                <div className="flex items-center justify-between mt-1">
                  <span className="ticket-code font-mono">{ticket.ticketCode}</span>
                  <button
                    type="button"
                    onClick={() => setActiveTicket(ticket)}
                    className="text-xs text-amber-400 hover:text-amber-300 flex items-center gap-1 font-semibold cursor-pointer"
                  >
                    <QrCodeIcon size={14} />
                    View QR Pass
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="confirmation-actions">
        {onViewPasses && (
          <button
            type="button"
            onClick={onViewPasses}
            className="btn-print-ticket cursor-pointer flex items-center justify-center gap-1.5"
          >
            <QrCodeIcon size={16} />
            My Passbook
          </button>
        )}
        <button
          type="button"
          onClick={handleDownloadInvoice}
          disabled={downloadingPdf}
          className="btn-print-ticket cursor-pointer flex items-center justify-center gap-1.5 border border-cyan-500/40 text-cyan-300 hover:bg-cyan-950/40"
          data-testid="confirmation-download-invoice-btn"
          title="Download Official Tax Invoice & Admission Receipt (PDF)"
        >
          <FileTextIcon size={16} />
          <span>{downloadingPdf ? 'Generating PDF...' : 'Tax Invoice (PDF)'}</span>
        </button>
        <button
          type="button"
          onClick={() => downloadBookingCalendarICS(booking.bookingId, booking.eventTitle)}
          className="btn-print-ticket cursor-pointer flex items-center justify-center gap-1.5 border border-amber-500/40 text-amber-300 hover:bg-amber-950/40"
          data-testid="confirmation-add-calendar-btn"
          title="Export Event to Apple, Google, or Outlook Calendar (.ics)"
        >
          <CalendarIcon size={16} />
          <span>Add to Calendar (.ics)</span>
        </button>
        <button
          type="button"
          onClick={() => window.print()}
          className="btn-print-ticket cursor-pointer"
        >
          Print Tickets / PDF
        </button>
        <button
          type="button"
          onClick={onDone}
          className="btn-done-booking cursor-pointer"
        >
          Book Another Event
        </button>
      </div>

      {activeTicket && (
        <TicketPassModal
          booking={booking}
          ticket={activeTicket}
          isOpen={Boolean(activeTicket)}
          onClose={() => setActiveTicket(null)}
        />
      )}
    </div>
  );
};
