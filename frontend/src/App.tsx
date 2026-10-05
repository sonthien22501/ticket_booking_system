import React, { useState, useEffect, useCallback } from 'react';
import { EventItem, SeatItem, BookingResponse, ApiError } from './types';
import { fetchEvents, fetchSeats, createBooking } from './services/api';
import { Header } from './components/Header';
import { EventCatalog } from './components/EventCatalog';
import { VenueSeatMap } from './components/VenueSeatMap';
import { BookingModal } from './components/BookingModal';
import { AlertNotification } from './components/AlertNotification';

export const App: React.FC = () => {
  // Catalog State
  const [events, setEvents] = useState<EventItem[]>([]);
  const [loadingEvents, setLoadingEvents] = useState<boolean>(true);
  const [eventsError, setEventsError] = useState<string | null>(null);
  const [apiConnected, setApiConnected] = useState<boolean>(true);

  // Selected Event & Seat Map State
  const [selectedEvent, setSelectedEvent] = useState<EventItem | null>(null);
  const [seats, setSeats] = useState<SeatItem[]>([]);
  const [loadingSeats, setLoadingSeats] = useState<boolean>(false);
  const [selectedSeatIds, setSelectedSeatIds] = useState<Set<string>>(new Set());

  // Checkout & Modal State
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false);
  const [submittingBooking, setSubmittingBooking] = useState<boolean>(false);
  const [conflictError, setConflictError] = useState<string | null>(null);
  const [conflictingSeats, setConflictingSeats] = useState<string[]>([]);
  const [confirmedBooking, setConfirmedBooking] = useState<BookingResponse | null>(null);
  const [generalAlert, setGeneralAlert] = useState<{ type: 'info' | 'success' | 'warning' | 'error'; message: string } | null>(null);

  /**
   * Load events on initial render
   */
  const loadEvents = useCallback(async () => {
    setLoadingEvents(true);
    setEventsError(null);
    try {
      const data = await fetchEvents();
      setEvents(data);
      setApiConnected(true);
    } catch (err) {
      console.error('Failed to load events:', err);
      setEventsError(
        'Unable to reach API Gateway. Please ensure services are running on port 8080.'
      );
      setApiConnected(false);
    } finally {
      setLoadingEvents(false);
    }
  }, []);

  useEffect(() => {
    loadEvents();
  }, [loadEvents]);

  /**
   * Load seats when an event is selected
   */
  const loadSeatsForEvent = useCallback(async (event: EventItem) => {
    setLoadingSeats(true);
    try {
      const seatData = await fetchSeats(event.id, event);
      setSeats(seatData);
    } catch (err) {
      console.error(`Failed to load seats for ${event.id}:`, err);
    } finally {
      setLoadingSeats(false);
    }
  }, []);

  const handleSelectEvent = (event: EventItem) => {
    setSelectedEvent(event);
    setSelectedSeatIds(new Set());
    setConflictError(null);
    setConflictingSeats([]);
    setConfirmedBooking(null);
    loadSeatsForEvent(event);
  };

  /**
   * Toggle seat selection
   */
  const handleToggleSeat = (seat: SeatItem) => {
    setSelectedSeatIds((prev) => {
      const next = new Set(prev);
      if (next.has(seat.id)) {
        next.delete(seat.id);
      } else {
        next.add(seat.id);
      }
      return next;
    });
  };

  const handleClearSelection = () => {
    setSelectedSeatIds(new Set());
  };

  const handleOpenCheckout = () => {
    setConflictError(null);
    setConflictingSeats([]);
    setIsModalOpen(true);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
    setConflictError(null);
    setConflictingSeats([]);
  };

  /**
   * Refreshes the seat map after a conflict or manual user action
   */
  const handleRefreshSeats = async () => {
    if (!selectedEvent) return;
    await loadSeatsForEvent(selectedEvent);
  };

  /**
   * Handles checkout submission with 409 Conflict logic
   */
  const handleSubmitBooking = async (customerName: string, customerEmail: string) => {
    if (!selectedEvent || selectedSeatIds.size === 0) return;

    setSubmittingBooking(true);
    setConflictError(null);
    setConflictingSeats([]);

    const seatIds = Array.from(selectedSeatIds);

    try {
      const booking = await createBooking({
        eventId: selectedEvent.id,
        tierId: selectedEvent.tiers[0]?.id,
        seats: seatIds,
        customerName,
        customerEmail,
      });

      // Booking confirmed successfully!
      setConfirmedBooking(booking);

      // Update local seat status
      setSeats((prev) =>
        prev.map((s) => (selectedSeatIds.has(s.id) ? { ...s, status: 'BOOKED' } : s))
      );
      setSelectedSeatIds(new Set());
    } catch (err: unknown) {
      const apiErr = err as ApiError;
      if (apiErr.status === 409) {
        // High concurrency conflict detected!
        const conflictSeatsList = apiErr.conflictingSeats && apiErr.conflictingSeats.length > 0
          ? apiErr.conflictingSeats
          : seatIds;

        setConflictError(
          apiErr.message ||
            'One or more of your chosen seats were just reserved by another customer. Please choose alternative seats.'
        );
        setConflictingSeats(conflictSeatsList);

        // Immediately reload seats to show updated booked statuses
        await handleRefreshSeats();

        // Remove conflicting seats from current selection
        setSelectedSeatIds((prev) => {
          const next = new Set(prev);
          conflictSeatsList.forEach((id) => next.delete(id));
          return next;
        });
      } else {
        setGeneralAlert({
          type: 'error',
          message: apiErr.message || 'An unexpected error occurred while placing your booking.',
        });
      }
    } finally {
      setSubmittingBooking(false);
    }
  };

  const handleResetAfterBooking = () => {
    setConfirmedBooking(null);
    setSelectedSeatIds(new Set());
    setSelectedEvent(null);
    loadEvents();
  };

  return (
    <div className="app-root">
      <Header
        onHomeClick={() => setSelectedEvent(null)}
        apiConnected={apiConnected}
      />

      <main className="main-content">
        {generalAlert && (
          <AlertNotification
            type={generalAlert.type}
            message={generalAlert.message}
            onClose={() => setGeneralAlert(null)}
          />
        )}

        {selectedEvent ? (
          <VenueSeatMap
            event={selectedEvent}
            seats={seats}
            selectedSeatIds={selectedSeatIds}
            loadingSeats={loadingSeats}
            onToggleSeat={handleToggleSeat}
            onClearSelection={handleClearSelection}
            onProceedToCheckout={handleOpenCheckout}
            onBackToCatalog={() => setSelectedEvent(null)}
            onRefreshSeats={handleRefreshSeats}
          />
        ) : (
          <EventCatalog
            events={events}
            loading={loadingEvents}
            error={eventsError}
            onRefresh={loadEvents}
            onSelectEvent={handleSelectEvent}
          />
        )}
      </main>

      {/* Booking Checkout Modal */}
      {selectedEvent && (
        <BookingModal
          isOpen={isModalOpen}
          event={selectedEvent}
          selectedSeats={seats.filter((s) => selectedSeatIds.has(s.id))}
          submitting={submittingBooking}
          conflictError={conflictError}
          conflictingSeats={conflictingSeats}
          confirmedBooking={confirmedBooking}
          onClose={handleCloseModal}
          onSubmitBooking={handleSubmitBooking}
          onRefreshMap={handleRefreshSeats}
          onResetBooking={handleResetAfterBooking}
        />
      )}
    </div>
  );
};
