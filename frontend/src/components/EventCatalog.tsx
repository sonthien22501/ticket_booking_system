import React, { useState, useMemo, useEffect } from 'react';
import { EventItem } from '../types';
import { EventCard } from './EventCard';
import { SearchIcon, RefreshCwIcon, MapPinIcon, XIcon, SparklesIcon, TicketIcon, CalendarIcon, ChevronRightIcon, PlayIcon, PauseIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';
import { AlertNotification } from './AlertNotification';
import { formatCurrency } from '../services/currency';

interface EventCatalogProps {
  events: EventItem[];
  loading: boolean;
  error: string | null;
  onRefresh: () => void;
  onSelectEvent: (event: EventItem) => void;
  onJoinWaitlist?: (event: EventItem) => void;
  onViewReviews?: (event: EventItem) => void;
  currentCurrency?: string;
}

export const BUDGET_RANGES = [
  { id: 'ALL', label: 'Any budget' },
  { id: 'UNDER_100', label: 'Under $100' },
  { id: '100_TO_200', label: '$100 – $200' },
  { id: 'OVER_200', label: 'Over $200' },
] as const;

export const EventCatalog: React.FC<EventCatalogProps> = ({
  events,
  loading,
  error,
  onRefresh,
  onSelectEvent,
  onJoinWaitlist,
  onViewReviews,
  currentCurrency = 'USD',
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('ALL');
  const [selectedCity, setSelectedCity] = useState<string>('ALL');
  const [selectedPriceRange, setSelectedPriceRange] = useState<string>('ALL');
  const [sortBy, setSortBy] = useState<string>('date_asc');
  const [featuredIndex, setFeaturedIndex] = useState<number>(0);
  const [carouselPaused, setCarouselPaused] = useState(false);

  // Featured events for the Premiere Hero Showcase (up to 4 active events)
  const featuredEvents = useMemo(() => {
    const active = events.filter((e) => e.status === 'ACTIVE');
    return (active.length > 0 ? active : events).slice(0, 4);
  }, [events]);

  // Rotate the featured showcase, but never while it is hovered, focused,
  // manually paused, or when the visitor asked for reduced motion.
  useEffect(() => {
    if (featuredEvents.length <= 1 || carouselPaused) return;
    if (typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const timer = setInterval(() => {
      setFeaturedIndex((prev) => (prev + 1) % featuredEvents.length);
    }, 6000);
    return () => clearInterval(timer);
  }, [featuredEvents.length, carouselPaused]);

  const currentFeatured = featuredEvents[featuredIndex % Math.max(1, featuredEvents.length)] || null;

  // Extract unique categories
  const categories = useMemo(() => {
    const set = new Set<string>();
    events.forEach((e) => set.add(e.category));
    return ['ALL', ...Array.from(set)];
  }, [events]);

  // Extract unique cities from locations (e.g., "San Francisco, CA" -> "San Francisco")
  const cities = useMemo(() => {
    const set = new Set<string>();
    events.forEach((e) => {
      const city = (e.venueLocation || '').split(',')[0].trim();
      if (city) set.add(city);
    });
    return ['ALL', ...Array.from(set)];
  }, [events]);

  // Filter events based on multi-facet search criteria
  const filteredEvents = useMemo(() => {
    return events
      .filter((e) => {
        // Keyword match
        const matchesSearch =
          !searchQuery.trim() ||
          e.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
          e.venueName.toLowerCase().includes(searchQuery.toLowerCase()) ||
          (e.venueLocation && e.venueLocation.toLowerCase().includes(searchQuery.toLowerCase())) ||
          e.description.toLowerCase().includes(searchQuery.toLowerCase());

        // Category facet
        const matchesCat =
          selectedCategory === 'ALL' || e.category.toUpperCase() === selectedCategory.toUpperCase();

        // City facet
        const eCity = (e.venueLocation || '').split(',')[0].trim().toLowerCase();
        const matchesCity =
          selectedCity === 'ALL' || eCity === selectedCity.toLowerCase() || (e.venueLocation || '').toLowerCase().includes(selectedCity.toLowerCase());

        // Price range facet
        const minPrice = e.tiers.length > 0 ? Math.min(...e.tiers.map((t) => t.price)) : 0;
        let matchesPrice = true;
        if (selectedPriceRange === 'UNDER_100') {
          matchesPrice = minPrice < 100;
        } else if (selectedPriceRange === '100_TO_200') {
          matchesPrice = minPrice >= 100 && minPrice <= 200;
        } else if (selectedPriceRange === 'OVER_200') {
          matchesPrice = minPrice > 200;
        }

        return matchesSearch && matchesCat && matchesCity && matchesPrice;
      })
      .sort((a, b) => {
        const priceA = a.tiers.length > 0 ? Math.min(...a.tiers.map((t) => t.price)) : 0;
        const priceB = b.tiers.length > 0 ? Math.min(...b.tiers.map((t) => t.price)) : 0;

        switch (sortBy) {
          case 'date_desc':
            return new Date(b.startTime).getTime() - new Date(a.startTime).getTime();
          case 'price_asc':
            return priceA - priceB;
          case 'price_desc':
            return priceB - priceA;
          case 'title_asc':
            return a.title.localeCompare(b.title);
          case 'date_asc':
          default:
            return new Date(a.startTime).getTime() - new Date(b.startTime).getTime();
        }
      });
  }, [events, searchQuery, selectedCategory, selectedCity, selectedPriceRange, sortBy]);

  const facetCount =
    (selectedCategory !== 'ALL' ? 1 : 0) +
    (selectedCity !== 'ALL' ? 1 : 0) +
    (selectedPriceRange !== 'ALL' ? 1 : 0);

  const hasActiveFilters =
    Boolean(searchQuery.trim()) ||
    selectedCategory !== 'ALL' ||
    selectedCity !== 'ALL' ||
    selectedPriceRange !== 'ALL' ||
    sortBy !== 'date_asc';

  const handleResetFilters = () => {
    setSearchQuery('');
    setSelectedCategory('ALL');
    setSelectedCity('ALL');
    setSelectedPriceRange('ALL');
    setSortBy('date_asc');
  };

  return (
    <section className="catalog-section">
      <div className="hero-banner premiere-hero-container">
        <h2 className="hero-title">Pick your night. Pick your seat.</h2>
        <p className="hero-subtitle">
          {events.length} {events.length === 1 ? 'event' : 'events'} on sale — every seat mapped live
          and held for you while you check out.
        </p>

        {/* Featured Premiere Spotlight Showcase Carousel */}
        {currentFeatured && !loading && (
          <div
            className="premiere-showcase-card"
            data-testid="premiere-hero-showcase"
            onMouseEnter={() => setCarouselPaused(true)}
            onMouseLeave={() => setCarouselPaused(false)}
            onFocusCapture={() => setCarouselPaused(true)}
            onBlurCapture={() => setCarouselPaused(false)}
          >
            <div className="premiere-showcase-content">
              <div className="premiere-showcase-left">
                <div className="premiere-badges-row mb-2">
                  <span className="premiere-spotlight-pill">
                    <SparklesIcon size={12} className="mr-1" />
                    FEATURED PREMIERE #{featuredIndex + 1}
                  </span>
                  <span className="premiere-category-tag">{currentFeatured.category}</span>
                  <span
                    className="premiere-venue-tag"
                    title={`${currentFeatured.venueName} (${currentFeatured.venueLocation})`}
                  >
                    <MapPinIcon size={12} className="mr-1 text-amber-400 shrink-0" />
                    <span className="truncate">{currentFeatured.venueName} ({currentFeatured.venueLocation})</span>
                  </span>
                </div>

                <h3 className="premiere-showcase-title" key={currentFeatured.id}>
                  {currentFeatured.title}
                </h3>
                <p className="premiere-showcase-desc line-clamp-2">
                  {currentFeatured.description}
                </p>

                <div className="premiere-showcase-meta">
                  <span className="premiere-date-tag flex items-center gap-1.5 text-xs text-slate-300">
                    <CalendarIcon size={14} className="text-amber-400" />
                    {new Date(currentFeatured.startTime).toLocaleDateString('en-US', {
                      weekday: 'short',
                      month: 'short',
                      day: 'numeric',
                      year: 'numeric',
                    })}
                  </span>
                  {currentFeatured.tiers.length > 0 && (
                    <span className="premiere-price-pill">
                      From {formatCurrency(Math.min(...currentFeatured.tiers.map((t) => t.price)), currentCurrency)}
                    </span>
                  )}
                </div>
              </div>

              <div className="premiere-showcase-right">
                <button
                  type="button"
                  onClick={() => onSelectEvent(currentFeatured)}
                  className="btn-premiere-cta cursor-pointer"
                  data-testid="premiere-hero-cta"
                >
                  <TicketIcon size={18} />
                  <span>Choose seats</span>
                  <ChevronRightIcon size={16} />
                </button>

                {/* Carousel Slide Indicators */}
                {featuredEvents.length > 1 && (
                  <div className="premiere-carousel-controls">
                    <div className="premiere-carousel-dots" role="tablist" aria-label="Featured events">
                      {featuredEvents.map((fe, idx) => (
                        <button
                          key={fe.id}
                          type="button"
                          role="tab"
                          aria-selected={idx === featuredIndex}
                          aria-label={`Featured event ${idx + 1}: ${fe.title}`}
                          onClick={() => setFeaturedIndex(idx)}
                          className={`premiere-dot cursor-pointer ${idx === featuredIndex ? 'active' : ''}`}
                          data-testid={`premiere-slide-indicator-${idx}`}
                        />
                      ))}
                    </div>
                    <button
                      type="button"
                      className="premiere-carousel-pause cursor-pointer"
                      onClick={() => setCarouselPaused((p) => !p)}
                      aria-label={carouselPaused ? 'Resume featured rotation' : 'Pause featured rotation'}
                      title={carouselPaused ? 'Resume featured rotation' : 'Pause featured rotation'}
                    >
                      {carouselPaused ? <PlayIcon size={11} /> : <PauseIcon size={11} />}
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        <div className="search-and-filter-bar">
          <div className="catalog-search-row">
            <div className="search-input-wrapper">
              <SearchIcon size={18} className="search-icon" />
              <input
                type="text"
                placeholder="Search events or venues"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="search-input"
                data-testid="catalog-search-input"
              />
            </div>

            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value)}
              className="catalog-sort-select"
              data-testid="catalog-sort-select"
              aria-label="Sort events"
            >
              <option value="date_asc">Date: soonest</option>
              <option value="date_desc">Date: furthest</option>
              <option value="price_asc">Price: low to high</option>
              <option value="price_desc">Price: high to low</option>
              <option value="title_asc">Title: A-Z</option>
            </select>
          </div>

          <div className="catalog-facet-row">
            <div className="category-pills" role="group" aria-label="Filter by category">
              {categories.map((cat) => (
                <button
                  key={cat}
                  type="button"
                  aria-pressed={selectedCategory === cat}
                  className={`category-pill cursor-pointer ${selectedCategory === cat ? 'active' : ''}`}
                  onClick={() => setSelectedCategory(cat)}
                  data-testid={`category-pill-${cat}`}
                >
                  {cat === 'ALL' ? 'Everything' : cat}
                </button>
              ))}
            </div>

            <details className="filter-disclosure">
              <summary className="filter-disclosure-summary">
                <span>Filters</span>
                {hasActiveFilters && <span className="filter-count">{facetCount}</span>}
              </summary>
              <div className="filter-disclosure-body">
                {cities.length > 2 && (
                  <fieldset className="filter-fieldset">
                    <legend>
                      <MapPinIcon size={12} /> City
                    </legend>
                    <div className="facet-options" data-testid="city-facets-bar">
                      {cities.map((city) => (
                        <button
                          key={city}
                          type="button"
                          onClick={() => setSelectedCity(city)}
                          aria-pressed={selectedCity === city}
                          className={`facet-option cursor-pointer ${selectedCity === city ? 'active' : ''}`}
                          data-testid={`city-pill-${city.replace(/\s+/g, '-').toLowerCase()}`}
                        >
                          {city === 'ALL' ? 'Anywhere' : city}
                        </button>
                      ))}
                    </div>
                  </fieldset>
                )}

                <fieldset className="filter-fieldset">
                  <legend>Budget per seat</legend>
                  <div className="facet-options">
                    {BUDGET_RANGES.map((b) => (
                      <button
                        key={b.id}
                        type="button"
                        onClick={() => setSelectedPriceRange(b.id)}
                        aria-pressed={selectedPriceRange === b.id}
                        className={`facet-option cursor-pointer ${selectedPriceRange === b.id ? 'active' : ''}`}
                        data-testid={`price-pill-${b.id}`}
                      >
                        {b.label}
                      </button>
                    ))}
                  </div>
                </fieldset>
              </div>
            </details>
          </div>

          <div className="catalog-summary-bar">
            <span>
              Showing <strong>{filteredEvents.length}</strong> of {events.length} events
            </span>
            <div className="active-filter-chips">
              {selectedCategory !== 'ALL' && (
                <button type="button" className="filter-chip" onClick={() => setSelectedCategory('ALL')}>
                  {selectedCategory}
                  <XIcon size={12} />
                </button>
              )}
              {selectedCity !== 'ALL' && (
                <button type="button" className="filter-chip" onClick={() => setSelectedCity('ALL')}>
                  {selectedCity}
                  <XIcon size={12} />
                </button>
              )}
              {selectedPriceRange !== 'ALL' && (
                <button type="button" className="filter-chip" onClick={() => setSelectedPriceRange('ALL')}>
                  {BUDGET_RANGES.find((b) => b.id === selectedPriceRange)?.label}
                  <XIcon size={12} />
                </button>
              )}
              {hasActiveFilters && (
                <button
                  type="button"
                  onClick={handleResetFilters}
                  className="filter-chip filter-chip-clear"
                  data-testid="reset-filters-btn"
                >
                  Clear all
                </button>
              )}
            </div>
          </div>
        </div>
      </div>

      {error && (
        <AlertNotification
          type="error"
          title="Catalog Service Notice"
          message={error}
          onRetry={onRefresh}
          actionLabel="Retry Connection"
        />
      )}

      {loading && (
        <div className="catalog-loading-wrapper">
          <LoadingSpinner message="Fetching live event catalog from API Gateway..." size="lg" />
        </div>
      )}

      {!loading && !error && filteredEvents.length === 0 && (
        <div className="empty-catalog-state">
          <p className="empty-title">Nothing matches those filters.</p>
          <p className="empty-copy">
            {hasActiveFilters
              ? 'Loosen a filter, or start over with the full line-up.'
              : 'The catalogue is empty right now. Try refreshing.'}
          </p>
          <div className="empty-actions">
            {hasActiveFilters && (
              <button type="button" onClick={handleResetFilters} className="btn-refresh cursor-pointer">
                Clear filters
              </button>
            )}
            <button type="button" onClick={onRefresh} className="btn-refresh cursor-pointer">
              <RefreshCwIcon size={16} className="mr-2" />
              Refresh catalogue
            </button>
          </div>
        </div>
      )}

      {!loading && filteredEvents.length > 0 && (
        <div className="events-grid">
          {filteredEvents.map((evt, idx) => (
            <EventCard
              key={evt.id}
              event={evt}
              index={idx}
              onSelectEvent={onSelectEvent}
              onJoinWaitlist={onJoinWaitlist}
              onViewReviews={onViewReviews}
              currentCurrency={currentCurrency}
            />
          ))}
        </div>
      )}
    </section>
  );
};
