import React from 'react';
import { EventItem } from '../types';
import { CalendarIcon, MapPinIcon, TicketIcon, FilmIcon, SparklesIcon } from './Icons';

interface EventCardProps {
  event: EventItem;
  onSelectEvent: (event: EventItem) => void;
}

export const EventCard: React.FC<EventCardProps> = ({ event, onSelectEvent }) => {
  // Format date nicely
  const formatDate = (dateStr: string) => {
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString('en-US', {
        weekday: 'short',
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  };

  // Find lowest price
  const lowestPrice = event.tiers.length > 0
    ? Math.min(...event.tiers.map((t) => t.price))
    : 0;

  // Cinematic theme category styling
  const categoryLower = event.category.toLowerCase();
  const isCinema = categoryLower.includes('cinema') || categoryLower.includes('movie');
  const isTheater = categoryLower.includes('theater') || categoryLower.includes('play');

  return (
    <div
      className="event-card cursor-pointer"
      onClick={() => onSelectEvent(event)}
      tabIndex={0}
      role="button"
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelectEvent(event);
        }
      }}
      aria-label={`Select seats for ${event.title}`}
    >
      {/* Cinematic Poster Header */}
      <div className={`event-poster-banner banner-${categoryLower}`}>
        <div className="poster-spotlight-beam"></div>
        <div className="poster-icon-watermark">
          {isCinema ? <FilmIcon size={56} /> : isTheater ? <SparklesIcon size={56} /> : <TicketIcon size={56} />}
        </div>
        <div className="event-card-header">
          <span className="event-category-badge">
            <SparklesIcon size={12} className="mr-1 inline-icon" />
            {event.category}
          </span>
          <span className={`event-status-badge status-${event.status.toLowerCase()}`}>
            {event.status}
          </span>
        </div>
      </div>

      <div className="event-card-body">
        <h3 className="event-title">{event.title}</h3>
        <p className="event-description">{event.description}</p>

        <div className="event-meta-list">
          <div className="event-meta-item">
            <CalendarIcon size={16} className="text-secondary" />
            <span>{formatDate(event.startTime)}</span>
          </div>
          <div className="event-meta-item">
            <MapPinIcon size={16} className="text-secondary" />
            <span>
              {event.venueName} • {event.venueLocation}
            </span>
          </div>
        </div>

        <div className="event-tiers-preview">
          <h4 className="tiers-title">Ticket Tiers:</h4>
          <div className="tiers-tags">
            {event.tiers.map((tier) => (
              <span key={tier.id} className="tier-tag">
                <span className="tier-tag-name">{tier.name}</span>
                <span className="tier-tag-price">${tier.price.toFixed(2)}</span>
              </span>
            ))}
          </div>
        </div>
      </div>

      <div className="event-card-footer">
        <div className="price-starting-box">
          <span className="price-label">Starting at</span>
          <span className="price-amount">${lowestPrice.toFixed(2)}</span>
        </div>

        <button
          type="button"
          className="btn-select-seats cursor-pointer"
          onClick={(e) => {
            e.stopPropagation();
            onSelectEvent(event);
          }}
          aria-label={`Select Seats for ${event.title}`}
        >
          <TicketIcon size={18} className="mr-2" />
          Select Seats
        </button>
      </div>
    </div>
  );
};
