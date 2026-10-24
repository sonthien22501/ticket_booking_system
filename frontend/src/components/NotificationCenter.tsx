import React, { useState } from 'react';
import { AppNotification } from '../types';
import { markNotificationRead, markAllNotificationsRead } from '../services/api';
import {
  BellIcon,
  CheckCircleIcon,
  AlertCircleIcon,
  TicketIcon,
  ClockIcon,
  XIcon,
  CheckIcon,
} from './Icons';

interface NotificationCenterProps {
  isOpen: boolean;
  onClose: () => void;
  currentUser: { id?: string; email?: string; name?: string } | null;
  unreadCount: number;
  notifications: AppNotification[];
  onRefresh: () => void;
  onOpenWaitlist?: () => void;
  onOpenBookings?: () => void;
}

export const NotificationCenter: React.FC<NotificationCenterProps> = ({
  isOpen,
  onClose,
  currentUser,
  unreadCount,
  notifications,
  onRefresh,
  onOpenWaitlist,
  onOpenBookings,
}) => {
  const [markingAll, setMarkingAll] = useState(false);

  if (!isOpen) return null;

  const handleMarkRead = async (id: string) => {
    try {
      await markNotificationRead(id);
      onRefresh();
    } catch (err) {
      console.error('Failed to mark notification read:', err);
    }
  };

  const handleMarkAllRead = async () => {
    if (!currentUser?.email && !currentUser?.id) return;
    setMarkingAll(true);
    try {
      await markAllNotificationsRead(currentUser.email, currentUser.id);
      onRefresh();
    } catch (err) {
      console.error('Failed to mark all notifications read:', err);
    } finally {
      setMarkingAll(false);
    }
  };

  const formatRelativeTime = (isoString: string) => {
    try {
      const now = new Date();
      const past = new Date(isoString);
      const diffMs = now.getTime() - past.getTime();
      const diffMin = Math.floor(diffMs / (1000 * 60));
      if (diffMin < 1) return 'Just now';
      if (diffMin < 60) return `${diffMin}m ago`;
      const diffHrs = Math.floor(diffMin / 60);
      if (diffHrs < 24) return `${diffHrs}h ago`;
      const diffDays = Math.floor(diffHrs / 24);
      return `${diffDays}d ago`;
    } catch {
      return '';
    }
  };

  const renderTypeIcon = (type: string) => {
    switch (type) {
      case 'WAITLIST_PROMOTED':
        return <ClockIcon size={18} className="text-amber-400" />;
      case 'TICKET_TRANSFERRED':
        return <TicketIcon size={18} className="text-cyan-400" />;
      case 'BOOKING_CONFIRMED':
        return <CheckCircleIcon size={18} className="text-emerald-400" />;
      case 'SYSTEM_ALERT':
      default:
        return <AlertCircleIcon size={18} className="text-purple-400" />;
    }
  };

  return (
    <div className="fixed inset-0 z-50 overflow-hidden bg-black/60 backdrop-blur-sm flex justify-end animate-fade-in" onClick={onClose}>
      <div
        className="w-full max-w-md bg-slate-900/95 border-l border-slate-700/60 shadow-2xl h-full flex flex-col text-slate-100"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/40">
          <div className="flex items-center gap-2.5">
            <div className="relative p-2 rounded-xl bg-slate-800/80 text-accent">
              <BellIcon size={20} />
              {unreadCount > 0 && (
                <span className="absolute -top-1 -right-1 w-5 h-5 rounded-full bg-rose-500 text-white text-[11px] font-bold flex items-center justify-center shadow-lg shadow-rose-500/50">
                  {unreadCount > 9 ? '9+' : unreadCount}
                </span>
              )}
            </div>
            <div>
              <h3 className="font-semibold text-base text-slate-100">Activity Center</h3>
              <p className="text-xs text-slate-400">
                {unreadCount === 0 ? 'All caught up' : `${unreadCount} unread notification${unreadCount === 1 ? '' : 's'}`}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {unreadCount > 0 && (
              <button
                onClick={handleMarkAllRead}
                disabled={markingAll}
                className="text-xs px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
                title="Mark all as read"
              >
                <CheckIcon size={13} />
                <span>Mark read</span>
              </button>
            )}
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
              aria-label="Close activity center"
            >
              <XIcon size={18} />
            </button>
          </div>
        </div>

        {/* Notifications List */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {notifications.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center p-8 text-slate-500">
              <div className="w-14 h-14 rounded-2xl bg-slate-800/40 border border-slate-700/40 flex items-center justify-center mb-3 text-slate-600">
                <BellIcon size={28} />
              </div>
              <p className="text-sm font-medium text-slate-400">No Notifications</p>
              <p className="text-xs text-slate-500 mt-1 max-w-[220px]">
                Updates regarding your bookings, tickets, and waitlist requests will appear here.
              </p>
            </div>
          ) : (
            notifications.map((item) => (
              <div
                key={item.id}
                onClick={() => !item.isRead && handleMarkRead(item.id)}
                className={`p-3.5 rounded-xl border transition-all duration-200 cursor-pointer ${
                  !item.isRead
                    ? 'bg-slate-800/70 border-accent/30 shadow-md shadow-accent/5'
                    : 'bg-slate-900/40 border-slate-800/60 opacity-80 hover:opacity-100 hover:bg-slate-850'
                }`}
              >
                <div className="flex items-start gap-3">
                  <div className="mt-0.5 p-2 rounded-lg bg-slate-800/80 border border-slate-700/50 shrink-0">
                    {renderTypeIcon(item.type)}
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between gap-2 mb-1">
                      <span className="text-xs font-semibold text-slate-200 truncate">
                        {item.title}
                      </span>
                      <span className="text-[11px] text-slate-500 shrink-0">
                        {formatRelativeTime(item.createdAt)}
                      </span>
                    </div>

                    <p className="text-xs text-slate-400 leading-relaxed break-words mb-2.5">
                      {item.message}
                    </p>

                    {/* Action Links */}
                    <div className="flex items-center gap-2 pt-1">
                      {item.type === 'WAITLIST_PROMOTED' && onOpenWaitlist && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            if (!item.isRead) handleMarkRead(item.id);
                            onClose();
                            onOpenWaitlist();
                          }}
                          className="text-[11px] font-medium px-2.5 py-1 rounded bg-amber-500/20 hover:bg-amber-500/30 text-amber-300 border border-amber-500/30 transition-colors flex items-center gap-1 cursor-pointer"
                        >
                          <span>Claim Hold</span>
                          <span>→</span>
                        </button>
                      )}

                      {(item.type === 'TICKET_TRANSFERRED' || item.type === 'BOOKING_CONFIRMED') && onOpenBookings && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            if (!item.isRead) handleMarkRead(item.id);
                            onClose();
                            onOpenBookings();
                          }}
                          className="text-[11px] font-medium px-2.5 py-1 rounded bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-300 border border-cyan-500/30 transition-colors flex items-center gap-1 cursor-pointer"
                        >
                          <span>View in Passbook</span>
                          <span>→</span>
                        </button>
                      )}

                      {!item.isRead && (
                        <span className="inline-block w-2 h-2 rounded-full bg-accent ml-auto shrink-0" title="Unread" />
                      )}
                    </div>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
};
