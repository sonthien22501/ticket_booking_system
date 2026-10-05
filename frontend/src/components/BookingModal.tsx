import React, { useEffect } from 'react';
import { EventItem, SeatItem, BookingResponse } from '../types';
import { CheckoutForm } from './CheckoutForm';
import { BookingConfirmation } from './BookingConfirmation';
import { XIcon } from './Icons';

interface BookingModalProps {
  isOpen: boolean;
  event: EventItem;
  selectedSeats: SeatItem[];
  submitting: boolean;
  conflictError: string | null;
  conflictingSeats: string[];
  confirmedBooking: BookingResponse | null;
  onClose: () => void;
  onSubmitBooking: (customerName: string, customerEmail: string) => void;
  onRefreshMap: () => void;
  onResetBooking: () => void;
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
          <h2 className="modal-title">
            {confirmedBooking ? 'Reservation Receipt' : 'Complete Your Reservation'}
          </h2>
          {!submitting && (
            <button
              type="button"
              onClick={onClose}
              className="modal-btn-close"
              aria-label="Close modal"
            >
              <XIcon size={20} />
            </button>
          )}
        </div>

        <div className="modal-body">
          {confirmedBooking ? (
            <BookingConfirmation
              booking={confirmedBooking}
              onDone={() => {
                onResetBooking();
                onClose();
              }}
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
            />
          )}
        </div>
      </div>
    </div>
  );
};
