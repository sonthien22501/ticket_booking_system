import React from 'react';
import { TicketIcon, UserIcon, LogOutIcon, SparklesIcon } from './Icons';
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
}) => {
  return (
    <header className="site-header">
      <div className="header-container">
        <div
          className="header-brand cursor-pointer"
          onClick={onHomeClick}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if ((e.key === 'Enter' || e.key === ' ') && onHomeClick) {
              e.preventDefault();
              onHomeClick();
            }
          }}
          aria-label="TicketVerse Home"
        >
          <div className="brand-icon-badge">
            <TicketIcon size={24} className="text-brand-accent" />
          </div>
          <div className="brand-text">
            <h1 className="brand-title">TicketVerse</h1>
            <span className="brand-subtitle">Cinematic Live Ticket Booking</span>
          </div>
        </div>

        <div className="header-status-area">
          {onToggleTheme && (
            <ThemeToggle theme={theme} onToggle={onToggleTheme} />
          )}

          {/* Multi-Currency Selector (Milestone 26) */}
          {onCurrencyChange && (
            <div className="currency-selector-pill flex items-center bg-slate-900/80 border border-slate-700/60 rounded-xl px-2 py-1 shadow-sm hover:border-cyan-500/70 transition-all">
              <label htmlFor="currency-select" className="sr-only">Select Currency</label>
              <select
                id="currency-select"
                aria-label="Currency"
                value={currentCurrency}
                onChange={(e) => onCurrencyChange(e.target.value)}
                className="bg-transparent text-cyan-300 font-bold text-xs outline-none cursor-pointer pr-1"
                data-testid="currency-selector"
              >
                {Object.values(SUPPORTED_CURRENCIES).map((c) => (
                  <option key={c.code} value={c.code} className="bg-slate-900 text-slate-200">
                    {c.symbol} {c.code}
                  </option>
                ))}
              </select>
            </div>
          )}

          <div
            className={`gateway-pill ${apiConnected ? 'pill-online' : 'pill-offline'}`}
            title={apiConnected ? 'Gateway Port 8080 Active' : 'Gateway Offline / Reconnecting'}
          >
            <span className="status-dot"></span>
            <span className="status-label">
              {apiConnected ? 'Gateway Connected' : 'Connecting to Gateway...'}
            </span>
          </div>

          {/* My Bookings / Tickets Button */}
          {onOpenBookings && (
            <button
              type="button"
              className="btn-my-tickets cursor-pointer flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-indigo-950/70 border border-indigo-700/50 hover:border-amber-500/60 hover:bg-indigo-900/80 text-amber-300 text-xs font-bold transition-all shadow-sm"
              onClick={onOpenBookings}
              title="View My Bookings & Tickets"
              aria-label="View My Bookings & Tickets"
              data-testid="my-tickets-btn"
            >
              <TicketIcon size={16} />
              <span>My Tickets</span>
            </button>
          )}

          {/* Admin Studio / Organizer Portal Button (Milestone 24) */}
          {onOpenAdmin && (
            <button
              type="button"
              className="btn-admin-studio cursor-pointer flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-purple-950/70 border border-purple-700/50 hover:border-purple-400 hover:bg-purple-900/80 text-purple-300 text-xs font-bold transition-all shadow-sm"
              onClick={onOpenAdmin}
              title="Open Organizer Studio & Live Telemetry"
              aria-label="Organizer Studio"
              data-testid="admin-studio-btn"
            >
              <SparklesIcon size={15} />
              <span>Organizer Studio</span>
            </button>
          )}

          {/* User Authentication Pill / Controls */}
          {currentUser ? (
            <div className="user-profile-pill" data-testid="user-profile-badge">
              <div className="user-avatar-badge" title={currentUser.email}>
                <UserIcon size={16} />
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
                  title="Sign Out"
                  aria-label="Sign Out"
                  data-testid="logout-btn"
                >
                  <LogOutIcon size={16} />
                  <span className="logout-text">Sign Out</span>
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
                <span>Sign In</span>
              </button>
            )
          )}
        </div>
      </div>
    </header>
  );
};


