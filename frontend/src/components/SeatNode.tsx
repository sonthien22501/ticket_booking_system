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
  const isWaitlist = seat.status === 'WAITLIST';
  const isDisabled = isHeld || isBooked || isWaitlist;
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
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || (isVip ? 'VIP' : 'General')}): Selected — $${seat.price?.toFixed(2) || '0.00'}`
    : `Seat ${seat.row}${seat.number} (${seat.tierName || (isVip ? 'VIP' : 'General')}): Available — $${seat.price?.toFixed(2) || '0.00'}`;

  return (
    <button
      type="button"
      className={`seat-node ${statusClass} ${isVip ? 'seat-vip' : ''} ${!isDisabled ? 'cursor-pointer' : ''}`}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      disabled={isDisabled}
      aria-label={tooltipText}
      title={tooltipText}
      tabIndex={isDisabled ? -1 : 0}
    >
      <span className="seat-label">{seat.number}</span>
      <span className="seat-glow-ring"></span>
    </button>
  );
};
