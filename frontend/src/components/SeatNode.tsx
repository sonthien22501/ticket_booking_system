import React from 'react';
import { SeatItem } from '../types';
import { formatCurrency } from '../services/currency';

interface SeatNodeProps {
  seat: SeatItem;
  isSelected: boolean;
  onToggle: (seat: SeatItem) => void;
  tabIndex?: number;
  onFocusSeat?: (seatId: string) => void;
  currentCurrency?: string;
}

export const SeatNode: React.FC<SeatNodeProps> = ({
  seat,
  isSelected,
  onToggle,
  tabIndex = -1,
  onFocusSeat,
  currentCurrency = 'USD',
}) => {
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

  const priceLabel = formatCurrency(seat.price || 0, currentCurrency);
  const tierLabel = seat.tierName || (isVip ? 'VIP' : 'General');

  const description = isWaitlist
    ? `Seat ${seat.row}${seat.number}, ${tierLabel}: waitlist open`
    : isHeld
    ? `Seat ${seat.row}${seat.number}, ${tierLabel}: temporarily held`
    : isBooked
    ? `Seat ${seat.row}${seat.number}, ${tierLabel}: sold`
    : isSelected
    ? `Seat ${seat.row}${seat.number}, ${tierLabel}, ${priceLabel}: selected`
    : `Seat ${seat.row}${seat.number}, ${tierLabel}, ${priceLabel}: available`;

  return (
    <button
      type="button"
      className={`seat-node ${statusClass} ${isVip ? 'seat-vip' : ''} ${!isDisabled ? 'cursor-pointer' : ''}`}
      onClick={handleClick}
      onFocus={() => onFocusSeat?.(seat.id)}
      disabled={isDisabled}
      aria-pressed={isDisabled ? undefined : isSelected}
      aria-label={description}
      title={description}
      tabIndex={isDisabled ? -1 : tabIndex}
      data-seat-id={seat.id}
    >
      <span className="seat-label">{seat.number}</span>
    </button>
  );
};
