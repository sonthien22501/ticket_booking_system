import React from 'react';
import { Theme } from '../services/theme';
import { SunIcon, MoonIcon } from './Icons';

interface ThemeToggleProps {
  theme: Theme;
  onToggle: () => void;
  className?: string;
}

export const ThemeToggle: React.FC<ThemeToggleProps> = ({
  theme,
  onToggle,
  className = '',
}) => {
  const isDark = theme === 'dark';

  const handleKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onToggle();
    }
  };

  return (
    <button
      type="button"
      role="switch"
      aria-checked={isDark}
      aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
      title={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
      onClick={onToggle}
      onKeyDown={handleKeyDown}
      className={`theme-toggle-btn ${isDark ? 'is-dark' : 'is-light'} ${className}`}
      tabIndex={0}
    >
      <span className="theme-toggle-track">
        <span className="theme-toggle-thumb">
          {isDark ? (
            <MoonIcon size={14} className="theme-toggle-icon moon" />
          ) : (
            <SunIcon size={14} className="theme-toggle-icon sun" />
          )}
        </span>
      </span>
      <span className="theme-toggle-label">
        {isDark ? 'Dark' : 'Light'}
      </span>
    </button>
  );
};
