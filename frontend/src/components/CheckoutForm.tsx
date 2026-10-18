import React, { useState, useEffect } from 'react';
import { EventItem, SeatItem, User, VoucherValidationResult } from '../types';
import { UserIcon, MailIcon, CreditCardIcon, AlertCircleIcon, RefreshCwIcon, CheckCircleIcon, TagIcon, XIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';
import { ConfirmationDialog } from './ConfirmationDialog';
import { validateVoucher } from '../services/api';

interface CheckoutFormProps {
  event: EventItem;
  selectedSeats: SeatItem[];
  conflictError: string | null;
  conflictingSeats: string[];
  submitting: boolean;
  onSubmit: (customerName: string, customerEmail: string, voucherCode?: string) => void;
  onRefreshMap: () => void;
  currentUser?: User | null;
}

export const CheckoutForm: React.FC<CheckoutFormProps> = ({
  event,
  selectedSeats,
  conflictError,
  conflictingSeats,
  submitting,
  onSubmit,
  onRefreshMap,
  currentUser = null,
}) => {
  const [name, setName] = useState(currentUser?.name || '');
  const [email, setEmail] = useState(currentUser?.email || '');
  const [nameError, setNameError] = useState('');
  const [emailError, setEmailError] = useState('');
  const [isPaymentConfirmOpen, setIsPaymentConfirmOpen] = useState(false);

  // Voucher / Coupon state (Milestone 25)
  const [voucherCodeInput, setVoucherCodeInput] = useState('');
  const [appliedVoucher, setAppliedVoucher] = useState<VoucherValidationResult | null>(null);
  const [voucherError, setVoucherError] = useState<string | null>(null);
  const [validatingVoucher, setValidatingVoucher] = useState(false);

  useEffect(() => {
    if (currentUser) {
      setName((prev) => prev || currentUser.name);
      setEmail((prev) => prev || currentUser.email);
    }
  }, [currentUser]);

  const subtotal = selectedSeats.reduce((sum, s) => sum + (s.price || 0), 0);
  const discountAmount = appliedVoucher?.discountAmount || 0;
  const totalPrice = Math.max(0, subtotal - discountAmount);

  const handleApplyVoucher = async () => {
    const trimmed = voucherCodeInput.trim().toUpperCase();
    if (!trimmed) {
      setVoucherError('Please enter a voucher code');
      return;
    }
    setValidatingVoucher(true);
    setVoucherError(null);
    try {
      const res = await validateVoucher(trimmed, subtotal, event.id);
      if (res.valid) {
        setAppliedVoucher(res);
        setVoucherError(null);
      } else {
        setAppliedVoucher(null);
        setVoucherError(res.message || 'Invalid or expired voucher code');
      }
    } catch (err: any) {
      setAppliedVoucher(null);
      setVoucherError(err.message || 'Failed to validate voucher');
    } finally {
      setValidatingVoucher(false);
    }
  };

  const handleRemoveVoucher = () => {
    setAppliedVoucher(null);
    setVoucherCodeInput('');
    setVoucherError(null);
  };

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
      // R24: Prompt confirmation dialog before executing payment/booking state change
      setIsPaymentConfirmOpen(true);
    }
  };

  const handleConfirmPayment = () => {
    setIsPaymentConfirmOpen(false);
    onSubmit(name.trim(), email.trim(), appliedVoucher?.code);
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
            className="btn-conflict-refresh cursor-pointer"
          >
            <RefreshCwIcon size={14} className="mr-1" />
            Refresh Seat Map & Select Alternative
          </button>
        </div>
      )}

      {/* Customer Info Section */}
      <div className="checkout-section">
        <h3 className="section-title">1. Attendee Information</h3>
        {currentUser && (
          <div className="checkout-user-badge" data-testid="checkout-user-badge">
            <CheckCircleIcon size={16} className="text-emerald" />
            <span>Logged in as <strong>{currentUser.name}</strong> ({currentUser.email})</span>
          </div>
        )}
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

      {/* Promotional Voucher / Coupon Section (Milestone 25) */}
      <div className="checkout-section voucher-section">
        <h3 className="section-title">3. Promotional Voucher</h3>
        {appliedVoucher ? (
          <div className="voucher-applied-card flex items-center justify-between p-3 rounded-lg border border-emerald-500/30 bg-emerald-950/20">
            <div className="flex items-center gap-2">
              <span className="p-1.5 rounded-md bg-emerald-500/20 text-emerald-400">
                <TagIcon size={16} />
              </span>
              <div>
                <p className="text-sm font-semibold text-emerald-300 flex items-center gap-2">
                  <span>{appliedVoucher.code}</span>
                  <span className="text-xs px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300 font-mono">
                    {appliedVoucher.discountType === 'PERCENTAGE'
                      ? `-${appliedVoucher.discountValue ?? 0}%`
                      : `-$${(appliedVoucher.discountValue ?? 0).toFixed(2)}`}
                  </span>
                </p>
                <p className="text-xs text-emerald-400/80">
                  {appliedVoucher.message || `Saved $${discountAmount.toFixed(2)} on this order`}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={handleRemoveVoucher}
              className="text-xs text-neutral-400 hover:text-rose-400 transition-colors cursor-pointer flex items-center gap-1 p-1"
              aria-label="Remove voucher"
            >
              <XIcon size={14} />
              <span>Remove</span>
            </button>
          </div>
        ) : (
          <div>
            <div className="flex gap-2">
              <div className="input-with-icon flex-1">
                <TagIcon size={18} className="input-icon" />
                <input
                  type="text"
                  placeholder="e.g. EARLYBIRD20, SAVE10"
                  value={voucherCodeInput}
                  onChange={(e) => {
                    setVoucherCodeInput(e.target.value);
                    if (voucherError) setVoucherError(null);
                  }}
                  disabled={submitting || validatingVoucher}
                  className={`form-input uppercase ${voucherError ? 'input-error' : ''}`}
                />
              </div>
              <button
                type="button"
                onClick={handleApplyVoucher}
                disabled={submitting || validatingVoucher || !voucherCodeInput.trim()}
                className="px-4 py-2 rounded-lg bg-brand-accent/20 hover:bg-brand-accent/30 text-brand-accent text-sm font-semibold border border-brand-accent/30 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-1.5"
              >
                {validatingVoucher ? <LoadingSpinner size="sm" /> : 'Apply'}
              </button>
            </div>
            {voucherError && (
              <p className="error-message text-rose-400 text-xs mt-1 flex items-center gap-1">
                <AlertCircleIcon size={12} />
                <span>{voucherError}</span>
              </p>
            )}
            <p className="text-xs text-neutral-400 mt-1.5">
              Have a promo code? Enter it above for instant ticket discounts.
            </p>
          </div>
        )}
      </div>

      {/* Order Summary Section */}
      <div className="checkout-section order-summary-section">
        <h3 className="section-title">4. Order Summary</h3>
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
        {appliedVoucher && (
          <>
            <div className="summary-row">
              <span className="summary-label">Subtotal:</span>
              <span className="summary-value">${subtotal.toFixed(2)}</span>
            </div>
            <div className="summary-row text-emerald-400">
              <span className="summary-label flex items-center gap-1">
                <TagIcon size={12} />
                <span>Promo Discount ({appliedVoucher.code}):</span>
              </span>
              <span className="summary-value font-medium">-${discountAmount.toFixed(2)}</span>
            </div>
          </>
        )}
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
          className={`btn-submit-booking ${!submitting && selectedSeats.length > 0 ? 'cursor-pointer' : ''}`}
        >
          {submitting ? (
            <LoadingSpinner message="Locking seats & reserving..." size="sm" />
          ) : (
            `Confirm & Reserve ${selectedSeats.length} ${selectedSeats.length === 1 ? 'Seat' : 'Seats'} ($${totalPrice.toFixed(2)})`
          )}
        </button>
      </div>

      {/* R24: Payment Authorization Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={isPaymentConfirmOpen}
        title="Authorize Payment & Reserve Tickets"
        message="Please verify your reservation and payment details before completing your order:"
        confirmLabel={`Authorize Payment ($${totalPrice.toFixed(2)})`}
        cancelLabel="Modify Information"
        confirmVariant="primary"
        loading={submitting}
        onConfirm={handleConfirmPayment}
        onCancel={() => setIsPaymentConfirmOpen(false)}
      >
        <div className="payment-confirm-details">
          <div className="summary-row">
            <span className="summary-label">Attendee:</span>
            <span className="summary-value font-medium">{name} ({email})</span>
          </div>
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
          {appliedVoucher && (
            <div className="summary-row text-emerald-400">
              <span className="summary-label">Promo Discount ({appliedVoucher.code}):</span>
              <span className="summary-value font-medium">-${discountAmount.toFixed(2)}</span>
            </div>
          )}
          <div className="summary-row total-row">
            <span className="summary-label font-bold">Total Authorization:</span>
            <span className="summary-value font-bold text-accent">${totalPrice.toFixed(2)}</span>
          </div>
        </div>
      </ConfirmationDialog>
    </form>
  );
};
