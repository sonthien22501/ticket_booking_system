import React, { useEffect } from 'react';
import { EventItem, SeatItem, BookingResponse, User } from '../types';
import { CheckoutForm } from './CheckoutForm';
import { BookingConfirmation } from './BookingConfirmation';
import { XIcon, CheckIcon } from './Icons';

interface BookingModalProps {
  isOpen: boolean;
  event: EventItem;
  selectedSeats: SeatItem[];
  submitting: boolean;
  conflictError: string | null;
  conflictingSeats: string[];
  confirmedBooking: BookingResponse | null;
  onClose: () => void;
  onSubmitBooking: (customerName: string, customerEmail: string, voucherCode?: string) => void;
  onRefreshMap: () => void;
  onResetBooking: () => void;
  currentUser?: User | null;
  onViewPasses?: () => void;
}

export const BookingModal: React.FC<BookingModalProps> = ({
  isOpen,
  event,
  selectedSeats,
  submitting,
  conflictError,
  conflictingSeats,
  confirmedBooking,
  onClose,
  onSubmitBooking,
  onRefreshMap,
  onResetBooking,
  currentUser = null,
  onViewPasses,
}) => {
  // Close on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !submitting) {
        onClose();
      }
    };
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
    }
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, submitting, onClose]);

  if (!isOpen) return null;

  return (
    <div className="modal-backdrop" onClick={() => !submitting && onClose()}>
      <div
        className="modal-container"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        <div className="modal-header">
          <div className="modal-header-titles">
            <h2 className="modal-title">
              {confirmedBooking ? 'Reservation Confirmed' : 'Complete Your Reservation'}
            </h2>
            <p className="modal-subtitle">
              {confirmedBooking
                ? 'Your tickets are digitally signed and confirmed.'
                : `${event.title} • ${selectedSeats.length} seat(s) selected`}
            </p>
          </div>
          {!submitting && (
            <button
              type="button"
              onClick={onClose}
              className="modal-btn-close cursor-pointer"
              aria-label="Close modal"
            >
              <XIcon size={20} />
            </button>
          )}
        </div>

        {/* 3-Step Progression Bar */}
        <div className="modal-step-progression" aria-label="Reservation Progress">
          <div className="step-item step-completed">
            <span className="step-circle"><CheckIcon size={12} /></span>
            <span className="step-label">1. Seats</span>
          </div>
          <div className="step-divider active"></div>
          <div className={`step-item ${confirmedBooking ? 'step-completed' : 'step-active'}`}>
            <span className="step-circle">{confirmedBooking ? <CheckIcon size={12} /> : '2'}</span>
            <span className="step-label">2. Review</span>
          </div>
          <div className={`step-divider ${confirmedBooking ? 'active' : ''}`}></div>
          <div className={`step-item ${confirmedBooking ? 'step-completed step-active' : ''}`}>
            <span className="step-circle">{confirmedBooking ? <CheckIcon size={12} /> : '3'}</span>
            <span className="step-label">3. Payment</span>
          </div>
        </div>

        <div className="modal-body">
          {confirmedBooking ? (
            <BookingConfirmation
              booking={confirmedBooking}
              onDone={() => {
                onResetBooking();
                onClose();
              }}
              onViewPasses={onViewPasses ? () => {
                onClose();
                onViewPasses();
              } : undefined}
            />
          ) : (
            <CheckoutForm
              event={event}
              selectedSeats={selectedSeats}
              conflictError={conflictError}
              conflictingSeats={conflictingSeats}
              submitting={submitting}
              onSubmit={onSubmitBooking}
              onRefreshMap={onRefreshMap}
              currentUser={currentUser}
            />
          )}
        </div>
      </div>
    </div>
  );
};
