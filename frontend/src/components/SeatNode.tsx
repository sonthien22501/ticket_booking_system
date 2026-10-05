import React from 'react';
import { SeatItem } from '../types';

interface SeatNodeProps {
  seat: SeatItem;
  isSelected: boolean;
  onToggle: (seat: SeatItem) => void;
}

export const SeatNode: React.FC<SeatNodeProps> = ({ seat, isSelected, onToggle }) => {
  const isBooked = seat.status === 'BOOKED' || seat.status === 'HOLD' || seat.status === 'BLOCKED';
  const isVip = seat.tierId?.toLowerCase().includes('vip') || seat.row === 'A' || seat.row === 'B';

  const handleClick = () => {
    if (!isBooked) {
      onToggle(seat);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isBooked && (e.key === 'Enter' || e.key === ' ')) {
      e.preventDefault();
      onToggle(seat);
    }
  };

  let statusClass = 'seat-available';
  if (isBooked) {
    statusClass = 'seat-booked';
  } else if (isSelected) {
    statusClass = 'seat-selected';
  }

  const tooltipText = isBooked
    ? `Seat ${seat.row}${seat.number} (${seat.tierName || 'Reserved'}): Already Booked`
    : `Seat ${seat.row}${seat.number} (${seat.tierName || (isVip ? 'VIP' : 'General')}): $${seat.price?.toFixed(2) || '0.00'}`;

  return (
    <button
      type="button"
      className={`seat-node ${statusClass} ${isVip ? 'seat-vip' : ''}`}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      disabled={isBooked}
      aria-label={tooltipText}
      title={tooltipText}
      tabIndex={isBooked ? -1 : 0}
    >
      <span className="seat-label">{seat.number}</span>
    </button>
  );
};
