export type Theme = 'dark' | 'light';

export const THEME_STORAGE_KEY = 'ticketverse_theme';

/**
 * Determines initial theme preference based on localStorage or OS media query.
 */
export const getInitialTheme = (): Theme => {
  if (typeof window === 'undefined') return 'dark';

  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    if (saved === 'dark' || saved === 'light') {
      return saved;
    }
  } catch (err) {
    // localStorage might be blocked or unavailable in incognito/strict iframe
    console.warn('Unable to access localStorage for theme preference:', err);
  }

  // Check system dark mode preference
  try {
    if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
      return 'dark';
    }
  } catch (err) {
    // Ignore media query error
  }

  return 'dark'; // TicketVerse default
};

/**
 * Toggles theme value
 */
export const toggleThemeValue = (current: Theme): Theme => {
  return current === 'dark' ? 'light' : 'dark';
};

/**
 * Applies the given theme to documentElement attributes and persists to localStorage
 */
export const applyThemeToDocument = (theme: Theme): void => {
  if (typeof document === 'undefined') return;

  const root = document.documentElement;
  root.setAttribute('data-theme', theme);
  if (theme === 'dark') {
    root.classList.add('dark');
    root.classList.remove('light');
  } else {
    root.classList.add('light');
    root.classList.remove('dark');
  }

  try {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(THEME_STORAGE_KEY, theme);
    }
  } catch (err) {
    console.warn('Failed to save theme to localStorage:', err);
  }
};
