import React from 'react';
import { TicketIcon, UserIcon, LogOutIcon, SparklesIcon, BellIcon } from './Icons';
import { ThemeToggle } from './ThemeToggle';
import { Theme } from '../services/theme';
import { User } from '../types';

import { SUPPORTED_CURRENCIES } from '../services/currency';

interface HeaderProps {
  onHomeClick?: () => void;
  apiConnected?: boolean;
  theme?: Theme;
  onToggleTheme?: () => void;
  currentUser?: User | null;
  onOpenAuth?: (tab?: 'login' | 'register') => void;
  onLogout?: () => void;
  onOpenBookings?: () => void;
  onOpenAdmin?: () => void;
  currentCurrency?: string;
  onCurrencyChange?: (curr: string) => void;
  onOpenNotifications?: () => void;
  unreadNotificationsCount?: number;
}

export const Header: React.FC<HeaderProps> = ({
  onHomeClick,
  apiConnected = true,
  theme = 'dark',
  onToggleTheme,
  currentUser = null,
  onOpenAuth,
  onLogout,
  onOpenBookings,
  onOpenAdmin,
  currentCurrency = 'USD',
  onCurrencyChange,
  onOpenNotifications,
  unreadNotificationsCount = 0,
}) => {
  return (
    <header className="site-header">
      <div className="header-container">
        <div className="header-brand">
          <span className="brand-mark" aria-hidden="true">
            <TicketIcon size={20} />
          </span>
          <div className="brand-text">
            <h1 className="brand-title">
              <button
                type="button"
                className="brand-home cursor-pointer"
                onClick={onHomeClick}
                aria-label="TicketVerse home"
              >
                TicketVerse
              </button>
            </h1>
            <span className="brand-subtitle">Live tickets, real seats</span>
          </div>
        </div>

        <div className="header-status-area">
          {/* Surface trouble only — a healthy gateway is not customer information. */}
          {!apiConnected && (
            <span className="header-offline" role="status">
              Reconnecting…
            </span>
          )}

          {onCurrencyChange && (
            <div className="currency-selector-pill">
              <label htmlFor="currency-select" className="sr-only">Currency</label>
              <select
                id="currency-select"
                aria-label="Currency"
                value={currentCurrency}
                onChange={(e) => onCurrencyChange(e.target.value)}
                className="currency-select-dropdown cursor-pointer"
                data-testid="currency-selector"
              >
                {Object.values(SUPPORTED_CURRENCIES).map((c) => (
                  <option key={c.code} value={c.code}>
                    {c.symbol} {c.code}
                  </option>
                ))}
              </select>
            </div>
          )}

          {onToggleTheme && (
            <ThemeToggle theme={theme} onToggle={onToggleTheme} />
          )}

          {currentUser && onOpenBookings && (
            <button
              type="button"
              className="header-action cursor-pointer"
              onClick={onOpenBookings}
              title="My tickets"
              aria-label="My tickets"
              data-testid="my-tickets-btn"
            >
              <TicketIcon size={16} />
              <span className="header-action-label">My tickets</span>
            </button>
          )}

          {currentUser && onOpenNotifications && (
            <button
              type="button"
              className="header-icon-button cursor-pointer"
              onClick={onOpenNotifications}
              title="Notifications"
              aria-label="Notifications"
              data-testid="notifications-bell-btn"
            >
              <BellIcon size={16} />
              {unreadNotificationsCount > 0 && (
                <span className="notification-badge-counter">
                  {unreadNotificationsCount > 9 ? '9+' : unreadNotificationsCount}
                </span>
              )}
            </button>
          )}

          {currentUser && onOpenAdmin && (
            <button
              type="button"
              className={`header-action btn-admin-studio cursor-pointer role-${currentUser.role || 'customer'}`}
              onClick={onOpenAdmin}
              title={
                currentUser.role === 'customer'
                  ? 'Open your VIP hub'
                  : currentUser.role === 'organizer'
                  ? 'Open organizer studio'
                  : 'Open admin command'
              }
              aria-label={
                currentUser.role === 'customer'
                  ? 'VIP Hub'
                  : currentUser.role === 'organizer'
                  ? 'Organizer Studio'
                  : 'Admin Command'
              }
              data-testid={
                currentUser.role === 'customer' ? 'role-workspace-btn' : 'admin-studio-btn'
              }
            >
              <SparklesIcon size={15} />
              <span className="header-action-label">
                {currentUser.role === 'customer'
                  ? 'VIP Hub'
                  : currentUser.role === 'organizer'
                  ? 'Organizer Studio'
                  : 'Admin Command'}
              </span>
            </button>
          )}

          {currentUser ? (
            <div className="user-profile-pill" data-testid="user-profile-badge">
              <div className="user-avatar-badge" title={currentUser.email}>
                <UserIcon size={15} />
              </div>
              <div className="user-info-text">
                <span className="user-display-name">{currentUser.name}</span>
                <span className={`user-role-badge role-${currentUser.role || 'customer'}`}>
                  {currentUser.role || 'customer'}
                </span>
              </div>
              {onLogout && (
                <button
                  type="button"
                  className="btn-logout cursor-pointer"
                  onClick={onLogout}
                  title="Sign out"
                  aria-label="Sign out"
                  data-testid="logout-btn"
                >
                  <LogOutIcon size={16} />
                </button>
              )}
            </div>
          ) : (
            onOpenAuth && (
              <button
                type="button"
                className="btn-auth-signin cursor-pointer"
                onClick={() => onOpenAuth('login')}
                data-testid="signin-btn"
              >
                <UserIcon size={16} />
                <span>Sign in</span>
              </button>
            )
          )}
        </div>
      </div>
    </header>
  );
};
