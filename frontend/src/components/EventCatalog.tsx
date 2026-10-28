import React, { useState, useMemo, useEffect } from 'react';
import { EventItem } from '../types';
import { EventCard } from './EventCard';
import { SearchIcon, RefreshCwIcon, MapPinIcon, XIcon, SparklesIcon, TicketIcon, CalendarIcon, ChevronRightIcon, ShieldCheckIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';
import { AlertNotification } from './AlertNotification';

interface EventCatalogProps {
  events: EventItem[];
  loading: boolean;
  error: string | null;
  onRefresh: () => void;
  onSelectEvent: (event: EventItem) => void;
  onJoinWaitlist?: (event: EventItem) => void;
  onViewReviews?: (event: EventItem) => void;
}

export const EventCatalog: React.FC<EventCatalogProps> = ({
  events,
  loading,
  error,
  onRefresh,
  onSelectEvent,
  onJoinWaitlist,
  onViewReviews,
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('ALL');
  const [selectedCity, setSelectedCity] = useState<string>('ALL');
  const [selectedPriceRange, setSelectedPriceRange] = useState<string>('ALL');
  const [sortBy, setSortBy] = useState<string>('date_asc');
  const [featuredIndex, setFeaturedIndex] = useState<number>(0);

  // Featured events for the Premiere Hero Showcase (up to 4 active events)
  const featuredEvents = useMemo(() => {
    const active = events.filter((e) => e.status === 'ACTIVE');
    return (active.length > 0 ? active : events).slice(0, 4);
  }, [events]);

  // Auto-advance Premiere Hero Showcase every 6 seconds
  useEffect(() => {
    if (featuredEvents.length <= 1) return;
    const timer = setInterval(() => {
      setFeaturedIndex((prev) => (prev + 1) % featuredEvents.length);
    }, 6000);
    return () => clearInterval(timer);
  }, [featuredEvents.length]);

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
        {/* Ambient Cinema Aurora & Dual Projector Beams */}
        <div className="hero-aurora-backdrop" aria-hidden="true">
          <div className="aurora-orb aurora-orb-gold" />
          <div className="aurora-orb aurora-orb-indigo" />
          <div className="aurora-orb aurora-orb-purple" />
          <div className="imax-projector-beam beam-left" />
          <div className="imax-projector-beam beam-right" />
        </div>

        {/* Live Telemetry Ribbon */}
        <div className="premiere-live-ribbon" data-testid="premiere-live-ribbon">
          <span className="ribbon-badge">
            <span className="live-pulse-dot" />
            <span>IMAX & LIVE ARENA ENGINE</span>
          </span>
          <span className="ribbon-divider">•</span>
          <span className="ribbon-metric">
            <ShieldCheckIcon size={13} className="inline-icon mr-1 text-emerald-400" />
            Zero-Oversell Atomic Locks
          </span>
          <span className="ribbon-divider hidden sm:inline">•</span>
          <span className="ribbon-metric hidden sm:inline">
            <SparklesIcon size={13} className="inline-icon mr-1 text-amber-400" />
            Holographic QR Passes
          </span>
        </div>

        <h2 className="hero-title">Experience Live Entertainment</h2>
        <p className="hero-subtitle">
          Book authentic seats in real time with our atomic high-concurrency ticket reservation engine.
        </p>

        {/* Featured Premiere Spotlight Showcase Carousel */}
        {currentFeatured && !loading && (
          <div
            className="premiere-showcase-card"
            data-testid="premiere-hero-showcase"
          >
            <div className="premiere-showcase-glow" aria-hidden="true" />
            <div className="premiere-showcase-content">
              <div className="premiere-showcase-left">
                <div className="flex items-center gap-2 flex-wrap mb-2">
                  <span className="premiere-spotlight-pill">
                    <SparklesIcon size={12} className="mr-1" />
                    FEATURED PREMIERE #{featuredIndex + 1}
                  </span>
                  <span className="premiere-category-tag">{currentFeatured.category}</span>
                  <span className="premiere-venue-tag">
                    <MapPinIcon size={12} className="mr-1 text-amber-400" />
                    {currentFeatured.venueName} ({currentFeatured.venueLocation})
                  </span>
                </div>

                <h3 className="premiere-showcase-title" key={currentFeatured.id}>
                  {currentFeatured.title}
                </h3>
                <p className="premiere-showcase-desc line-clamp-2">
                  {currentFeatured.description}
                </p>

                <div className="premiere-showcase-meta">
                  <span className="flex items-center gap-1.5 text-xs text-slate-300">
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
                      From ${Math.min(...currentFeatured.tiers.map((t) => t.price)).toFixed(2)}
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
                  <span>Enter Interactive Arena</span>
                  <ChevronRightIcon size={16} />
                </button>

                {/* Carousel Slide Indicators */}
                {featuredEvents.length > 1 && (
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
                )}
              </div>
            </div>
          </div>
        )}

        <div className="search-and-filter-bar space-y-3">
          {/* Search Input & Sort Dropdown */}
          <div className="catalog-search-row flex flex-col sm:flex-row gap-2">
            <div className="search-input-wrapper flex-1">
              <SearchIcon size={18} className="search-icon" />
              <input
                type="text"
                placeholder="Search by event title, venue, or artist..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="search-input"
                data-testid="catalog-search-input"
              />
            </div>

            <div className="flex items-center gap-2">
              <select
                value={sortBy}
                onChange={(e) => setSortBy(e.target.value)}
                className="catalog-sort-select px-3 py-2 rounded-xl bg-slate-900 border border-slate-700 text-xs text-slate-200 focus:outline-none focus:border-amber-400 cursor-pointer"
                data-testid="catalog-sort-select"
                aria-label="Sort events"
              >
                <option value="date_asc">📅 Date: Soonest</option>
                <option value="date_desc">📅 Date: Furthest</option>
                <option value="price_asc">💵 Price: Low to High</option>
                <option value="price_desc">💎 Price: High to Low</option>
                <option value="title_asc">🔤 Title: A-Z</option>
              </select>

              {hasActiveFilters && (
                <button
                  type="button"
                  onClick={handleResetFilters}
                  className="btn-reset-filters px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-rose-300 text-xs font-semibold flex items-center gap-1 cursor-pointer transition-colors"
                  data-testid="reset-filters-btn"
                >
                  <XIcon size={14} />
                  <span>Clear</span>
                </button>
              )}
            </div>
          </div>

          {/* Category Facets */}
          <div className="category-pills" role="tablist" aria-label="Event category filter">
            {categories.map((cat) => (
              <button
                key={cat}
                type="button"
                role="tab"
                aria-selected={selectedCategory === cat}
                className={`category-pill cursor-pointer ${selectedCategory === cat ? 'active' : ''}`}
                onClick={() => setSelectedCategory(cat)}
                data-testid={`category-pill-${cat}`}
              >
                {cat}
              </button>
            ))}
          </div>

          {/* City / Geolocation Facets */}
          {cities.length > 2 && (
            <div className="city-facets-bar flex items-center gap-1.5 overflow-x-auto pb-1 text-xs" data-testid="city-facets-bar">
              <span className="city-facets-label text-slate-400 font-semibold flex items-center gap-1 pl-1 pr-2 whitespace-nowrap">
                <MapPinIcon size={12} className="text-amber-400" />
                <span>Cities:</span>
              </span>
              {cities.map((city) => (
                <button
                  key={city}
                  type="button"
                  onClick={() => setSelectedCity(city)}
                  className={`city-pill ${selectedCity === city ? 'active' : ''} px-2.5 py-1 rounded-full text-[11px] font-medium whitespace-nowrap cursor-pointer transition-colors ${
                    selectedCity === city
                      ? 'bg-cyan-500 text-slate-950 font-bold shadow-sm'
                      : 'bg-slate-900/80 hover:bg-slate-800 text-slate-300 border border-slate-800'
                  }`}
                  data-testid={`city-pill-${city.replace(/\s+/g, '-').toLowerCase()}`}
                >
                  {city === 'ALL' ? 'All Locations' : city}
                </button>
              ))}
            </div>
          )}

          {/* Active Filter Summary Bar */}
          <div className="catalog-summary-bar flex items-center justify-between text-xs text-slate-400 pt-1 border-t border-slate-800/60">
            <span>
              Showing <strong>{filteredEvents.length}</strong> of {events.length} live events
            </span>
            <div className="budget-pills-group flex items-center gap-2">
              <span className="text-[11px] text-slate-500">Budget:</span>
              {[
                { id: 'ALL', label: 'Any' },
                { id: 'UNDER_100', label: '< $100' },
                { id: '100_TO_200', label: '$100-$200' },
                { id: 'OVER_200', label: '$200+' },
              ].map((b) => (
                <button
                  key={b.id}
                  type="button"
                  onClick={() => setSelectedPriceRange(b.id)}
                  className={`budget-pill ${selectedPriceRange === b.id ? 'active' : ''} px-2 py-0.5 rounded text-[11px] cursor-pointer transition-colors ${
                    selectedPriceRange === b.id
                      ? 'bg-amber-500/20 text-amber-300 font-bold border border-amber-500/40'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                  data-testid={`price-pill-${b.id}`}
                >
                  {b.label}
                </button>
              ))}
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
          <p className="empty-title">No events found matching your criteria.</p>
          <button type="button" onClick={onRefresh} className="btn-refresh cursor-pointer">
            <RefreshCwIcon size={16} className="mr-2" />
            Refresh Catalog
          </button>
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
            />
          ))}
        </div>
      )}
    </section>
  );
};
