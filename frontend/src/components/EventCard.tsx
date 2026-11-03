import React, { useState } from 'react';
import { EventItem } from '../types';
import { CalendarIcon, MapPinIcon, TicketIcon, FilmIcon, SparklesIcon, ClockIcon, StarIcon } from './Icons';
import { calculateSeatScarcity } from '../utils/motion';
import { formatCurrency } from '../services/currency';

interface EventCardProps {
  event: EventItem;
  index?: number;
  onSelectEvent: (event: EventItem) => void;
  onJoinWaitlist?: (event: EventItem) => void;
  onViewReviews?: (event: EventItem) => void;
  currentCurrency?: string;
}

export const EventCard: React.FC<EventCardProps> = ({
  event,
  onSelectEvent,
  onJoinWaitlist,
  onViewReviews,
  currentCurrency = 'USD',
}) => {
  const [imageFailed, setImageFailed] = useState(false);
  const hasPoster = Boolean(event.imageUrl) && !imageFailed;

  const formatDate = (dateStr: string) => {
    try {
      const d = new Date(dateStr);
      return d.toLocaleDateString('en-US', {
        weekday: 'short',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  };

  const lowestPrice = event.tiers.length > 0
    ? Math.min(...event.tiers.map((t) => t.price))
    : 0;

  const scarcity = calculateSeatScarcity(event.tiers);
  const totalAvailable = event.tiers.reduce((acc, t) => acc + (t.availableSeats ?? 0), 0);
  const isSoldOut = event.status === 'SOLD_OUT' || (event.tiers.length > 0 && totalAvailable === 0);

  const categoryLower = event.category.toLowerCase();
  const isCinema = categoryLower.includes('cinema') || categoryLower.includes('movie');
  const isTheater = categoryLower.includes('theater') || categoryLower.includes('play');

  const handleCardClick = () => {
    if (isSoldOut && onJoinWaitlist) {
      onJoinWaitlist(event);
    } else {
      onSelectEvent(event);
    }
  };

  return (
    <article
      className="event-card"
      onClick={handleCardClick}
      aria-labelledby={`event-title-${event.id}`}
      data-testid={`event-card-${event.id}`}
    >
      <div className={`event-poster-banner banner-${categoryLower}${hasPoster ? ' has-image' : ''}`}>
        {hasPoster && (
          <img
            className="event-poster-image"
            src={event.imageUrl}
            alt={`${event.title} at ${event.venueName}`}
            loading="lazy"
            decoding="async"
            onError={() => setImageFailed(true)}
          />
        )}
        {!hasPoster && (
          <div className="poster-icon-watermark" aria-hidden="true">
            {isCinema ? <FilmIcon size={56} /> : isTheater ? <SparklesIcon size={56} /> : <TicketIcon size={56} />}
          </div>
        )}
        <div className="event-card-header">
          <span className="event-category-badge">{event.category}</span>
          {isSoldOut ? (
            <span className="event-status-badge status-sold-out">Waitlist open</span>
          ) : (
            <span className={`event-status-badge status-${event.status.toLowerCase()}`}>
              {event.status}
            </span>
          )}
        </div>
      </div>

      <div className="event-card-body">
        <h3 className="event-title" id={`event-title-${event.id}`}>{event.title}</h3>
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

        {event.tiers.length > 0 && (
          <div className="event-scarcity-widget" data-testid={`event-scarcity-${event.id}`}>
            <div className="scarcity-header">
              <span className="scarcity-label">
                {isSoldOut ? (
                  <span className="text-cyan-300 font-semibold">Sold out — waitlist open</span>
                ) : scarcity.isHighDemand ? (
                  <span className="text-amber-300 font-semibold">{totalAvailable} seats left</span>
                ) : (
                  <span className="text-emerald-300 font-medium">{totalAvailable} seats available</span>
                )}
              </span>
              <span className="scarcity-percent">{isSoldOut ? '100%' : `${Math.max(12, scarcity.soldPercent)}% booked`}</span>
            </div>
            <div className="scarcity-track">
              <div
                className={`scarcity-fill ${isSoldOut ? 'fill-soldout' : scarcity.isHighDemand ? 'fill-hot' : 'fill-normal'}`}
                style={{ width: `${isSoldOut ? 100 : Math.max(14, scarcity.soldPercent)}%` }}
              />
            </div>
          </div>
        )}

        <div className="event-tiers-preview">
          <h4 className="tiers-title">Ticket tiers</h4>
          <div className="tiers-tags">
            {event.tiers.map((tier) => (
              <span key={tier.id} className="tier-tag">
                <span className="tier-tag-name">{tier.name}</span>
                <span className="tier-tag-price">{formatCurrency(tier.price, currentCurrency)}</span>
              </span>
            ))}
          </div>
        </div>
      </div>

      <div className="event-card-footer">
        <div className="price-starting-box">
          <span className="price-label">From</span>
          <span className="price-amount">{formatCurrency(lowestPrice, currentCurrency)}</span>
        </div>

        <div className="event-card-actions">
          {onViewReviews && (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onViewReviews(event);
              }}
              className="btn-reviews-quiet cursor-pointer"
              data-testid={`btn-event-reviews-${event.id}`}
            >
              <StarIcon size={13} filled />
              <span>Reviews</span>
            </button>
          )}

          {isSoldOut ? (
            <button
              type="button"
              className="btn-select-seats btn-waitlist cursor-pointer"
              onClick={(e) => {
                e.stopPropagation();
                if (onJoinWaitlist) {
                  onJoinWaitlist(event);
                } else {
                  onSelectEvent(event);
                }
              }}
              aria-label={`Join the waitlist for ${event.title}`}
            >
              <ClockIcon size={17} className="mr-2" />
              Join waitlist
            </button>
          ) : (
            <button
              type="button"
              className="btn-select-seats cursor-pointer"
              onClick={(e) => {
                e.stopPropagation();
                onSelectEvent(event);
              }}
              aria-label={`Choose seats for ${event.title}`}
            >
              <TicketIcon size={17} className="mr-2" />
              Choose seats
            </button>
          )}
        </div>
      </div>
    </article>
  );
};
