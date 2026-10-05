import React from 'react';
import { AlertCircleIcon, CheckCircleIcon, XIcon, RefreshCwIcon } from './Icons';

export type AlertType = 'info' | 'success' | 'warning' | 'error' | 'conflict';

interface AlertNotificationProps {
  type: AlertType;
  title?: string;
  message: string;
  onClose?: () => void;
  onRetry?: () => void;
  actionLabel?: string;
}

export const AlertNotification: React.FC<AlertNotificationProps> = ({
  type,
  title,
  message,
  onClose,
  onRetry,
  actionLabel = 'Refresh',
}) => {
  const getIcon = () => {
    switch (type) {
      case 'success':
        return <CheckCircleIcon size={20} className="alert-icon-success" />;
      case 'conflict':
        return <AlertCircleIcon size={20} className="alert-icon-conflict" />;
      case 'warning':
        return <AlertCircleIcon size={20} className="alert-icon-warning" />;
      case 'error':
        return <AlertCircleIcon size={20} className="alert-icon-error" />;
      default:
        return <AlertCircleIcon size={20} className="alert-icon-info" />;
    }
  };

  const defaultTitles = {
    info: 'Information',
    success: 'Success',
    warning: 'Attention',
    error: 'System Error',
    conflict: 'Seat Conflict Detected (409)',
  };

  return (
    <div className={`alert-box alert-${type}`} role="alert">
      <div className="alert-content-wrapper">
        <div className="alert-icon-wrapper">{getIcon()}</div>
        <div className="alert-text-wrapper">
          <h4 className="alert-title">{title || defaultTitles[type]}</h4>
          <p className="alert-message">{message}</p>
        </div>
      </div>

      <div className="alert-actions">
        {onRetry && (
          <button type="button" onClick={onRetry} className="alert-btn-retry">
            <RefreshCwIcon size={14} className="mr-1" />
            {actionLabel}
          </button>
        )}
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            className="alert-btn-close"
            aria-label="Dismiss notification"
          >
            <XIcon size={16} />
          </button>
        )}
      </div>
    </div>
  );
};
