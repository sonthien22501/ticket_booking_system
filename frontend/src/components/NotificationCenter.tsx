import React, { useState, useEffect } from 'react';
import { AppNotification, NotificationDeliveryLog } from '../types';
import {
  markNotificationRead,
  markAllNotificationsRead,
  fetchNotificationDeliveries,
  dispatchTestNotification,
} from '../services/api';
import {
  BellIcon,
  CheckCircleIcon,
  AlertCircleIcon,
  TicketIcon,
  ClockIcon,
  XIcon,
  CheckIcon,
  MailIcon,
  SmartphoneIcon,
  RefreshCwIcon,
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
  const [activeTab, setActiveTab] = useState<'in-app' | 'worker'>('in-app');
  const [markingAll, setMarkingAll] = useState(false);
  const [deliveries, setDeliveries] = useState<NotificationDeliveryLog[]>([]);
  const [loadingDeliveries, setLoadingDeliveries] = useState(false);
  const [channelFilter, setChannelFilter] = useState<'ALL' | 'EMAIL' | 'SMS'>('ALL');
  const [dispatching, setDispatching] = useState(false);
  const [selectedDelivery, setSelectedDelivery] = useState<NotificationDeliveryLog | null>(null);

  useEffect(() => {
    if (isOpen && activeTab === 'worker') {
      loadDeliveries();
    }
  }, [isOpen, activeTab]);

  const loadDeliveries = async () => {
    setLoadingDeliveries(true);
    try {
      const data = await fetchNotificationDeliveries();
      setDeliveries(data);
    } catch (err) {
      console.error('Failed to load deliveries:', err);
    } finally {
      setLoadingDeliveries(false);
    }
  };

  const handleDispatchTest = async (channel: 'EMAIL' | 'SMS') => {
    setDispatching(true);
    try {
      const recipient = channel === 'SMS' ? '+15551234567' : (currentUser?.email || 'user@tickets.com');
      await dispatchTestNotification({
        eventType: 'booking.confirmed',
        recipient,
        customerName: currentUser?.name || 'Alex Morgan',
        eventTitle: 'Neon Symphony 2026',
        bookingRef: `REF-TST-${Math.floor(1000 + Math.random() * 9000)}`,
        seats: ['VIP-1', 'VIP-2'],
        totalPrice: 240.0,
        currency: 'USD',
        ticketPassUrl: 'https://tickets.example.com/passes/test',
        channel,
      });
      await loadDeliveries();
    } catch (err) {
      console.error('Failed to dispatch test notification:', err);
    } finally {
      setDispatching(false);
    }
  };

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

  const filteredDeliveries = deliveries.filter((d) => {
    if (channelFilter === 'ALL') return true;
    return d.channel.toUpperCase() === channelFilter;
  });

  return (
    <div className="fixed inset-0 z-50 overflow-hidden bg-black/60 backdrop-blur-sm flex justify-end animate-fade-in" onClick={onClose}>
      <div
        className="w-full max-w-lg bg-slate-900/95 border-l border-slate-700/60 shadow-2xl h-full flex flex-col text-slate-100"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/40">
          <div className="flex items-center gap-2.5">
            <div className="relative p-2 rounded-xl bg-slate-800/80 text-accent">
              <BellIcon size={20} />
              {unreadCount > 0 && activeTab === 'in-app' && (
                <span className="absolute -top-1 -right-1 w-5 h-5 rounded-full bg-rose-500 text-white text-[11px] font-bold flex items-center justify-center shadow-lg shadow-rose-500/50">
                  {unreadCount > 9 ? '9+' : unreadCount}
                </span>
              )}
            </div>
            <div>
              <h3 className="font-semibold text-base text-slate-100">Notification Hub</h3>
              <p className="text-xs text-slate-400">
                {activeTab === 'in-app'
                  ? unreadCount === 0 ? 'All caught up' : `${unreadCount} unread alert${unreadCount === 1 ? '' : 's'}`
                  : `${deliveries.length} external delivery logs`}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {activeTab === 'in-app' && unreadCount > 0 && (
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
            {activeTab === 'worker' && (
              <button
                onClick={loadDeliveries}
                disabled={loadingDeliveries}
                className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
                title="Refresh audit logs"
              >
                <RefreshCwIcon size={16} className={loadingDeliveries ? 'animate-spin' : ''} />
              </button>
            )}
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
              aria-label="Close notification center"
            >
              <XIcon size={18} />
            </button>
          </div>
        </div>

        {/* Tab Switcher */}
        <div className="flex items-center border-b border-slate-800/80 bg-slate-950/20 px-4 pt-2">
          <button
            onClick={() => setActiveTab('in-app')}
            className={`pb-2.5 px-3 text-xs font-medium border-b-2 transition-all flex items-center gap-1.5 cursor-pointer ${
              activeTab === 'in-app'
                ? 'border-accent text-accent'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <BellIcon size={14} />
            <span>In-App Alerts</span>
            {unreadCount > 0 && (
              <span className="px-1.5 py-0.2 rounded-full bg-rose-500/20 text-rose-300 border border-rose-500/30 text-[10px]">
                {unreadCount}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('worker')}
            className={`pb-2.5 px-3 text-xs font-medium border-b-2 transition-all flex items-center gap-1.5 cursor-pointer ${
              activeTab === 'worker'
                ? 'border-accent text-accent'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <MailIcon size={14} />
            <span>Email / SMS Worker</span>
            <span className="px-1.5 py-0.2 rounded-full bg-slate-800 text-slate-300 text-[10px] border border-slate-700/50">
              M32
            </span>
          </button>
        </div>

        {/* Tab 1: In-App Alerts */}
        {activeTab === 'in-app' && (
          <div className="flex-1 overflow-y-auto p-4 space-y-3">
            {notifications.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-center p-8 text-slate-500">
                <div className="w-14 h-14 rounded-2xl bg-slate-800/40 border border-slate-700/40 flex items-center justify-center mb-3 text-slate-600">
                  <BellIcon size={28} />
                </div>
                <p className="text-sm font-medium text-slate-400">No In-App Alerts</p>
                <p className="text-xs text-slate-500 mt-1 max-w-[220px]">
                  Real-time updates regarding your bookings, tickets, and waitlist holds appear here.
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
        )}

        {/* Tab 2: Async Notification Worker Deliveries (Milestone 32) */}
        {activeTab === 'worker' && (
          <div className="flex-1 flex flex-col overflow-hidden">
            {/* Action Bar & Channel Filters */}
            <div className="p-3 border-b border-slate-800 bg-slate-950/30 flex items-center justify-between gap-2 flex-wrap">
              <div className="flex items-center gap-1">
                {(['ALL', 'EMAIL', 'SMS'] as const).map((ch) => (
                  <button
                    key={ch}
                    onClick={() => setChannelFilter(ch)}
                    className={`px-2.5 py-1 rounded-md text-[11px] font-medium transition-colors cursor-pointer ${
                      channelFilter === ch
                        ? 'bg-accent/20 text-accent border border-accent/40'
                        : 'bg-slate-800/60 text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {ch}
                  </button>
                ))}
              </div>

              <div className="flex items-center gap-1.5">
                <button
                  onClick={() => handleDispatchTest('EMAIL')}
                  disabled={dispatching}
                  className="px-2 py-1 rounded-md bg-blue-600/20 hover:bg-blue-600/30 text-blue-300 border border-blue-500/30 text-[11px] font-medium flex items-center gap-1 cursor-pointer disabled:opacity-50"
                  title="Dispatch simulated transactional email"
                >
                  <MailIcon size={12} />
                  <span>Test Email</span>
                </button>
                <button
                  onClick={() => handleDispatchTest('SMS')}
                  disabled={dispatching}
                  className="px-2 py-1 rounded-md bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/30 text-[11px] font-medium flex items-center gap-1 cursor-pointer disabled:opacity-50"
                  title="Dispatch simulated SMS"
                >
                  <SmartphoneIcon size={12} />
                  <span>Test SMS</span>
                </button>
              </div>
            </div>

            {/* Delivery Records List */}
            <div className="flex-1 overflow-y-auto p-4 space-y-3">
              {filteredDeliveries.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-center p-8 text-slate-500">
                  <div className="w-12 h-12 rounded-xl bg-slate-800/40 border border-slate-700/40 flex items-center justify-center mb-3 text-slate-600">
                    <MailIcon size={24} />
                  </div>
                  <p className="text-sm font-medium text-slate-400">No Delivery Records</p>
                  <p className="text-xs text-slate-500 mt-1 max-w-[220px]">
                    Click "Test Email" or "Test SMS" above to simulate an asynchronous RabbitMQ worker dispatch.
                  </p>
                </div>
              ) : (
                filteredDeliveries.map((del) => (
                  <div
                    key={del.id}
                    onClick={() => setSelectedDelivery(del)}
                    className="p-3.5 rounded-xl border border-slate-800 bg-slate-900/60 hover:bg-slate-850 hover:border-slate-700 transition-all cursor-pointer group"
                  >
                    <div className="flex items-start justify-between gap-2 mb-2">
                      <div className="flex items-center gap-2">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] font-bold tracking-wider flex items-center gap-1 ${
                            del.channel === 'EMAIL'
                              ? 'bg-blue-500/20 text-blue-300 border border-blue-500/30'
                              : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                          }`}
                        >
                          {del.channel === 'EMAIL' ? <MailIcon size={11} /> : <SmartphoneIcon size={11} />}
                          <span>{del.channel}</span>
                        </span>

                        <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                          {del.status}
                        </span>
                      </div>

                      <span className="text-[11px] text-slate-500">
                        {formatRelativeTime(del.createdAt)}
                      </span>
                    </div>

                    <div className="text-xs font-semibold text-slate-200 mb-1 group-hover:text-accent transition-colors truncate">
                      {del.subject || del.eventType}
                    </div>

                    <div className="text-[11px] text-slate-400 truncate mb-1">
                      To: <span className="text-slate-300 font-mono">{del.recipient}</span>
                    </div>

                    <div className="text-xs text-slate-400 line-clamp-2 font-serif italic bg-slate-950/40 p-2 rounded border border-slate-800/60">
                      {del.body.replace(/<[^>]*>?/gm, '').slice(0, 120)}...
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* Audit Modal for Selected Delivery */}
            {selectedDelivery && (
              <div className="p-4 border-t border-slate-800 bg-slate-950/80 max-h-56 overflow-y-auto">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-bold text-accent uppercase tracking-wide">
                    Delivery Inspection: {selectedDelivery.id}
                  </span>
                  <button
                    onClick={() => setSelectedDelivery(null)}
                    className="text-slate-400 hover:text-white text-xs cursor-pointer"
                  >
                    Close
                  </button>
                </div>
                <div className="text-xs text-slate-300 space-y-1">
                  <div><strong>Event:</strong> {selectedDelivery.eventType}</div>
                  <div><strong>Channel:</strong> {selectedDelivery.channel}</div>
                  <div><strong>Recipient:</strong> {selectedDelivery.recipient}</div>
                  <div><strong>Attempts:</strong> {selectedDelivery.attempts}</div>
                  <div className="pt-1 font-mono text-[11px] bg-slate-900 p-2 rounded border border-slate-800 break-words max-h-24 overflow-y-auto">
                    {selectedDelivery.body}
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
