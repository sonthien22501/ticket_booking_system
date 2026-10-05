import React, { useMemo } from 'react';
import { EventItem, SeatItem } from '../types';
import { SeatGrid } from './SeatGrid';
import { ArrowLeftIcon, RefreshCwIcon, TicketIcon, CalendarIcon, MapPinIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';

interface VenueSeatMapProps {
  event: EventItem;
  seats: SeatItem[];
  selectedSeatIds: Set<string>;
  loadingSeats: boolean;
  onToggleSeat: (seat: SeatItem) => void;
  onClearSelection: () => void;
  onProceedToCheckout: () => void;
  onBackToCatalog: () => void;
  onRefreshSeats: () => void;
}

export const VenueSeatMap: React.FC<VenueSeatMapProps> = ({
  event,
  seats,
  selectedSeatIds,
  loadingSeats,
  onToggleSeat,
  onClearSelection,
  onProceedToCheckout,
  onBackToCatalog,
  onRefreshSeats,
}) => {
  // Calculate selection totals
  const selectedSeatsList = useMemo(() => {
    return seats.filter((s) => selectedSeatIds.has(s.id));
  }, [seats, selectedSeatIds]);

  const subtotal = useMemo(() => {
    return selectedSeatsList.reduce((acc, s) => acc + (s.price || 0), 0);
  }, [selectedSeatsList]);

  const availableCount = useMemo(() => {
    return seats.filter((s) => s.status === 'AVAILABLE').length;
  }, [seats]);

  return (
    <div className="venue-seatmap-page">
      {/* Top Navigation & Event Header */}
      <div className="seatmap-top-bar">
        <button type="button" onClick={onBackToCatalog} className="btn-back">
          <ArrowLeftIcon size={18} className="mr-2" />
          Back to Events
        </button>

        <div className="seatmap-event-info">
          <h2 className="seatmap-event-title">{event.title}</h2>
          <div className="seatmap-event-meta">
            <span>
              <CalendarIcon size={14} className="mr-1 inline-icon" />
              {new Date(event.startTime).toLocaleDateString(undefined, {
                month: 'short',
                day: 'numeric',
                year: 'numeric',
              })}
            </span>
            <span>
              <MapPinIcon size={14} className="mr-1 inline-icon" />
              {event.venueName}
            </span>
            <span className="available-seats-badge">
              {availableCount} seats available
            </span>
          </div>
        </div>

        <button
          type="button"
          onClick={onRefreshSeats}
          className="btn-refresh-seats"
          title="Refresh seat availability"
        >
          <RefreshCwIcon size={16} className="mr-1" />
          Refresh Map
        </button>
      </div>

      {/* Main Seat Map Area */}
      <div className="seatmap-viewport">
        {/* Stage Curved Banner */}
        <div className="stage-container">
          <div className="stage-curve">
            <span className="stage-title">STAGE / PERFORMANCE AREA</span>
          </div>
          <div className="stage-lights"></div>
        </div>

        {/* Legend */}
        <div className="seatmap-legend">
          <div className="legend-item">
            <span className="legend-dot dot-available"></span>
            <span>Available</span>
          </div>
          <div className="legend-item">
            <span className="legend-dot dot-selected"></span>
            <span>Selected</span>
          </div>
          <div className="legend-item">
            <span className="legend-dot dot-booked"></span>
            <span>Booked / Reserved</span>
          </div>
          <div className="legend-item">
            <span className="legend-dot dot-vip"></span>
            <span>VIP Tier</span>
          </div>
        </div>

        {/* Interactive Grid */}
        {loadingSeats ? (
          <div className="seatmap-loading">
            <LoadingSpinner message="Checking real-time seat inventory locks..." size="md" />
          </div>
        ) : (
          <SeatGrid
            seats={seats}
            selectedSeatIds={selectedSeatIds}
            onToggleSeat={onToggleSeat}
          />
        )}
      </div>

      {/* Sticky Selection Summary Footer */}
      <div className="selection-summary-footer">
        <div className="summary-left">
          <div className="selected-count-badge">
            <TicketIcon size={18} className="mr-1" />
            <span>
              {selectedSeatsList.length} {selectedSeatsList.length === 1 ? 'Seat' : 'Seats'} Selected
            </span>
          </div>

          <div className="selected-chips-list">
            {selectedSeatsList.map((seat) => (
              <span key={seat.id} className="selected-seat-chip">
                {seat.row}{seat.number} (${seat.price?.toFixed(0)})
              </span>
            ))}
          </div>
        </div>

        <div className="summary-right">
          <div className="price-total-box">
            <span className="total-label">Subtotal</span>
            <span className="total-amount">${subtotal.toFixed(2)}</span>
          </div>

          {selectedSeatsList.length > 0 && (
            <button
              type="button"
              onClick={onClearSelection}
              className="btn-clear-selection"
            >
              Clear
            </button>
          )}

          <button
            type="button"
            className="btn-checkout"
            disabled={selectedSeatsList.length === 0}
            onClick={onProceedToCheckout}
          >
            Checkout ({selectedSeatsList.length})
          </button>
        </div>
      </div>
    </div>
  );
};
