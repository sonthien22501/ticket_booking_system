import React, { useState, useEffect } from 'react';
import { EventItem, SeatItem, User, VoucherValidationResult, EventAddon, SelectedAddonItem } from '../types';
import { UserIcon, MailIcon, CreditCardIcon, AlertCircleIcon, RefreshCwIcon, CheckCircleIcon, TagIcon, XIcon, BuildingOfficeIcon, SparklesIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';
import { ConfirmationDialog } from './ConfirmationDialog';
import { validateVoucher, fetchEventAddons } from '../services/api';
import { formatCurrency, calculateTax } from '../services/currency';

interface CheckoutFormProps {
  event: EventItem;
  selectedSeats: SeatItem[];
  conflictError: string | null;
  conflictingSeats: string[];
  submitting: boolean;
  onSubmit: (
    customerName: string,
    customerEmail: string,
    voucherCode?: string,
    currency?: string,
    corporateDetails?: { companyName?: string; taxId?: string; billingAddress?: string },
    addons?: SelectedAddonItem[]
  ) => void;
  onRefreshMap: () => void;
  currentUser?: User | null;
  currentCurrency?: string;
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
  currentCurrency = 'USD',
}) => {
  const [name, setName] = useState(currentUser?.name || '');
  const [email, setEmail] = useState(currentUser?.email || '');
  const [nameError, setNameError] = useState('');
  const [emailError, setEmailError] = useState('');
  const [isPaymentConfirmOpen, setIsPaymentConfirmOpen] = useState(false);

  // VIP Add-ons state (Milestone 29)
  const [availableAddons, setAvailableAddons] = useState<EventAddon[]>([]);
  const [selectedAddons, setSelectedAddons] = useState<Record<string, number>>({});
  const [loadingAddons, setLoadingAddons] = useState(false);

  // Voucher / Coupon state (Milestone 25)
  const [voucherCodeInput, setVoucherCodeInput] = useState('');
  const [appliedVoucher, setAppliedVoucher] = useState<VoucherValidationResult | null>(null);
  const [voucherError, setVoucherError] = useState<string | null>(null);
  const [validatingVoucher, setValidatingVoucher] = useState(false);

  // Corporate Billing state (Milestone 26)
  const [showCorporateFields, setShowCorporateFields] = useState(false);
  const [companyName, setCompanyName] = useState('');
  const [taxId, setTaxId] = useState('');
  const [billingAddress, setBillingAddress] = useState('');

  useEffect(() => {
    if (currentUser) {
      setName((prev) => prev || currentUser.name);
      setEmail((prev) => prev || currentUser.email);
    }
  }, [currentUser]);

  // Load VIP Add-ons for current event
  useEffect(() => {
    let isMounted = true;
    setLoadingAddons(true);
    fetchEventAddons(event.id)
      .then((addons) => {
        if (isMounted) {
          setAvailableAddons(addons || []);
        }
      })
      .catch((err) => {
        console.warn('Could not load addons for event:', err);
      })
      .finally(() => {
        if (isMounted) setLoadingAddons(false);
      });
    return () => {
      isMounted = false;
    };
  }, [event.id]);

  const handleAddonQuantityChange = (addonId: string, delta: number) => {
    const addon = availableAddons.find((a) => a.id === addonId);
    if (!addon) return;

    setSelectedAddons((prev) => {
      const currentQty = prev[addonId] || 0;
      const nextQty = Math.max(0, currentQty + delta);
      const maxLimit = addon.maxQuantityPerBooking && addon.maxQuantityPerBooking > 0 ? addon.maxQuantityPerBooking : 10;
      const effectiveMax = addon.availableInventory >= 0 ? Math.min(maxLimit, addon.availableInventory) : maxLimit;

      if (nextQty > effectiveMax) {
        return prev;
      }

      if (nextQty === 0) {
        const copy = { ...prev };
        delete copy[addonId];
        return copy;
      }
      return { ...prev, [addonId]: nextQty };
    });
  };

  const ticketsSubtotal = selectedSeats.reduce((sum, s) => sum + (s.price || 0), 0);
  const addonsSubtotal = availableAddons.reduce((sum, a) => {
    const qty = selectedAddons[a.id] || 0;
    return sum + (a.price || 0) * qty;
  }, 0);
  const subtotal = ticketsSubtotal + addonsSubtotal;
  const discountAmount = appliedVoucher?.discountAmount || 0;
  const taxDetails = calculateTax(subtotal, discountAmount, currentCurrency);

  const getChosenAddonList = (): SelectedAddonItem[] => {
    return availableAddons
      .filter((a) => (selectedAddons[a.id] || 0) > 0)
      .map((a) => ({
        addonId: a.id,
        quantity: selectedAddons[a.id],
      }));
  };

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
    const chosenAddons = getChosenAddonList();
    onSubmit(
      name.trim(),
      email.trim(),
      appliedVoucher?.code,
      currentCurrency,
      showCorporateFields
        ? {
            companyName: companyName.trim(),
            taxId: taxId.trim(),
            billingAddress: billingAddress.trim(),
          }
        : undefined,
      chosenAddons.length > 0 ? chosenAddons : undefined
    );
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

      {/* VIP Add-ons, Merchandise & Concessions Section (Milestone 29) */}
      {(loadingAddons || availableAddons.length > 0) && (
        <div className="checkout-section" data-testid="vip-addons-section">
          <div className="flex items-center justify-between mb-2">
            <h3 className="section-title mb-0 flex items-center gap-1.5">
              <SparklesIcon size={18} className="text-amber-400" />
              <span>2. VIP Upgrades & Event Merchandise</span>
            </h3>
            <span className="text-xs text-amber-400/80 font-medium">Optional Extras</span>
          </div>
          <p className="text-xs text-slate-400 mb-3">
            Enhance your experience with priority parking, commemorative merch, and VIP lounge hospitality.
          </p>

          {loadingAddons ? (
            <div className="py-4 text-center text-xs text-slate-400 flex items-center justify-center gap-2">
              <LoadingSpinner size="sm" />
              <span>Loading add-ons...</span>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              {availableAddons.map((addon) => {
                const qty = selectedAddons[addon.id] || 0;
                const isSoldOut = addon.availableInventory === 0;
                const maxAllowed = addon.maxQuantityPerBooking && addon.maxQuantityPerBooking > 0 ? addon.maxQuantityPerBooking : 10;
                const reachedMax = qty >= maxAllowed || (addon.availableInventory >= 0 && qty >= addon.availableInventory);

                return (
                  <div
                    key={addon.id}
                    data-testid={`addon-item-${addon.id}`}
                    className={`p-3 rounded-xl border transition-all ${
                      qty > 0
                        ? 'bg-amber-950/20 border-amber-500/50 shadow-md shadow-amber-950/20'
                        : 'bg-slate-900/60 border-slate-800 hover:border-slate-700'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div>
                        <div className="flex items-center gap-1.5">
                          <span className="text-[10px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wider bg-slate-800 text-cyan-400 border border-slate-700">
                            {addon.category}
                          </span>
                          {isSoldOut && (
                            <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-rose-500/20 text-rose-400 border border-rose-500/40">
                              Sold Out
                            </span>
                          )}
                        </div>
                        <h4 className="text-sm font-bold text-slate-100 mt-1">{addon.name}</h4>
                        <p className="text-xs text-slate-400 mt-0.5 line-clamp-2">{addon.description}</p>
                      </div>
                      <span className="text-sm font-bold text-amber-300 whitespace-nowrap">
                        {formatCurrency(addon.price, currentCurrency)}
                      </span>
                    </div>

                    <div className="flex items-center justify-between mt-3 pt-2 border-t border-slate-800/80">
                      <span className="text-[11px] text-slate-400">
                        {addon.availableInventory >= 0 ? `${addon.availableInventory} in stock` : 'Available'}
                      </span>
                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          disabled={submitting || qty === 0}
                          onClick={() => handleAddonQuantityChange(addon.id, -1)}
                          data-testid={`addon-dec-${addon.id}`}
                          className="w-7 h-7 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-30 disabled:cursor-not-allowed flex items-center justify-center font-bold text-sm cursor-pointer transition-colors"
                          aria-label={`Decrease ${addon.name}`}
                        >
                          -
                        </button>
                        <span
                          data-testid={`addon-qty-${addon.id}`}
                          className="font-mono font-bold text-sm text-slate-100 min-w-[20px] text-center"
                        >
                          {qty}
                        </span>
                        <button
                          type="button"
                          disabled={submitting || isSoldOut || reachedMax}
                          onClick={() => handleAddonQuantityChange(addon.id, 1)}
                          data-testid={`addon-inc-${addon.id}`}
                          className="w-7 h-7 rounded-lg bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/40 disabled:opacity-30 disabled:cursor-not-allowed flex items-center justify-center font-bold text-sm cursor-pointer transition-colors"
                          aria-label={`Increase ${addon.name}`}
                        >
                          +
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Payment Simulation Section */}
      <div className="checkout-section">
        <h3 className="section-title">3. Payment Simulation</h3>
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
        <h3 className="section-title">4. Promotional Voucher</h3>
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

      {/* Corporate Tax Invoice & Business Details (Milestone 26) */}
      <div className="checkout-section">
        <div className="flex items-center justify-between">
          <h3 className="section-title mb-0">5. Corporate Expense & Tax Receipt</h3>
          <button
            type="button"
            onClick={() => setShowCorporateFields(!showCorporateFields)}
            className="text-xs font-semibold text-cyan-400 hover:text-cyan-300 flex items-center gap-1 cursor-pointer"
            data-testid="toggle-corporate-billing"
          >
            <BuildingOfficeIcon size={14} />
            <span>{showCorporateFields ? 'Hide Details' : '+ Add Business / Tax ID'}</span>
          </button>
        </div>

        {showCorporateFields && (
          <div className="mt-3 p-3.5 rounded-xl bg-slate-900/60 border border-slate-800 space-y-3 animate-in fade-in duration-150">
            <p className="text-xs text-slate-400">
              Provide company details to receive an audit-compliant PDF tax invoice with VAT/Sales tax breakdown for business expense claims.
            </p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div className="form-group">
                <label htmlFor="company-name" className="form-label text-xs">
                  Company / Organization Name
                </label>
                <input
                  id="company-name"
                  type="text"
                  placeholder="e.g. Acme Corp Ltd."
                  value={companyName}
                  onChange={(e) => setCompanyName(e.target.value)}
                  className="form-input text-xs"
                  data-testid="company-name-input"
                />
              </div>
              <div className="form-group">
                <label htmlFor="tax-id" className="form-label text-xs">
                  Tax / VAT Registration #
                </label>
                <input
                  id="tax-id"
                  type="text"
                  placeholder="e.g. EU-VAT-12345678"
                  value={taxId}
                  onChange={(e) => setTaxId(e.target.value)}
                  className="form-input text-xs"
                  data-testid="tax-id-input"
                />
              </div>
            </div>
            <div className="form-group">
              <label htmlFor="billing-address" className="form-label text-xs">
                Billing Address (Optional)
              </label>
              <input
                id="billing-address"
                type="text"
                placeholder="e.g. 100 Innovation Way, Suite 400"
                value={billingAddress}
                onChange={(e) => setBillingAddress(e.target.value)}
                className="form-input text-xs"
                data-testid="billing-address-input"
              />
            </div>
          </div>
        )}
      </div>

      {/* Order Summary Section */}
      <div className="checkout-section order-summary-section">
        <h3 className="section-title">6. Order Summary</h3>
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
          <span className="summary-label">Tickets:</span>
          <span className="summary-value">
            {selectedSeats.length} ticket(s) ({formatCurrency(ticketsSubtotal, currentCurrency)})
          </span>
        </div>
        {availableAddons
          .filter((a) => (selectedAddons[a.id] || 0) > 0)
          .map((a) => (
            <div key={a.id} className="summary-row text-amber-300">
              <span className="summary-label">
                + {a.name} (x{selectedAddons[a.id]}):
              </span>
              <span className="summary-value font-medium">
                +{formatCurrency(a.price * (selectedAddons[a.id] || 0), currentCurrency)}
              </span>
            </div>
          ))}
        <div className="summary-row">
          <span className="summary-label">Subtotal ({currentCurrency}):</span>
          <span className="summary-value">{formatCurrency(subtotal, currentCurrency)}</span>
        </div>
        {appliedVoucher && (
          <div className="summary-row text-emerald-400">
            <span className="summary-label flex items-center gap-1">
              <TagIcon size={12} />
              <span>Promo Discount ({appliedVoucher.code}):</span>
            </span>
            <span className="summary-value font-medium">-{formatCurrency(discountAmount, currentCurrency)}</span>
          </div>
        )}
        <div className="summary-row text-slate-300">
          <span className="summary-label">Sales Tax / VAT ({taxDetails.taxRatePercent}):</span>
          <span className="summary-value font-medium">+{taxDetails.taxAmountFormatted}</span>
        </div>
        <div className="summary-divider"></div>
        <div className="summary-row total-row">
          <span className="summary-label font-bold">Total Price ({currentCurrency}):</span>
          <span className="summary-value font-bold text-accent">{taxDetails.finalTotalFormatted}</span>
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
            `Confirm & Reserve ${selectedSeats.length} ${selectedSeats.length === 1 ? 'Seat' : 'Seats'} (${taxDetails.finalTotalFormatted})`
          )}
        </button>
      </div>

      {/* R24: Payment Authorization Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={isPaymentConfirmOpen}
        title="Authorize Payment & Reserve Tickets"
        message="Please verify your reservation and payment details before completing your order:"
        confirmLabel={`Authorize Payment (${taxDetails.finalTotalFormatted})`}
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
          {companyName && (
            <div className="summary-row">
              <span className="summary-label">Billed To (Company):</span>
              <span className="summary-value font-medium">{companyName} {taxId ? `(${taxId})` : ''}</span>
            </div>
          )}
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
          {availableAddons
            .filter((a) => (selectedAddons[a.id] || 0) > 0)
            .map((a) => (
              <div key={a.id} className="summary-row text-amber-300">
                <span className="summary-label">
                  + {a.name} (x{selectedAddons[a.id]}):
                </span>
                <span className="summary-value font-medium">
                  +{formatCurrency(a.price * (selectedAddons[a.id] || 0), currentCurrency)}
                </span>
              </div>
            ))}
          {appliedVoucher && (
            <div className="summary-row text-emerald-400">
              <span className="summary-label">Promo Discount ({appliedVoucher.code}):</span>
              <span className="summary-value font-medium">-{formatCurrency(discountAmount, currentCurrency)}</span>
            </div>
          )}
          <div className="summary-row text-slate-300">
            <span className="summary-label">Sales Tax / VAT ({taxDetails.taxRatePercent}):</span>
            <span className="summary-value font-medium">+{taxDetails.taxAmountFormatted}</span>
          </div>
          <div className="summary-row total-row">
            <span className="summary-label font-bold">Total Authorization:</span>
            <span className="summary-value font-bold text-accent">{taxDetails.finalTotalFormatted}</span>
          </div>
        </div>
      </ConfirmationDialog>
    </form>
  );
};
