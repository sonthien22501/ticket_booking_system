import React, { useState, useEffect, useMemo } from 'react';
import { EventItem, SeatItem, VenueFloorplan } from '../types';
import { SeatGrid } from './SeatGrid';
import { ArrowLeftIcon, RefreshCwIcon, TicketIcon, CalendarIcon, MapPinIcon, BuildingOfficeIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';
import { fetchVenueFloorplan } from '../services/api';

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
  const [floorplan, setFloorplan] = useState<VenueFloorplan | null>(null);
  const [selectedSection, setSelectedSection] = useState<string>('ALL');
  const [showFloorplanModal, setShowFloorplanModal] = useState<boolean>(false);

  useEffect(() => {
    let isMounted = true;
    fetchVenueFloorplan(event.id)
      .then((data) => {
        if (isMounted) setFloorplan(data);
      })
      .catch((err) => {
        console.warn('Failed to load venue floorplan:', err);
      });
    return () => {
      isMounted = false;
    };
  }, [event.id]);

  // Filter seats if a specific section is selected
  const displayedSeats = useMemo(() => {
    if (selectedSection === 'ALL') return seats;
    if (selectedSection === 'VIP') {
      return seats.filter((s) => s.row === 'A' || s.row === 'B' || (s.tierId && s.tierId.includes('vip')));
    }
    if (selectedSection === 'GENERAL') {
      return seats.filter((s) => s.row !== 'A' && s.row !== 'B' && (!s.tierId || !s.tierId.includes('vip')));
    }
    return seats;
  }, [seats, selectedSection]);
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
        <button type="button" onClick={onBackToCatalog} className="btn-back cursor-pointer">
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
          className="btn-refresh-seats cursor-pointer"
          title="Refresh seat availability"
        >
          <RefreshCwIcon size={16} className="mr-1" />
          Refresh Map
        </button>
      </div>

      {/* Main Seat Map Area */}
      <div className="seatmap-viewport">
        {/* Premium Curved Cinema Screen Canvas */}
        <div className="stage-container cinema-screen-container">
          <div className="cinema-projector-light"></div>
          <div className="stage-curve cinema-screen-curve">
            <div className="screen-reflection"></div>
            <span className="stage-title">CURVED CINEMA SCREEN / STAGE</span>
          </div>
          <div className="stage-lights cinema-screen-glow"></div>
        </div>

        {/* Legend */}
        <div className="seatmap-legend" role="list" aria-label="Seat Map Legend">
          <div className="legend-item" role="listitem">
            <span className="legend-dot dot-available"></span>
            <span>Available</span>
          </div>
          <div className="legend-item" role="listitem">
            <span className="legend-dot dot-selected"></span>
            <span>Selected</span>
          </div>
          <div className="legend-item" role="listitem">
            <span className="legend-dot dot-held"></span>
            <span>On Hold</span>
          </div>
          <div className="legend-item" role="listitem">
            <span className="legend-dot dot-booked"></span>
            <span>Reserved / Sold</span>
          </div>
          <div className="legend-item" role="listitem">
            <span className="legend-dot dot-vip"></span>
            <span>VIP Tier</span>
          </div>
          <div className="legend-item" role="listitem">
            <span className="legend-dot dot-waitlist"></span>
            <span>Waitlist</span>
          </div>
        </div>

        {/* Section Switcher & Floorplan Modal Button (Milestone 31) */}
        <div className="flex items-center justify-between px-6 py-2 bg-slate-900/60 rounded-xl mx-4 mb-3 border border-slate-800">
          <div className="flex items-center gap-1.5 text-xs">
            <span className="text-slate-400 font-semibold mr-1">Section:</span>
            {[
              { id: 'ALL', label: 'All Sections' },
              { id: 'VIP', label: 'VIP Front (Rows A-B)' },
              { id: 'GENERAL', label: 'General Admission (Rows C-F)' },
            ].map((sec) => (
              <button
                key={sec.id}
                type="button"
                onClick={() => setSelectedSection(sec.id)}
                className={`px-2.5 py-1 rounded-lg text-xs font-semibold cursor-pointer transition-colors ${
                  selectedSection === sec.id
                    ? 'bg-amber-500 text-slate-950 font-bold'
                    : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
                }`}
                data-testid={`section-filter-${sec.id.toLowerCase()}`}
              >
                {sec.label}
              </button>
            ))}
          </div>

          <button
            type="button"
            onClick={() => setShowFloorplanModal(!showFloorplanModal)}
            className="px-3 py-1 rounded-lg bg-indigo-900/40 hover:bg-indigo-800 text-indigo-300 hover:text-white border border-indigo-500/30 text-xs font-semibold flex items-center gap-1.5 cursor-pointer transition-colors"
            data-testid="toggle-floorplan-btn"
          >
            <BuildingOfficeIcon size={14} />
            <span>Floorplan Schematics</span>
          </button>
        </div>

        {/* Venue Floorplan Modal / Drawer */}
        {showFloorplanModal && floorplan && (
          <div className="p-4 mx-4 mb-4 rounded-2xl bg-slate-900/90 border border-indigo-500/40 animate-in fade-in space-y-3" data-testid="floorplan-overview-card">
            <div className="flex items-center justify-between border-b border-slate-800 pb-2">
              <h4 className="text-sm font-bold text-slate-100 flex items-center gap-2">
                <BuildingOfficeIcon size={16} className="text-amber-400" />
                <span>{floorplan.venueName} • Architectural Floorplan Matrix</span>
              </h4>
              <span className="text-xs text-slate-400">Orientation: {floorplan.stageOrientation}</span>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              {floorplan.sections.map((sec) => (
                <div key={sec.id} className="p-3 rounded-xl bg-slate-950/70 border border-slate-800">
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-xs font-bold text-slate-200">{sec.name}</span>
                    <span className="text-[10px] font-bold px-1.5 py-0.5 rounded uppercase" style={{ backgroundColor: `${sec.color}20`, color: sec.color }}>
                      {sec.category}
                    </span>
                  </div>
                  <p className="text-[11px] text-slate-400">{sec.rowRange}</p>
                  <div className="flex items-center justify-between text-xs mt-2 pt-1 border-t border-slate-800/60 font-mono">
                    <span className="text-slate-400">Cap: {sec.capacity > 0 ? sec.capacity : 'N/A'}</span>
                    <span className="text-amber-300 font-bold">${sec.basePrice.toFixed(2)}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Interactive Grid */}
        {loadingSeats ? (
          <div className="seatmap-loading">
            <LoadingSpinner message="Checking real-time seat inventory locks..." size="md" />
          </div>
        ) : (
          <SeatGrid
            seats={displayedSeats}
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
              className="btn-clear-selection cursor-pointer"
            >
              Clear
            </button>
          )}

          <button
            type="button"
            className="btn-checkout cursor-pointer"
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
