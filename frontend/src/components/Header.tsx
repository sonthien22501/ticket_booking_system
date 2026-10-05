import React from 'react';
import { TicketIcon } from './Icons';

interface HeaderProps {
  onHomeClick?: () => void;
  apiConnected?: boolean;
}

export const Header: React.FC<HeaderProps> = ({ onHomeClick, apiConnected = true }) => {
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
          <div className={`gateway-pill ${apiConnected ? 'pill-online' : 'pill-offline'}`}>
            <span className="status-dot"></span>
            <span className="status-label">
              {apiConnected ? 'API Gateway Connected' : 'Connecting to Gateway...'}
            </span>
          </div>
        </div>
      </div>
    </header>
  );
};
