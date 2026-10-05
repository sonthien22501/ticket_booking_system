import React, { useMemo } from 'react';
import { SeatItem } from '../types';
import { SeatNode } from './SeatNode';

interface SeatGridProps {
  seats: SeatItem[];
  selectedSeatIds: Set<string>;
  onToggleSeat: (seat: SeatItem) => void;
}

export const SeatGrid: React.FC<SeatGridProps> = ({
  seats,
  selectedSeatIds,
  onToggleSeat,
}) => {
  // Group seats by row preserving order
  const groupedRows = useMemo(() => {
    const rowMap = new Map<string, SeatItem[]>();
    seats.forEach((seat) => {
      const existing = rowMap.get(seat.row) || [];
      existing.push(seat);
      rowMap.set(seat.row, existing);
    });

    // Sort seats in each row by seat number
    Array.from(rowMap.keys()).forEach((rowKey) => {
      const rowSeats = rowMap.get(rowKey)!;
      rowSeats.sort((a, b) => a.number - b.number);
    });

    return Array.from(rowMap.entries());
  }, [seats]);

  return (
    <div className="seat-grid-container">
      {groupedRows.map(([rowLabel, rowSeats]) => {
        const isVipRow = rowLabel === 'A' || rowLabel === 'B' || rowSeats[0]?.tierId?.includes('vip');
        return (
          <div key={rowLabel} className="seat-row-wrapper">
            <div className="seat-row-label">
              <span className="row-letter">{rowLabel}</span>
              {isVipRow && <span className="row-badge-vip">VIP</span>}
            </div>

            <div className="seat-row-items">
              {rowSeats.map((seat) => (
                <SeatNode
                  key={seat.id}
                  seat={seat}
                  isSelected={selectedSeatIds.has(seat.id)}
                  onToggle={onToggleSeat}
                />
              ))}
            </div>

            <div className="seat-row-label-right">
              <span className="row-letter">{rowLabel}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
};
