import React, { useEffect } from 'react';
import { XIcon, AlertCircleIcon, CheckCircleIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';

export interface ConfirmationDialogProps {
  isOpen: boolean;
  title: string;
  message?: string;
  children?: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  confirmVariant?: 'primary' | 'danger' | 'warning';
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export const ConfirmationDialog: React.FC<ConfirmationDialogProps> = ({
  isOpen,
  title,
  message,
  children,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  confirmVariant = 'primary',
  loading = false,
  onConfirm,
  onCancel,
}) => {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !loading) {
        onCancel();
      }
    };
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
    }
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, loading, onCancel]);

  if (!isOpen) return null;

  const btnVariantClass =
    confirmVariant === 'danger'
      ? 'btn-dialog-danger'
      : confirmVariant === 'warning'
      ? 'btn-dialog-warning'
      : 'btn-dialog-primary';

  return (
    <div className="modal-backdrop confirmation-backdrop" onClick={() => !loading && onCancel()}>
      <div
        className="modal-container confirmation-dialog-container"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirmation-dialog-title"
      >
        <div className="modal-header">
          <div className="flex items-center gap-2">
            {confirmVariant === 'danger' || confirmVariant === 'warning' ? (
              <AlertCircleIcon size={20} className={confirmVariant === 'danger' ? 'text-danger' : 'text-warning'} />
            ) : (
              <CheckCircleIcon size={20} className="text-brand-accent" />
            )}
            <h2 id="confirmation-dialog-title" className="modal-title">
              {title}
            </h2>
          </div>
          {!loading && (
            <button
              type="button"
              onClick={onCancel}
              className="modal-btn-close cursor-pointer"
              aria-label="Close dialog"
            >
              <XIcon size={18} />
            </button>
          )}
        </div>

        <div className="modal-body confirmation-dialog-body">
          {message && <p className="confirmation-dialog-message">{message}</p>}
          {children}
        </div>

        <div className="modal-footer confirmation-dialog-footer">
          <button
            type="button"
            onClick={onCancel}
            disabled={loading}
            className="btn-dialog-cancel cursor-pointer"
          >
            {cancelLabel}
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={loading}
            className={`btn-dialog-confirm cursor-pointer ${btnVariantClass}`}
          >
            {loading ? <LoadingSpinner size="sm" message="Processing..." /> : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
};
