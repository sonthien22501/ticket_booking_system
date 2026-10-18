import React, { useState, useEffect } from 'react';
import { User } from '../types';
import { loginUser, registerUser } from '../services/auth';
import { UserIcon, LockIcon, LogInIcon, XIcon } from './Icons';
import { LoadingSpinner } from './LoadingSpinner';

interface AuthModalProps {
  isOpen: boolean;
  onClose: () => void;
  onAuthSuccess: (user: User, token: string) => void;
  initialTab?: 'login' | 'register';
}

export const AuthModal: React.FC<AuthModalProps> = ({
  isOpen,
  onClose,
  onAuthSuccess,
  initialTab = 'login',
}) => {
  const [activeTab, setActiveTab] = useState<'login' | 'register'>(initialTab);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [role, setRole] = useState<'customer' | 'admin'>('customer');
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // Sync initial tab when modal opens
  useEffect(() => {
    if (isOpen) {
      setActiveTab(initialTab);
      setErrorMsg(null);
    }
  }, [isOpen, initialTab]);

  // Handle escape key listener
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
    }
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);

    const cleanEmail = email.trim().toLowerCase();
    const cleanPassword = password.trim();
    const cleanName = name.trim();

    if (!cleanEmail || !cleanEmail.includes('@')) {
      setErrorMsg('Please enter a valid email address.');
      return;
    }

    if (cleanPassword.length < 6) {
      setErrorMsg('Password must be at least 6 characters long.');
      return;
    }

    if (activeTab === 'register' && !cleanName) {
      setErrorMsg('Full name is required to create an account.');
      return;
    }

    setLoading(true);

    try {
      if (activeTab === 'login') {
        const res = await loginUser({ email: cleanEmail, password: cleanPassword });
        onAuthSuccess(res.user, res.token);
        onClose();
      } else {
        const res = await registerUser({
          name: cleanName,
          email: cleanEmail,
          password: cleanPassword,
          role,
        });
        onAuthSuccess(res.user, res.token);
        onClose();
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Authentication failed. Please try again.';
      setErrorMsg(msg);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="modal-overlay auth-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="auth-modal-title"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          onClose();
        }
      }}
    >
      <div className="modal-container auth-modal-container">
        <div className="modal-header auth-modal-header">
          <div className="auth-brand-badge">
            <UserIcon size={24} className="text-brand-accent" />
          </div>
          <h2 id="auth-modal-title" className="modal-title">
            {activeTab === 'login' ? 'Welcome Back' : 'Create an Account'}
          </h2>
          <p className="modal-subtitle">
            {activeTab === 'login'
              ? 'Sign in to access your bookings and reserve tickets instantly'
              : 'Join TicketVerse to book authentic tickets with guaranteed zero-overselling'}
          </p>
          <button
            type="button"
            className="modal-btn-close auth-modal-close-btn cursor-pointer"
            onClick={onClose}
            aria-label="Close authentication modal"
          >
            <XIcon size={20} />
          </button>
        </div>

        {/* Tab Switcher */}
        <div className="auth-tabs" role="tablist">
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'login'}
            className={`auth-tab cursor-pointer ${activeTab === 'login' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('login');
              setErrorMsg(null);
            }}
          >
            Sign In
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'register'}
            className={`auth-tab cursor-pointer ${activeTab === 'register' ? 'active' : ''}`}
            onClick={() => {
              setActiveTab('register');
              setErrorMsg(null);
            }}
          >
            Create Account
          </button>
        </div>

        {errorMsg && (
          <div className="auth-error-banner" role="alert">
            <span className="auth-error-dot"></span>
            <span>{errorMsg}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="auth-form">
          {activeTab === 'register' && (
            <div className="form-group">
              <label htmlFor="auth-name" className="form-label">
                Full Name
              </label>
              <div className="input-with-icon">
                <UserIcon size={18} className="input-icon" />
                <input
                  id="auth-name"
                  type="text"
                  className="form-input with-icon"
                  placeholder="e.g. Max Mustermann"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  autoComplete="name"
                  disabled={loading}
                />
              </div>
            </div>
          )}

          <div className="form-group">
            <label htmlFor="auth-email" className="form-label">
              Email Address
            </label>
            <div className="input-with-icon">
              <UserIcon size={18} className="input-icon" />
              <input
                id="auth-email"
                type="email"
                className="form-input with-icon"
                placeholder="name@example.de"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                autoComplete="email"
                disabled={loading}
                required
              />
            </div>
          </div>

          <div className="form-group">
            <label htmlFor="auth-password" className="form-label">
              Password
            </label>
            <div className="input-with-icon">
              <LockIcon size={18} className="input-icon" />
              <input
                id="auth-password"
                type="password"
                className="form-input with-icon"
                placeholder="Minimum 6 characters"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete={activeTab === 'login' ? 'current-password' : 'new-password'}
                disabled={loading}
                required
              />
            </div>
          </div>

          {activeTab === 'register' && (
            <div className="form-group">
              <label htmlFor="auth-role" className="form-label">
                Account Role
              </label>
              <select
                id="auth-role"
                className="form-input cursor-pointer"
                value={role}
                onChange={(e) => setRole(e.target.value as 'customer' | 'admin')}
                disabled={loading}
              >
                <option value="customer">Customer (Standard Attendee)</option>
                <option value="admin">Department Admin (Organizer)</option>
              </select>
            </div>
          )}

          <div className="auth-actions">
            <button
              type="submit"
              className="btn btn-primary auth-submit-btn cursor-pointer"
              disabled={loading}
            >
              {loading ? (
                <span className="btn-loading-flex">
                  <LoadingSpinner size="sm" message="" />
                  <span>{activeTab === 'login' ? 'Signing In...' : 'Registering...'}</span>
                </span>
              ) : (
                <span className="btn-icon-flex">
                  <LogInIcon size={18} />
                  <span>{activeTab === 'login' ? 'Sign In' : 'Create Account'}</span>
                </span>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
