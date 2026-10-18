import React from 'react';
import { SeatItem } from '../types';

interface SeatNodeProps {
  seat: SeatItem;
  isSelected: boolean;
  onToggle: (seat: SeatItem) => void;
}

export const SeatNode: React.FC<SeatNodeProps> = ({ seat, isSelected, onToggle }) => {
  const isHeld = seat.status === 'HOLD';
  const isBooked = seat.status === 'BOOKED' || seat.status === 'BLOCKED';
  const isDisabled = isHeld || isBooked;
  const isVip = seat.tierId?.toLowerCase().includes('vip') || seat.row === 'A' || seat.row === 'B';

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
  } else if (isHeld) {
    statusClass = 'seat-held';
  } else if (isBooked) {
    statusClass = 'seat-booked';
  }

  const tooltipText = isHeld
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || 'Tier'}): Temporarily Held`
    : isBooked
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || 'Tier'}): Booked / Sold Out`
    : isSelected
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || (isVip ? 'VIP' : 'General')}): Selected - $${seat.price?.toFixed(2) || '0.00'}`
    : `Seat ${seat.row}${seat.number} (${seat.tierName || (isVip ? 'VIP' : 'General')}): Available - $${seat.price?.toFixed(2) || '0.00'}`;

  return (
    <button
      type="button"
      className={`seat-node ${statusClass} ${isVip ? 'seat-vip' : ''}`}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      disabled={isDisabled}
      aria-label={tooltipText}
      title={tooltipText}
      tabIndex={isDisabled ? -1 : 0}
    >
      <span className="seat-label">{seat.number}</span>
    </button>
  );
};
