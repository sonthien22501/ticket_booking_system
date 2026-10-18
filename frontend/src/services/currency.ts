import { CurrencyInfo } from '../types';

export const SUPPORTED_CURRENCIES: Record<string, CurrencyInfo> = {
  USD: { code: 'USD', name: 'US Dollar', symbol: '$', rate: 1.0, taxRate: 0.085, decimals: 2 },
  EUR: { code: 'EUR', name: 'Euro', symbol: '€', rate: 0.92, taxRate: 0.20, decimals: 2 },
  GBP: { code: 'GBP', name: 'British Pound', symbol: '£', rate: 0.79, taxRate: 0.20, decimals: 2 },
  JPY: { code: 'JPY', name: 'Japanese Yen', symbol: '¥', rate: 155.0, taxRate: 0.10, decimals: 0 },
  CAD: { code: 'CAD', name: 'Canadian Dollar', symbol: 'CA$', rate: 1.38, taxRate: 0.13, decimals: 2 },
  AUD: { code: 'AUD', name: 'Australian Dollar', symbol: 'A$', rate: 1.52, taxRate: 0.10, decimals: 2 },
};

const STORAGE_KEY = 'ticketverse_currency';

/**
 * Retrieves the currently selected currency code from localStorage or defaults to 'USD'.
 */
export function getStoredCurrency(): string {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved && SUPPORTED_CURRENCIES[saved.toUpperCase()]) {
      return saved.toUpperCase();
    }
  } catch {
    // Fallback for SSR or disabled storage
  }
  return 'USD';
}

/**
 * Persists the chosen currency in localStorage.
 */
export function setStoredCurrency(code: string): void {
  try {
    localStorage.setItem(STORAGE_KEY, code.toUpperCase());
  } catch {
    // Ignore storage errors
  }
}

/**
 * Formats a base USD numeric amount into localized currency string.
 * Ensures zero-decimal precision for JPY and 2 decimal places for other currencies.
 */
export function formatCurrency(
  amountUSD: number,
  currencyCode: string = 'USD',
  overrideRates?: Record<string, number>
): string {
  if (isNaN(amountUSD) || amountUSD < 0) {
    amountUSD = 0;
  }

  const code = (currencyCode || 'USD').toUpperCase();
  const info = SUPPORTED_CURRENCIES[code] || SUPPORTED_CURRENCIES.USD;
  const rate = overrideRates && overrideRates[code] ? overrideRates[code] : info.rate;

  const converted = amountUSD * rate;

  if (info.decimals === 0) {
    const intVal = Math.round(converted);
    return `${info.symbol}${intVal.toLocaleString('en-US')}`;
  }

  return `${info.symbol}${converted.toFixed(2)}`;
}

/**
 * Computes localized sales tax / VAT based on chosen currency.
 * Guarantees tax is applied strictly AFTER subtracting voucher discounts.
 */
export function calculateTax(
  subtotalUSD: number,
  discountUSD: number = 0,
  currencyCode: string = 'USD'
): {
  taxRate: number;
  taxRatePercent: string;
  taxAmount: number;
  taxAmountFormatted: string;
  taxableAmount: number;
  taxableAmountFormatted: string;
  finalTotal: number;
  finalTotalFormatted: string;
  symbol: string;
} {
  const code = (currencyCode || 'USD').toUpperCase();
  const info = SUPPORTED_CURRENCIES[code] || SUPPORTED_CURRENCIES.USD;

  const taxableUSD = Math.max(0, subtotalUSD - discountUSD);
  const taxableConverted = taxableUSD * info.rate;
  const rawTax = taxableConverted * info.taxRate;

  let taxAmount = rawTax;
  let taxableAmount = taxableConverted;
  let finalTotal = taxableConverted + rawTax;

  if (info.decimals === 0) {
    taxAmount = Math.round(taxAmount);
    taxableAmount = Math.round(taxableAmount);
    finalTotal = Math.round(finalTotal);
  } else {
    taxAmount = Math.round(taxAmount * 100) / 100;
    taxableAmount = Math.round(taxableAmount * 100) / 100;
    finalTotal = Math.round(finalTotal * 100) / 100;
  }

  const format = (val: number) => {
    if (info.decimals === 0) {
      return `${info.symbol}${val.toLocaleString('en-US')}`;
    }
    return `${info.symbol}${val.toFixed(2)}`;
  };

  return {
    taxRate: info.taxRate,
    taxRatePercent: `${(info.taxRate * 100).toFixed(0)}%`,
    taxAmount,
    taxAmountFormatted: format(taxAmount),
    taxableAmount,
    taxableAmountFormatted: format(taxableAmount),
    finalTotal,
    finalTotalFormatted: format(finalTotal),
    symbol: info.symbol,
  };
}
