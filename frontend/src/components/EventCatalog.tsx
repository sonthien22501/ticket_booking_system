import React, { useState, useMemo } from 'react';
import { EventItem } from '../types';
import { EventCard } from './EventCard';
import { SearchIcon, RefreshCwIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';
import { AlertNotification } from './AlertNotification';

interface EventCatalogProps {
  events: EventItem[];
  loading: boolean;
  error: string | null;
  onRefresh: () => void;
  onSelectEvent: (event: EventItem) => void;
}

export const EventCatalog: React.FC<EventCatalogProps> = ({
  events,
  loading,
  error,
  onRefresh,
  onSelectEvent,
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('ALL');

  // Extract unique categories
  const categories = useMemo(() => {
    const set = new Set<string>();
    events.forEach((e) => set.add(e.category));
    return ['ALL', ...Array.from(set)];
  }, [events]);

  // Filter events based on search and category
  const filteredEvents = useMemo(() => {
    return events.filter((e) => {
      const matchesSearch =
        e.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        e.venueName.toLowerCase().includes(searchQuery.toLowerCase()) ||
        e.description.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesCat =
        selectedCategory === 'ALL' || e.category.toUpperCase() === selectedCategory.toUpperCase();

      return matchesSearch && matchesCat;
    });
  }, [events, searchQuery, selectedCategory]);

  return (
    <section className="catalog-section">
      <div className="hero-banner">
        <h2 className="hero-title">Experience Live Entertainment</h2>
        <p className="hero-subtitle">
          Book authentic seats in real time with our atomic high-concurrency ticket reservation engine.
        </p>

        <div className="search-and-filter-bar">
          <div className="search-input-wrapper">
            <SearchIcon size={18} className="search-icon" />
            <input
              type="text"
              placeholder="Search by event title, venue, or artist..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="search-input"
            />
          </div>

          <div className="category-pills">
            {categories.map((cat) => (
              <button
                key={cat}
                type="button"
                className={`category-pill ${selectedCategory === cat ? 'active' : ''}`}
                onClick={() => setSelectedCategory(cat)}
              >
                {cat}
              </button>
            ))}
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
          <button type="button" onClick={onRefresh} className="btn-refresh">
            <RefreshCwIcon size={16} className="mr-2" />
            Refresh Catalog
          </button>
        </div>
      )}

      {!loading && filteredEvents.length > 0 && (
        <div className="events-grid">
          {filteredEvents.map((evt) => (
            <EventCard key={evt.id} event={evt} onSelectEvent={onSelectEvent} />
          ))}
        </div>
      )}
    </section>
  );
};
