import React, { useState, useEffect, useCallback } from 'react';
import { EventItem, User, EventReview, ReviewSummary } from '../types';
import { fetchEventReviews, submitEventReview, voteHelpfulReview } from '../services/api';
import {
  XIcon,
  StarIcon,
  ThumbsUpIcon,
  CheckCircleIcon,
  AlertCircleIcon,
} from './Icons';
import { LoadingSpinner } from './LoadingSpinner';

interface ReviewModalProps {
  event: EventItem;
  isOpen: boolean;
  currentUser: User | null;
  onClose: () => void;
}

export const ReviewModal: React.FC<ReviewModalProps> = ({
  event,
  isOpen,
  currentUser,
  onClose,
}) => {
  const [reviews, setReviews] = useState<EventReview[]>([]);
  const [summary, setSummary] = useState<ReviewSummary>({
    averageRating: 0,
    totalReviews: 0,
    ratingDistribution: { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 },
  });
  const [loading, setLoading] = useState(true);
  const [selectedRatingFilter, setSelectedRatingFilter] = useState<number | 'ALL'>('ALL');
  const [verifiedOnlyFilter, setVerifiedOnlyFilter] = useState(false);

  // Submission Form State
  const [showForm, setShowForm] = useState(false);
  const [rating, setRating] = useState<number>(5);
  const [hoverRating, setHoverRating] = useState<number | null>(null);
  const [title, setTitle] = useState('');
  const [comment, setComment] = useState('');
  const [reviewerName, setReviewerName] = useState(currentUser?.name || '');
  const [reviewerEmail, setReviewerEmail] = useState(currentUser?.email || '');
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [formSuccess, setFormSuccess] = useState<string | null>(null);

  // Upvoted reviews tracking
  const [votedReviews, setVotedReviews] = useState<Set<string>>(new Set());

  const loadReviews = useCallback(async () => {
    setLoading(true);
    try {
      const data = await fetchEventReviews(event.id);
      setReviews(data.reviews || []);
      setSummary(data.summary || {
        averageRating: 0,
        totalReviews: 0,
        ratingDistribution: { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 },
      });
    } catch (err) {
      console.warn('Failed to load reviews:', err);
    } finally {
      setLoading(false);
    }
  }, [event.id]);

  useEffect(() => {
    if (isOpen) {
      loadReviews();
      if (currentUser) {
        setReviewerName(currentUser.name);
        setReviewerEmail(currentUser.email);
      }
    }
  }, [isOpen, loadReviews, currentUser]);

  if (!isOpen) return null;

  const handleSubmitReview = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reviewerName.trim()) {
      setFormError('Please enter your name');
      return;
    }
    if (!reviewerEmail.trim()) {
      setFormError('Please enter your email');
      return;
    }
    if (!comment.trim()) {
      setFormError('Please write a comment or review');
      return;
    }

    setSubmitting(true);
    setFormError(null);
    setFormSuccess(null);

    try {
      const created = await submitEventReview({
        eventId: event.id,
        userId: currentUser?.id,
        reviewerName: reviewerName.trim(),
        reviewerEmail: reviewerEmail.trim(),
        rating,
        title: title.trim() || `${rating}-Star Experience`,
        comment: comment.trim(),
      });

      setReviews((prev) => [created, ...prev]);
      setFormSuccess('Thank you! Your verified review has been published.');
      setTitle('');
      setComment('');
      setShowForm(false);
      // Refresh summary
      loadReviews();
    } catch (err: any) {
      setFormError(err.message || 'Failed to submit review');
    } finally {
      setSubmitting(false);
    }
  };

  const handleVote = async (reviewId: string) => {
    if (votedReviews.has(reviewId)) return;
    setVotedReviews((prev) => new Set(prev).add(reviewId));
    setReviews((prev) =>
      prev.map((r) => (r.id === reviewId ? { ...r, helpfulVotes: r.helpfulVotes + 1 } : r))
    );
    try {
      await voteHelpfulReview(reviewId);
    } catch {
      // Ignore background vote failure
    }
  };

  const filteredReviews = reviews.filter((r) => {
    if (selectedRatingFilter !== 'ALL' && r.rating !== selectedRatingFilter) {
      return false;
    }
    if (verifiedOnlyFilter && !r.verifiedBuyer) {
      return false;
    }
    return true;
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md animate-in fade-in duration-200">
      <div
        className="relative w-full max-w-2xl max-h-[90vh] flex flex-col rounded-3xl bg-slate-950/95 border border-slate-800 shadow-2xl shadow-indigo-950/50 overflow-hidden"
        role="dialog"
        aria-modal="true"
      >
        {/* Modal Header */}
        <div className="p-6 border-b border-slate-800/80 flex items-start justify-between bg-gradient-to-r from-amber-950/20 via-slate-900 to-indigo-950/20">
          <div>
            <span className="text-[10px] font-mono uppercase tracking-widest text-amber-400 font-bold block mb-1">
              Attendee Reviews & Social Proof
            </span>
            <h3 className="text-xl font-bold text-slate-100 font-display">{event.title}</h3>
            <p className="text-xs text-slate-400 mt-0.5">{event.venueName} • {event.venueLocation}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-2 rounded-xl text-slate-400 hover:text-slate-100 hover:bg-slate-800/60 transition-colors cursor-pointer"
            aria-label="Close review modal"
          >
            <XIcon size={20} />
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-6 overflow-y-auto space-y-6 flex-1">
          {/* Rating Summary Scorecard */}
          <div className="p-5 rounded-2xl bg-slate-900/80 border border-slate-800 flex flex-col sm:flex-row items-center gap-6">
            <div className="text-center sm:text-left sm:pr-6 sm:border-r sm:border-slate-800 flex flex-col items-center sm:items-start">
              <span className="text-4xl font-extrabold text-amber-300 font-mono tracking-tight" data-testid="review-avg-rating">
                {summary.averageRating > 0 ? summary.averageRating.toFixed(1) : '5.0'}
              </span>
              <div className="flex items-center gap-1 my-1 text-amber-400">
                {[1, 2, 3, 4, 5].map((star) => (
                  <StarIcon
                    key={star}
                    size={16}
                    filled={star <= Math.round(summary.averageRating || 5)}
                    className="text-amber-400"
                  />
                ))}
              </div>
              <span className="text-xs text-slate-400" data-testid="review-total-count">
                Based on {summary.totalReviews} verified reviews
              </span>
            </div>

            {/* Distribution Bars */}
            <div className="flex-1 w-full space-y-1.5 text-xs">
              {[5, 4, 3, 2, 1].map((stars) => {
                const count = summary.ratingDistribution[stars] || 0;
                const percent = summary.totalReviews > 0 ? (count / summary.totalReviews) * 100 : 0;
                return (
                  <div key={stars} className="flex items-center gap-2">
                    <span className="w-8 text-slate-400 font-mono text-right">{stars}★</span>
                    <div className="flex-1 h-2 rounded-full bg-slate-800 overflow-hidden">
                      <div
                        className="h-full bg-amber-400 rounded-full transition-all duration-500"
                        style={{ width: `${percent}%` }}
                      ></div>
                    </div>
                    <span className="w-6 text-slate-500 font-mono text-[11px]">{count}</span>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Action Bar: Write Review & Filters */}
          <div className="flex flex-wrap items-center justify-between gap-3 pt-2">
            <div className="flex items-center gap-1.5 flex-wrap">
              <button
                type="button"
                onClick={() => setSelectedRatingFilter('ALL')}
                className={`px-3 py-1 rounded-lg text-xs font-semibold cursor-pointer transition-colors ${
                  selectedRatingFilter === 'ALL'
                    ? 'bg-amber-500 text-slate-950 font-bold'
                    : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
                }`}
              >
                All
              </button>
              {[5, 4, 3, 2, 1].map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => setSelectedRatingFilter(s)}
                  className={`px-2.5 py-1 rounded-lg text-xs font-semibold cursor-pointer transition-colors ${
                    selectedRatingFilter === s
                      ? 'bg-amber-500 text-slate-950 font-bold'
                      : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
                  }`}
                >
                  {s}★
                </button>
              ))}
              <button
                type="button"
                onClick={() => setVerifiedOnlyFilter(!verifiedOnlyFilter)}
                className={`px-3 py-1 rounded-lg text-xs font-semibold cursor-pointer transition-colors flex items-center gap-1 ${
                  verifiedOnlyFilter
                    ? 'bg-emerald-500 text-slate-950 font-bold'
                    : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
                }`}
              >
                <CheckCircleIcon size={12} />
                <span>Verified Only</span>
              </button>
            </div>

            <button
              type="button"
              onClick={() => setShowForm(!showForm)}
              className="px-4 py-1.5 rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 hover:from-amber-400 hover:to-amber-500 text-slate-950 font-bold text-xs shadow-md shadow-amber-950/40 cursor-pointer transition-all"
              data-testid="toggle-review-form-btn"
            >
              {showForm ? 'Cancel Review' : '✍️ Write Review'}
            </button>
          </div>

          {/* Write Review Form */}
          {showForm && (
            <form
              onSubmit={handleSubmitReview}
              className="p-5 rounded-2xl bg-slate-900 border border-amber-500/40 space-y-4 animate-in fade-in duration-150"
              data-testid="review-submission-form"
            >
              <h4 className="text-sm font-bold text-slate-100 flex items-center gap-1.5">
                <span>Rate your live experience for {event.title}</span>
              </h4>

              {/* Star Rating Selector */}
              <div>
                <label className="block text-xs text-slate-400 mb-1">Your Rating</label>
                <div className="flex items-center gap-2">
                  {[1, 2, 3, 4, 5].map((star) => {
                    const active = (hoverRating !== null ? hoverRating : rating) >= star;
                    return (
                      <button
                        key={star}
                        type="button"
                        onClick={() => setRating(star)}
                        onMouseEnter={() => setHoverRating(star)}
                        onMouseLeave={() => setHoverRating(null)}
                        className="p-1 text-slate-600 hover:text-amber-400 cursor-pointer transition-colors"
                        aria-label={`Rate ${star} star`}
                      >
                        <StarIcon
                          size={24}
                          filled={active}
                          className={active ? 'text-amber-400 scale-110' : 'text-slate-600'}
                        />
                      </button>
                    );
                  })}
                  <span className="font-mono text-xs text-amber-300 ml-2 font-bold">
                    {rating} of 5 Stars
                  </span>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label htmlFor="rev-name" className="block text-xs text-slate-400 mb-1">
                    Your Name
                  </label>
                  <input
                    id="rev-name"
                    type="text"
                    required
                    value={reviewerName}
                    onChange={(e) => setReviewerName(e.target.value)}
                    placeholder="e.g. Jane Doe"
                    className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs text-slate-200 focus:outline-none focus:border-amber-500"
                  />
                </div>
                <div>
                  <label htmlFor="rev-email" className="block text-xs text-slate-400 mb-1">
                    Email Address (for Verified Buyer Badge)
                  </label>
                  <input
                    id="rev-email"
                    type="email"
                    required
                    value={reviewerEmail}
                    onChange={(e) => setReviewerEmail(e.target.value)}
                    placeholder="e.g. jane@example.com"
                    className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs text-slate-200 focus:outline-none focus:border-amber-500"
                  />
                </div>
              </div>

              <div>
                <label htmlFor="rev-title" className="block text-xs text-slate-400 mb-1">
                  Headline / Title
                </label>
                <input
                  id="rev-title"
                  type="text"
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder="e.g. Unbelievable performance and great acoustics"
                  className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs text-slate-200 focus:outline-none focus:border-amber-500"
                />
              </div>

              <div>
                <label htmlFor="rev-comment" className="block text-xs text-slate-400 mb-1">
                  Detailed Review & Feedback
                </label>
                <textarea
                  id="rev-comment"
                  rows={3}
                  required
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder="Share details on venue access, audio quality, view from seats, and VIP perks..."
                  className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs text-slate-200 focus:outline-none focus:border-amber-500 resize-none"
                ></textarea>
              </div>

              {formError && (
                <div className="p-2.5 rounded-lg bg-rose-950/40 border border-rose-500/40 text-rose-300 text-xs flex items-center gap-1.5">
                  <AlertCircleIcon size={14} />
                  <span>{formError}</span>
                </div>
              )}

              <div className="flex justify-end gap-2 pt-1">
                <button
                  type="button"
                  onClick={() => setShowForm(false)}
                  className="px-4 py-2 rounded-xl bg-slate-800 text-slate-300 text-xs font-semibold cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="px-5 py-2 rounded-xl bg-amber-500 hover:bg-amber-400 text-slate-950 text-xs font-bold disabled:opacity-50 cursor-pointer"
                >
                  {submitting ? 'Publishing...' : 'Submit Verified Review'}
                </button>
              </div>
            </form>
          )}

          {formSuccess && (
            <div className="p-3 rounded-xl bg-emerald-950/40 border border-emerald-500/40 text-emerald-300 text-xs flex items-center gap-2">
              <CheckCircleIcon size={16} />
              <span>{formSuccess}</span>
            </div>
          )}

          {/* Reviews List */}
          {loading ? (
            <div className="py-12 text-center text-slate-400 text-xs">
              <LoadingSpinner size="sm" />
              <p className="mt-2">Loading verified attendee reviews...</p>
            </div>
          ) : filteredReviews.length === 0 ? (
            <div className="py-12 text-center text-slate-400 text-xs space-y-2">
              <p className="text-slate-300 font-semibold">No reviews matching your filter</p>
              <p>Be the first to share your experience from this event!</p>
            </div>
          ) : (
            <div className="space-y-3" data-testid="reviews-list">
              {filteredReviews.map((rev) => {
                const hasVoted = votedReviews.has(rev.id);
                return (
                  <div
                    key={rev.id}
                    className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800 hover:border-slate-700/80 transition-all space-y-2"
                    data-testid={`review-card-${rev.id}`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-xs text-slate-100">{rev.reviewerName}</span>
                          {rev.verifiedBuyer && (
                            <span
                              className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 flex items-center gap-1"
                              data-testid="verified-buyer-badge"
                            >
                              <CheckCircleIcon size={10} />
                              Verified Attendee
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-1 mt-1 text-amber-400">
                          {[1, 2, 3, 4, 5].map((s) => (
                            <StarIcon
                              key={s}
                              size={12}
                              filled={s <= rev.rating}
                              className={s <= rev.rating ? 'text-amber-400' : 'text-slate-700'}
                            />
                          ))}
                          <span className="text-[11px] text-slate-500 ml-1">
                            {new Date(rev.createdAt).toLocaleDateString()}
                          </span>
                        </div>
                      </div>

                      {/* Helpful Button */}
                      <button
                        type="button"
                        onClick={() => handleVote(rev.id)}
                        disabled={hasVoted}
                        className={`px-2.5 py-1 rounded-lg text-[11px] font-medium flex items-center gap-1.5 transition-colors cursor-pointer ${
                          hasVoted
                            ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                            : 'bg-slate-800 text-slate-400 hover:text-slate-200 hover:bg-slate-700'
                        }`}
                        title="Mark this review as helpful"
                        data-testid={`helpful-vote-btn-${rev.id}`}
                      >
                        <ThumbsUpIcon size={12} />
                        <span>Helpful ({rev.helpfulVotes})</span>
                      </button>
                    </div>

                    <h5 className="text-xs font-bold text-slate-200 pt-0.5">{rev.title}</h5>
                    <p className="text-xs text-slate-400 leading-relaxed">{rev.comment}</p>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="p-4 border-t border-slate-800/80 bg-slate-900/40 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold cursor-pointer transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
