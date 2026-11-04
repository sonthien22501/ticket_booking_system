import React from 'react';
import { SeatItem } from '../types';
import { calculateStaggerDelay } from '../utils/motion';
import { formatCurrency } from '../services/currency';

interface SeatNodeProps {
  seat: SeatItem;
  isSelected: boolean;
  onToggle: (seat: SeatItem) => void;
  rowIndex?: number;
  colIndex?: number;
  currentCurrency?: string;
}

export const SeatNode: React.FC<SeatNodeProps> = ({
  seat,
  isSelected,
  onToggle,
  rowIndex = 0,
  colIndex = 0,
  currentCurrency = 'USD',
}) => {
  const isHeld = seat.status === 'HOLD';
  const isBooked = seat.status === 'BOOKED' || seat.status === 'BLOCKED';
  const isWaitlist = seat.status === 'WAITLIST';
  const isDisabled = isHeld || isBooked || isWaitlist;
  const isVip = seat.tierId?.toLowerCase().includes('vip') || seat.row === 'A' || seat.row === 'B';

  const waveDelayMs = calculateStaggerDelay(rowIndex * 4 + colIndex, 14, 420);

  const handleClick = () => {
    if (!isDisabled) {
      onToggle(seat);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isDisabled && (e.key === 'Enter' || e.key === ' ')) {
      e.preventDefault();
      onToggle(seat);
    }
  };

  let statusClass = 'seat-available';
  if (isSelected) {
    statusClass = 'seat-selected';
  } else if (isWaitlist) {
    statusClass = 'seat-waitlist';
  } else if (isHeld) {
    statusClass = 'seat-held';
  } else if (isBooked) {
    statusClass = 'seat-booked';
  }

  const tooltipText = isWaitlist
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || 'Tier'}): Waitlist Queue Active`
    : isHeld
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || 'Tier'}): Temporarily Held`
    : isBooked
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || 'Tier'}): Booked / Reserved`
    : isSelected
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || (isVip ? 'VIP' : 'General')}): Selected — ${formatCurrency(seat.price || 0, currentCurrency)}`
    : `Seat ${seat.row}${seat.number} (${seat.tierName || (isVip ? 'VIP' : 'General')}): Available — ${formatCurrency(seat.price || 0, currentCurrency)}`;

  return (
    <button
      type="button"
      className={`seat-node seat-wave-in ${statusClass} ${isVip ? 'seat-vip' : ''} ${!isDisabled ? 'cursor-pointer' : ''}`}
      style={{ '--seat-stagger': `${waveDelayMs}ms` } as React.CSSProperties}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      disabled={isDisabled}
      aria-label={tooltipText}
      title={tooltipText}
      tabIndex={isDisabled ? -1 : 0}
    >
      <span className="seat-label">{seat.number}</span>
      <span className="seat-glow-ring"></span>
      {isSelected && <span className="seat-ripple-ring" aria-hidden="true"></span>}
    </button>
  );
};
