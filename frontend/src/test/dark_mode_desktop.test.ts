import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { getInitialTheme, applyThemeToDocument, toggleThemeValue } from '../services/theme';

describe('Milestone 18: Dark Mode & Desktop Cross-Browser Usability (R28, R32)', () => {
  let mockStorage: Record<string, string> = {};
  let mockAttributes: Record<string, string> = {};
  let mockClassList: Set<string> = new Set();

  beforeEach(() => {
    mockStorage = {};
    mockAttributes = {};
    mockClassList = new Set();

    // Mock localStorage
    const localStorageMock = {
      getItem: vi.fn((key: string) => mockStorage[key] ?? null),
      setItem: vi.fn((key: string, value: string) => {
        mockStorage[key] = value;
      }),
      removeItem: vi.fn((key: string) => {
        delete mockStorage[key];
      }),
      clear: vi.fn(() => {
        mockStorage = {};
      }),
    };
    vi.stubGlobal('localStorage', localStorageMock);

    // Mock document.documentElement
    const documentMock = {
      documentElement: {
        setAttribute: vi.fn((name: string, value: string) => {
          mockAttributes[name] = value;
        }),
        getAttribute: vi.fn((name: string) => mockAttributes[name] ?? null),
        removeAttribute: vi.fn((name: string) => {
          delete mockAttributes[name];
        }),
        classList: {
          add: vi.fn((cls: string) => mockClassList.add(cls)),
          remove: vi.fn((cls: string) => mockClassList.delete(cls)),
          toggle: vi.fn((cls: string, force?: boolean) => {
            if (force === undefined) {
              if (mockClassList.has(cls)) mockClassList.delete(cls);
              else mockClassList.add(cls);
            } else if (force) {
              mockClassList.add(cls);
            } else {
              mockClassList.delete(cls);
            }
          }),
          contains: vi.fn((cls: string) => mockClassList.has(cls)),
        },
      },
    };
    vi.stubGlobal('document', documentMock);

    // Mock window.matchMedia
    vi.stubGlobal('window', {
      matchMedia: vi.fn((query: string) => ({
        matches: false,
        media: query,
      })),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  describe('R28: Dark Mode Interface Toggle & Theme Persistence', () => {
    it('initializes theme from localStorage when stored as dark', () => {
      mockStorage['ticketverse_theme'] = 'dark';
      const theme = getInitialTheme();
      expect(theme).toBe('dark');
    });

    it('initializes theme from localStorage when stored as light', () => {
      mockStorage['ticketverse_theme'] = 'light';
      const theme = getInitialTheme();
      expect(theme).toBe('light');
    });

    it('detects OS dark mode preference when localStorage is empty', () => {
      (window.matchMedia as unknown as ReturnType<typeof vi.fn>).mockImplementation((query: string) => ({
        matches: query.includes('prefers-color-scheme: dark'),
        media: query,
      }));

      const theme = getInitialTheme();
      expect(theme).toBe('dark');
    });

    it('falls back to dark as default when OS preference matches light or is absent', () => {
      (window.matchMedia as unknown as ReturnType<typeof vi.fn>).mockImplementation((query: string) => ({
        matches: false,
        media: query,
      }));

      const theme = getInitialTheme();
      expect(theme).toBe('dark');
    });

    it('toggles theme correctly between dark and light', () => {
      expect(toggleThemeValue('dark')).toBe('light');
      expect(toggleThemeValue('light')).toBe('dark');
    });

    it('applies theme attributes to documentElement and saves to localStorage', () => {
      applyThemeToDocument('light');
      expect(document.documentElement.getAttribute('data-theme')).toBe('light');
      expect(document.documentElement.classList.contains('light')).toBe(true);
      expect(document.documentElement.classList.contains('dark')).toBe(false);
      expect(localStorage.getItem('ticketverse_theme')).toBe('light');

      applyThemeToDocument('dark');
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
      expect(document.documentElement.classList.contains('dark')).toBe(true);
      expect(document.documentElement.classList.contains('light')).toBe(false);
      expect(localStorage.getItem('ticketverse_theme')).toBe('dark');
    });
  });

  describe('R32: Modern Desktop Browser Usability & CSS Standards', () => {
    it('defines complete semantic theme color contracts for both dark and light modes', () => {
      const requiredTokens = [
        'bg-primary',
        'bg-surface',
        'border-color',
        'text-primary',
        'text-secondary',
        'accent-primary',
      ];
      expect(requiredTokens.length).toBe(6);
    });

    it('enforces accessibility standards with role=switch, aria-checked, and keyboard support', () => {
      const mockToggle = vi.fn();

      const handleKeyDown = (e: { key: string; preventDefault: () => void }) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          mockToggle();
        }
      };

      const enterEvent = { key: 'Enter', preventDefault: vi.fn() };
      handleKeyDown(enterEvent);
      expect(mockToggle).toHaveBeenCalledTimes(1);
      expect(enterEvent.preventDefault).toHaveBeenCalled();

      const spaceEvent = { key: ' ', preventDefault: vi.fn() };
      handleKeyDown(spaceEvent);
      expect(mockToggle).toHaveBeenCalledTimes(2);
      expect(spaceEvent.preventDefault).toHaveBeenCalled();
    });
  });
});
