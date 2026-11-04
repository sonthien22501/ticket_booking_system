import React, { useMemo, useRef, useState } from 'react';
import { SeatItem } from '../types';
import { SeatNode } from './SeatNode';
import { formatCurrency } from '../services/currency';

interface SeatGridProps {
  seats: SeatItem[];
  selectedSeatIds: Set<string>;
  onToggleSeat: (seat: SeatItem) => void;
  currentCurrency?: string;
}

const NAV_KEYS = ['ArrowRight', 'ArrowLeft', 'ArrowUp', 'ArrowDown', 'Home', 'End'];

export const SeatGrid: React.FC<SeatGridProps> = ({
  seats,
  selectedSeatIds,
  onToggleSeat,
  currentCurrency = 'USD',
}) => {
  const gridRef = useRef<HTMLDivElement | null>(null);
  const [activeSeatId, setActiveSeatId] = useState<string | null>(null);

  // Group seats by row preserving order
  const groupedRows = useMemo(() => {
    const rowMap = new Map<string, SeatItem[]>();
    seats.forEach((seat) => {
      const existing = rowMap.get(seat.row) || [];
      existing.push(seat);
      rowMap.set(seat.row, existing);
    });

    Array.from(rowMap.keys()).forEach((rowKey) => {
      const rowSeats = rowMap.get(rowKey)!;
      rowSeats.sort((a, b) => a.number - b.number);
    });

    return Array.from(rowMap.entries());
  }, [seats]);

  const maxColumns = useMemo(() => {
    let max = 0;
    seats.forEach((s) => {
      if (s.number > max) max = s.number;
    });
    return max || 12;
  }, [seats]);

  // One stop in the tab order for the whole grid; arrows move within it.
  const firstSelectableId = useMemo(
    () => seats.find((s) => s.status !== 'HOLD' && s.status !== 'BOOKED' && s.status !== 'BLOCKED' && s.status !== 'WAITLIST')?.id ?? null,
    [seats]
  );

  const handleKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (!NAV_KEYS.includes(e.key)) return;

    const nodes = Array.from(
      gridRef.current?.querySelectorAll<HTMLButtonElement>('.seat-node:not(:disabled)') ?? []
    );
    if (nodes.length === 0) return;

    const currentIndex = nodes.indexOf(document.activeElement as HTMLButtonElement);
    if (currentIndex === -1) return;

    let nextIndex = currentIndex;
    if (e.key === 'ArrowRight') nextIndex = Math.min(nodes.length - 1, currentIndex + 1);
    if (e.key === 'ArrowLeft') nextIndex = Math.max(0, currentIndex - 1);
    if (e.key === 'ArrowDown') nextIndex = Math.min(nodes.length - 1, currentIndex + maxColumns);
    if (e.key === 'ArrowUp') nextIndex = Math.max(0, currentIndex - maxColumns);
    if (e.key === 'Home') nextIndex = 0;
    if (e.key === 'End') nextIndex = nodes.length - 1;

    e.preventDefault();
    nodes[nextIndex].focus();
    setActiveSeatId(nodes[nextIndex].getAttribute('data-seat-id'));
  };

  return (
    <div
      className="seat-grid-container"
      ref={gridRef}
      role="group"
      aria-label="Seat map. Use arrow keys to move between seats, Enter or Space to select."
      onKeyDown={handleKeyDown}
    >
      <div className="seat-column-indicators" aria-hidden="true">
        <div className="column-label-spacer"></div>
        <div className="column-numbers-row">
          {Array.from({ length: maxColumns }, (_, i) => i + 1).map((col) => (
            <span key={col} className="column-number-badge">
              {col}
            </span>
          ))}
        </div>
        <div className="column-label-spacer"></div>
      </div>

      {groupedRows.map(([rowLabel, rowSeats]) => {
        const isVipRow = rowLabel === 'A' || rowLabel === 'B' || rowSeats[0]?.tierId?.includes('vip');
        const rowTier = rowSeats[0]?.tierName || (isVipRow ? 'VIP' : '');
        const rowPrice = rowSeats.find((seat) => typeof seat.price === 'number')?.price;
        const priceLabel = typeof rowPrice === 'number' ? formatCurrency(rowPrice, currentCurrency) : '';
        return (
          <div key={rowLabel} className="seat-row-wrapper">
            <div className="seat-row-label">
              <span className="row-letter">{rowLabel}</span>
              {isVipRow && <span className="row-badge-vip">VIP</span>}
              {priceLabel && <span className="row-price row-price-compact">{priceLabel}</span>}
            </div>

            <div className="seat-row-items">
              {rowSeats.map((seat) => {
                const isSelectable =
                  seat.status !== 'HOLD' &&
                  seat.status !== 'BOOKED' &&
                  seat.status !== 'BLOCKED' &&
                  seat.status !== 'WAITLIST';
                const tabIndex =
                  isSelectable && (activeSeatId ? seat.id === activeSeatId : seat.id === firstSelectableId)
                    ? 0
                    : -1;

                return (
                  <SeatNode
                    key={seat.id}
                    seat={seat}
                    isSelected={selectedSeatIds.has(seat.id)}
                    onToggle={onToggleSeat}
                    tabIndex={tabIndex}
                    onFocusSeat={setActiveSeatId}
                    currentCurrency={currentCurrency}
                  />
                );
              })}
            </div>

            <div className="seat-row-label-right">
              {rowTier && <span className="row-tier-name">{rowTier}</span>}
              {priceLabel && <span className="row-price">{priceLabel}</span>}
            </div>
          </div>
        );
      })}
    </div>
  );
};
