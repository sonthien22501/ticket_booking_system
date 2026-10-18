import React from 'react';
import { TicketIcon, UserIcon, LogOutIcon } from './Icons';
import { ThemeToggle } from './ThemeToggle';
import { Theme } from '../services/theme';
import { User } from '../types';

interface HeaderProps {
  onHomeClick?: () => void;
  apiConnected?: boolean;
  theme?: Theme;
  onToggleTheme?: () => void;
  currentUser?: User | null;
  onOpenAuth?: (tab?: 'login' | 'register') => void;
  onLogout?: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  onHomeClick,
  apiConnected = true,
  theme = 'dark',
  onToggleTheme,
  currentUser = null,
  onOpenAuth,
  onLogout,
}) => {
  return (
    <header className="site-header">
      <div className="header-container">
        <div className="header-brand" onClick={onHomeClick} role="button" tabIndex={0}>
          <div className="brand-icon-badge">
            <TicketIcon size={24} className="text-brand-accent" />
          </div>
          <div className="brand-text">
            <h1 className="brand-title">TicketVerse</h1>
            <span className="brand-subtitle">Microservices Ticket Booking</span>
          </div>
        </div>

        <div className="header-status-area">
          {onToggleTheme && (
            <ThemeToggle theme={theme} onToggle={onToggleTheme} />
          )}

          <div className={`gateway-pill ${apiConnected ? 'pill-online' : 'pill-offline'}`}>
            <span className="status-dot"></span>
            <span className="status-label">
              {apiConnected ? 'API Gateway Connected' : 'Connecting to Gateway...'}
            </span>
          </div>

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
                  className="btn-logout"
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
                className="btn-auth-signin"
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


