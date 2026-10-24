import React, { useState, useEffect, useCallback } from 'react';
import { EventItem, SeatItem, BookingResponse, ApiError, User, AppNotification } from './types';
import { fetchEvents, fetchSeats, createBooking, fetchNotifications } from './services/api';
import { getStoredUser, verifyStoredSession, clearStoredSession } from './services/auth';
import { Header } from './components/Header';
import { EventCatalog } from './components/EventCatalog';
import { VenueSeatMap } from './components/VenueSeatMap';
import { BookingModal } from './components/BookingModal';
import { AuthModal } from './components/AuthModal';
import { MyBookingsModal } from './components/MyBookingsModal';
import { AdminDashboardModal } from './components/AdminDashboardModal';
import { WaitlistModal } from './components/WaitlistModal';
import { NotificationCenter } from './components/NotificationCenter';
import { AlertNotification } from './components/AlertNotification';
import { ConfirmationDialog } from './components/ConfirmationDialog';
import { Theme, getInitialTheme, applyThemeToDocument, toggleThemeValue } from './services/theme';
import { getStoredCurrency, setStoredCurrency } from './services/currency';

export const App: React.FC = () => {
  // Theme State (R28)
  const [theme, setTheme] = useState<Theme>(getInitialTheme);

  // Multi-Currency State (Milestone 26)
  const [currentCurrency, setCurrentCurrency] = useState<string>(getStoredCurrency);

  // Authentication State (R3.1)
  const [currentUser, setCurrentUser] = useState<User | null>(getStoredUser);
  const [isAuthModalOpen, setIsAuthModalOpen] = useState<boolean>(false);
  const [authModalTab, setAuthModalTab] = useState<'login' | 'register'>('login');
  const [isMyBookingsOpen, setIsMyBookingsOpen] = useState<boolean>(false);
  const [isAdminModalOpen, setIsAdminModalOpen] = useState<boolean>(false);
  const [waitlistEvent, setWaitlistEvent] = useState<EventItem | null>(null);

  // Notifications State (Milestone 28)
  const [isNotificationsOpen, setIsNotificationsOpen] = useState<boolean>(false);
  const [notifications, setNotifications] = useState<AppNotification[]>([]);
  const [unreadNotificationsCount, setUnreadNotificationsCount] = useState<number>(0);

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

  // R24 Dialog Confirmation States
  const [isSelectionConfirmOpen, setIsSelectionConfirmOpen] = useState<boolean>(false);
  const [isClearConfirmOpen, setIsClearConfirmOpen] = useState<boolean>(false);

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

  // Auto-verify stored JWT session on mount (R3.1)
  useEffect(() => {
    verifyStoredSession()
      .then((user) => {
        if (user) {
          setCurrentUser(user);
        } else {
          setCurrentUser(null);
        }
      })
      .catch(() => {
        setCurrentUser(null);
      });
  }, []);

  // Load notifications (Milestone 28)
  const loadNotifications = useCallback(async () => {
    try {
      const email = currentUser?.email || 'user@tickets.com';
      const res = await fetchNotifications(email, currentUser?.id);
      setNotifications(res.notifications || []);
      setUnreadNotificationsCount(res.unreadCount || 0);
    } catch (err) {
      // Quiet fail if API not connected
    }
  }, [currentUser]);

  useEffect(() => {
    loadNotifications();
    const interval = setInterval(loadNotifications, 30000);
    return () => clearInterval(interval);
  }, [loadNotifications]);

  const handleOpenAuth = (tab: 'login' | 'register' = 'login') => {
    setAuthModalTab(tab);
    setIsAuthModalOpen(true);
  };

  const handleLogout = () => {
    clearStoredSession();
    setCurrentUser(null);
    setGeneralAlert({
      type: 'info',
      message: 'You have been successfully signed out.',
    });
  };

  const handleAuthSuccess = (user: User) => {
    setCurrentUser(user);
    setGeneralAlert({
      type: 'success',
      message: `Welcome, ${user.name}! You are now signed in.`,
    });
  };

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
   * Real-time SSE Stream subscription for selected event (R10 & R25)
   */
  useEffect(() => {
    if (!selectedEvent || typeof EventSource === 'undefined') return;

    const sseUrl = `/api/inventory/${selectedEvent.id}/seats/stream`;
    const eventSource = new EventSource(sseUrl);

    eventSource.addEventListener('seat_update', (e: MessageEvent) => {
      try {
        const payload = JSON.parse(e.data);
        if (payload && payload.seat_id && payload.status) {
          setSeats((prev) =>
            prev.map((s) => (s.id === payload.seat_id ? { ...s, status: payload.status } : s))
          );
          // Auto-deselect if seat was claimed by another user
          if (payload.status !== 'AVAILABLE') {
            setSelectedSeatIds((prev) => {
              if (prev.has(payload.seat_id)) {
                const next = new Set(prev);
                next.delete(payload.seat_id);
                return next;
              }
              return prev;
            });
          }
        }
      } catch (err) {
        console.warn('Failed to parse SSE seat_update event:', err);
      }
    });

    eventSource.onerror = () => {
      // Reconnect is handled automatically by the browser EventSource
    };

    return () => {
      eventSource.close();
    };
  }, [selectedEvent]);

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

  /**
   * Gated selection clear (R24 dialog confirmation)
   */
  const handleClearSelection = () => {
    if (selectedSeatIds.size > 1) {
      setIsClearConfirmOpen(true);
    } else {
      setSelectedSeatIds(new Set());
    }
  };

  const handleConfirmClearSelection = () => {
    setSelectedSeatIds(new Set());
    setIsClearConfirmOpen(false);
  };

  /**
   * Gated checkout launch (R24 dialog confirmation)
   */
  const handleRequestCheckout = () => {
    if (selectedSeatIds.size === 0) return;
    setIsSelectionConfirmOpen(true);
  };

  const handleConfirmCheckout = () => {
    setIsSelectionConfirmOpen(false);
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
  const handleSubmitBooking = async (
    customerName: string,
    customerEmail: string,
    voucherCode?: string,
    currency?: string,
    corporateDetails?: { companyName?: string; taxId?: string; billingAddress?: string }
  ) => {
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
        userId: currentUser?.id,
        voucherCode,
        currency: currency || currentCurrency,
        companyName: corporateDetails?.companyName,
        taxId: corporateDetails?.taxId,
        billingAddress: corporateDetails?.billingAddress,
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

  /**
   * Synchronize active theme with document attributes & storage
   */
  useEffect(() => {
    applyThemeToDocument(theme);
  }, [theme]);

  const handleToggleTheme = () => {
    setTheme((prev) => {
      const next = toggleThemeValue(prev);
      applyThemeToDocument(next);
      return next;
    });
  };

  return (
    <div className="app-root">
      <Header
        onHomeClick={() => setSelectedEvent(null)}
        apiConnected={apiConnected}
        theme={theme}
        onToggleTheme={handleToggleTheme}
        currentUser={currentUser}
        onOpenAuth={handleOpenAuth}
        onLogout={handleLogout}
        onOpenBookings={() => setIsMyBookingsOpen(true)}
        onOpenAdmin={() => setIsAdminModalOpen(true)}
        currentCurrency={currentCurrency}
        onCurrencyChange={(curr) => {
          setCurrentCurrency(curr);
          setStoredCurrency(curr);
        }}
        onOpenNotifications={() => setIsNotificationsOpen(true)}
        unreadNotificationsCount={unreadNotificationsCount}
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
            onProceedToCheckout={handleRequestCheckout}
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
            onJoinWaitlist={(event) => setWaitlistEvent(event)}
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
          currentUser={currentUser}
          onViewPasses={() => setIsMyBookingsOpen(true)}
          currentCurrency={currentCurrency}
        />
      )}

      {/* R24: Seat Selection Confirmation Dialog */}
      {selectedEvent && (
        <ConfirmationDialog
          isOpen={isSelectionConfirmOpen}
          title="Confirm Seat Selection"
          message={`You have selected ${selectedSeatIds.size} ${selectedSeatIds.size === 1 ? 'seat' : 'seats'} for ${selectedEvent.title}.`}
          confirmLabel={`Proceed to Reservation (${selectedSeatIds.size})`}
          cancelLabel="Modify Seats"
          confirmVariant="primary"
          onConfirm={handleConfirmCheckout}
          onCancel={() => setIsSelectionConfirmOpen(false)}
        >
          <div className="seat-confirm-breakdown">
            <div className="seat-confirm-chips">
              {seats
                .filter((s) => selectedSeatIds.has(s.id))
                .map((seat) => (
                  <div key={seat.id} className="seat-confirm-chip">
                    <span className="chip-label">{seat.row}{seat.number}</span>
                    <span className="chip-tier">{seat.tierName || 'Standard'}</span>
                    <span className="chip-price">${seat.price?.toFixed(2)}</span>
                  </div>
                ))}
            </div>
            <div className="seat-confirm-total-bar">
              <span>Estimated Subtotal:</span>
              <span className="font-bold text-accent">
                ${seats
                  .filter((s) => selectedSeatIds.has(s.id))
                  .reduce((sum, s) => sum + (s.price || 0), 0)
                  .toFixed(2)}
              </span>
            </div>
          </div>
        </ConfirmationDialog>
      )}

      {/* R24: Clear Selection Confirmation Dialog */}
      <ConfirmationDialog
        isOpen={isClearConfirmOpen}
        title="Clear Selected Seats?"
        message={`Are you sure you want to deselect all ${selectedSeatIds.size} selected seats?`}
        confirmLabel="Yes, Clear Selection"
        cancelLabel="Keep Selection"
        confirmVariant="warning"
        onConfirm={handleConfirmClearSelection}
        onCancel={() => setIsClearConfirmOpen(false)}
      />

      {/* Authentication Modal (R3.1) */}
      <AuthModal
        isOpen={isAuthModalOpen}
        onClose={() => setIsAuthModalOpen(false)}
        onAuthSuccess={handleAuthSuccess}
        initialTab={authModalTab}
      />

      {/* My Bookings & Passbook Modal (Milestone 23) */}
      <MyBookingsModal
        isOpen={isMyBookingsOpen}
        onClose={() => setIsMyBookingsOpen(false)}
        currentUser={currentUser}
        onBrowseEvents={() => {
          setSelectedEvent(null);
          loadEvents();
        }}
        onBookingCancelled={(bookingId) => {
          setGeneralAlert({
            type: 'info',
            message: `Reservation #${bookingId} has been cancelled and seats were released.`,
          });
          if (selectedEvent) {
            loadSeatsForEvent(selectedEvent);
          }
        }}
      />

      {/* Priority Waitlist Modal (Milestone 27) */}
      {waitlistEvent && (
        <WaitlistModal
          isOpen={Boolean(waitlistEvent)}
          event={waitlistEvent}
          currentUser={currentUser}
          onClose={() => setWaitlistEvent(null)}
          onOpenPassbook={() => setIsMyBookingsOpen(true)}
          onWaitlistJoined={(entry) => {
            setGeneralAlert({
              type: 'success',
              message: `You're on the waitlist for ${waitlistEvent.title}! Position #${entry.position || 1} in queue.`,
            });
          }}
        />
      )}

      {/* Organizer Studio & Live Telemetry Modal (Milestone 24) */}
      <AdminDashboardModal
        isOpen={isAdminModalOpen}
        onClose={() => setIsAdminModalOpen(false)}
        user={currentUser}
        onEventCreated={() => {
          loadEvents();
          setGeneralAlert({
            type: 'success',
            message: 'Catalog synchronized with newly published event!',
          });
        }}
      />
      {/* In-App Notification Center (Milestone 28) */}
      <NotificationCenter
        isOpen={isNotificationsOpen}
        onClose={() => setIsNotificationsOpen(false)}
        currentUser={currentUser || { email: 'user@tickets.com' }}
        unreadCount={unreadNotificationsCount}
        notifications={notifications}
        onRefresh={loadNotifications}
        onOpenWaitlist={() => setIsMyBookingsOpen(true)}
        onOpenBookings={() => setIsMyBookingsOpen(true)}
      />
    </div>
  );
};
