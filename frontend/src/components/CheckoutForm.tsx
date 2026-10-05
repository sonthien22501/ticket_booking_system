import React, { useState } from 'react';
import { EventItem, SeatItem } from '../types';
import { UserIcon, MailIcon, CreditCardIcon, AlertCircleIcon, RefreshCwIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';

interface CheckoutFormProps {
  event: EventItem;
  selectedSeats: SeatItem[];
  conflictError: string | null;
  conflictingSeats: string[];
  submitting: boolean;
  onSubmit: (customerName: string, customerEmail: string) => void;
  onRefreshMap: () => void;
}

export const CheckoutForm: React.FC<CheckoutFormProps> = ({
  event,
  selectedSeats,
  conflictError,
  conflictingSeats,
  submitting,
  onSubmit,
  onRefreshMap,
}) => {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [nameError, setNameError] = useState('');
  const [emailError, setEmailError] = useState('');

  const totalPrice = selectedSeats.reduce((sum, s) => sum + (s.price || 0), 0);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    let valid = true;
    if (!name.trim() || name.trim().length < 2) {
      setNameError('Please enter a valid customer name (at least 2 characters).');
      valid = false;
    } else {
      setNameError('');
    }

    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!email.trim() || !emailRegex.test(email.trim())) {
      setEmailError('Please enter a valid email address.');
      valid = false;
    } else {
      setEmailError('');
    }

    if (valid) {
      onSubmit(name.trim(), email.trim());
    }
  };

  return (
    <form onSubmit={handleSubmit} className="checkout-form">
      {/* 409 Conflict Banner if any */}
      {conflictError && (
        <div className="checkout-conflict-banner" role="alert">
          <div className="conflict-banner-header">
            <AlertCircleIcon size={20} className="text-conflict" />
            <h4 className="conflict-title">Seat Unavailable (409 Conflict)</h4>
          </div>
          <p className="conflict-text">
            {conflictError}
          </p>
          {conflictingSeats.length > 0 && (
            <p className="conflict-seats-list">
              Conflicted Seats: <strong>{conflictingSeats.join(', ')}</strong>
            </p>
          )}
          <button
            type="button"
            onClick={onRefreshMap}
            className="btn-conflict-refresh"
          >
            <RefreshCwIcon size={14} className="mr-1" />
            Refresh Seat Map & Select Alternative
          </button>
        </div>
      )}

      {/* Customer Info Section */}
      <div className="checkout-section">
        <h3 className="section-title">1. Attendee Information</h3>
        <div className="form-group">
          <label htmlFor="customer-name" className="form-label">
            Full Name <span className="text-required">*</span>
          </label>
          <div className="input-with-icon">
            <UserIcon size={18} className="input-icon" />
            <input
              id="customer-name"
              type="text"
              placeholder="e.g. Jane Doe"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                if (nameError) setNameError('');
              }}
              disabled={submitting}
              className={`form-input ${nameError ? 'input-error' : ''}`}
            />
          </div>
          {nameError && <p className="error-message">{nameError}</p>}
        </div>

        <div className="form-group">
          <label htmlFor="customer-email" className="form-label">
            Email Address <span className="text-required">*</span>
          </label>
          <div className="input-with-icon">
            <MailIcon size={18} className="input-icon" />
            <input
              id="customer-email"
              type="email"
              placeholder="e.g. jane.doe@example.com"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                if (emailError) setEmailError('');
              }}
              disabled={submitting}
              className={`form-input ${emailError ? 'input-error' : ''}`}
            />
          </div>
          {emailError && <p className="error-message">{emailError}</p>}
        </div>
      </div>

      {/* Payment Simulation Section */}
      <div className="checkout-section">
        <h3 className="section-title">2. Payment Simulation</h3>
        <div className="payment-method-card">
          <div className="payment-card-left">
            <CreditCardIcon size={24} className="text-brand-accent" />
            <div>
              <p className="payment-name">Simulated Payment Gateway</p>
              <p className="payment-sub">Test Mode: Instant Authorization</p>
            </div>
          </div>
          <span className="payment-badge">SIMULATED</span>
        </div>
      </div>

      {/* Order Summary Section */}
      <div className="checkout-section order-summary-section">
        <h3 className="section-title">3. Order Summary</h3>
        <div className="summary-row">
          <span className="summary-label">Event:</span>
          <span className="summary-value font-medium">{event.title}</span>
        </div>
        <div className="summary-row">
          <span className="summary-label">Venue:</span>
          <span className="summary-value">{event.venueName}</span>
        </div>
        <div className="summary-row">
          <span className="summary-label">Selected Seats:</span>
          <span className="summary-value font-mono">
            {selectedSeats.map((s) => `${s.row}${s.number}`).join(', ')}
          </span>
        </div>
        <div className="summary-row">
          <span className="summary-label">Quantity:</span>
          <span className="summary-value">{selectedSeats.length} ticket(s)</span>
        </div>
        <div className="summary-divider"></div>
        <div className="summary-row total-row">
          <span className="summary-label font-bold">Total Price:</span>
          <span className="summary-value font-bold text-accent">${totalPrice.toFixed(2)}</span>
        </div>
      </div>

      {/* Submit Button */}
      <div className="checkout-footer">
        <button
          type="submit"
          disabled={submitting || selectedSeats.length === 0}
          className="btn-submit-booking"
        >
          {submitting ? (
            <LoadingSpinner message="Locking seats & reserving..." size="sm" />
          ) : (
            `Confirm & Reserve ${selectedSeats.length} ${selectedSeats.length === 1 ? 'Seat' : 'Seats'} ($${totalPrice.toFixed(2)})`
          )}
        </button>
      </div>
    </form>
  );
};
