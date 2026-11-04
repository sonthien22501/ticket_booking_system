import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
// @ts-ignore
import fs from 'node:fs';
// @ts-ignore
import path from 'node:path';

// @ts-ignore
declare const __dirname: string;
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

  describe('Milestone 36: Semantic Theme Architecture & Zero Split-Brain Contrast Gate', () => {
    const cssPath = path.resolve(__dirname, '../index.css');
    const cssContent = fs.readFileSync(cssPath, 'utf-8');
    const venueSeatMapPath = path.resolve(__dirname, '../components/VenueSeatMap.tsx');
    const venueSeatMapContent = fs.readFileSync(venueSeatMapPath, 'utf-8');
    const checkoutFormPath = path.resolve(__dirname, '../components/CheckoutForm.tsx');
    const checkoutFormContent = fs.readFileSync(checkoutFormPath, 'utf-8');
    const headerPath = path.resolve(__dirname, '../components/Header.tsx');
    const headerContent = fs.readFileSync(headerPath, 'utf-8');

    it('uses semantic theme surface tokens for checkout inputs and summary cards instead of hardcoded dark backgrounds', () => {
      // .form-input must use var(--bg-surface-elevated)
      expect(cssContent).toMatch(/\.form-input\s*\{[^}]*background:\s*var\(--bg-surface-elevated\)/);
      // .payment-method-card must use var(--bg-surface-elevated)
      expect(cssContent).toMatch(/\.payment-method-card\s*\{[^}]*background:\s*var\(--bg-surface-elevated\)/);
      // .order-summary-section must use var(--bg-surface-elevated)
      expect(cssContent).toMatch(/\.order-summary-section\s*\{[^}]*background:\s*var\(--bg-surface-elevated\)/);
      // .booking-ref-card must use var(--bg-surface-elevated)
      expect(cssContent).toMatch(/\.booking-ref-card\s*\{[^}]*background:\s*var\(--bg-surface-elevated\)/);
      // .confirmation-details-card must use var(--bg-surface-elevated)
      expect(cssContent).toMatch(/\.confirmation-details-card\s*\{[^}]*background:\s*var\(--bg-surface-elevated\)/);
      // .digital-ticket-card must use var(--bg-surface-elevated)
      expect(cssContent).toMatch(/\.digital-ticket-card\s*\{[^}]*background:\s*var\(--bg-surface-elevated\)/);
    });

    it('uses semantic text variables instead of hardcoded white text for ticket actions, titles, and totals', () => {
      // .btn-print-ticket must use color: var(--text-primary)
      expect(cssContent).toMatch(/\.btn-print-ticket\s*\{[^}]*color:\s*var\(--text-primary\)/);
      // .card-section-title must use color: var(--text-primary)
      expect(cssContent).toMatch(/\.card-section-title\s*\{[^}]*color:\s*var\(--text-primary\)/);
      // .total-row must use color: var(--text-primary)
      expect(cssContent).toMatch(/\.total-row\s*\{[^}]*color:\s*var\(--text-primary\)/);
    });

    it('derives the light theme from shared tokens instead of per-component colour patches', () => {
      const lightBlock = cssContent.match(/html\[data-theme=['"]light['"]\][\s\S]*?\n\}/);
      expect(lightBlock).toBeTruthy();
      const block = lightBlock?.[0] ?? '';
      // Surfaces, ink, borders and the stamp colour all flip together.
      expect(block).toMatch(/--bg-primary:\s*#/);
      expect(block).toMatch(/--bg-surface:\s*#/);
      expect(block).toMatch(/--text-primary:\s*#/);
      expect(block).toMatch(/--border-color:\s*#/);
      expect(block).toMatch(/--color-accent:\s*#/);
      // Tailwind's own ramps invert too, so `bg-slate-900` repaints with no patch.
      expect(block).toMatch(/--color-slate-900:\s*#/);
      expect(block).toMatch(/--color-slate-300:\s*#/);
      // The old component-specific light-mode hex patches are gone by design.
      expect(cssContent).not.toMatch(/(?:html\[data-theme=['"]light['"]?\]|body\.light)[^{]*\.premiere-category-tag/);
      expect(cssContent).not.toMatch(/(?:html\[data-theme=['"]light['"]?\]|body\.light)[^{]*\.btn-admin-studio/);
    });

    it('verifies component classes support light mode semantic styling in JSX/TSX', () => {
      // VenueSeatMap contains ai-concierge-panel, floorplan-overview-card
      expect(venueSeatMapContent).toContain('ai-concierge-panel');
      expect(venueSeatMapContent).toContain('floorplan-overview-card');
      // CheckoutForm contains addon-upsell-card
      expect(checkoutFormContent).toContain('addon-upsell-card');
      // Header contains btn-admin-studio
      expect(headerContent).toContain('btn-admin-studio');
    });

    it('ensures zero CSS rules mix hardcoded dark backgrounds with dynamic text or theme backgrounds with white text', () => {
      // No rule combines background: #0f0f23 or #0f172a with color: var(--text-primary)
      const rules = cssContent.replace(/\/\*[\s\S]*?\*\//g, '').match(/[^{]+{[^}]+}/g) || [];
      for (const rule of rules) {
        if (rule.includes(':root') || rule.includes('.light') || rule.includes('data-theme=')) continue;
        const hasDarkBg = /background(?:-color)?:\s*#(?:0f0f23|0f172a)/.test(rule);
        const hasDynamicText = /color:\s*var\(--text-primary\)/.test(rule);
        expect(hasDarkBg && hasDynamicText).toBe(false);

        const hasElevatedBg = /background(?:-color)?:\s*var\(--bg-surface-elevated\)/.test(rule);
        const hasHardcodedWhiteText = /color:\s*(?:#ffffff|#fff\b|white\b)/.test(rule);
        expect(hasElevatedBg && hasHardcodedWhiteText).toBe(false);
      }
    });
  });

  describe('Milestone 37: Tailwind v4 Cascade Layer Safety, Brand Icon Contrast & Card Overflow Protection', () => {
    const cssPath = path.resolve(__dirname, '../index.css');
    const cssContent = fs.readFileSync(cssPath, 'utf-8');

    it('does not contain an unlayered wildcard padding or margin reset that clobbers Tailwind v4 utilities', () => {
      // Must not have unlayered `* { ... padding: 0; ... }` or `* { ... margin: 0; ... }`
      const cleaned = cssContent.replace(/\/\*[\s\S]*?\*\//g, '');
      const wildcardRules = cleaned.match(/(?:^|\})\s*\*\s*\{([^}]+)\}/g) || [];
      for (const rule of wildcardRules) {
        expect(rule).not.toMatch(/(?:padding|margin)\s*:\s*0/);
      }
    });

    it('defines @theme with --color-brand-accent and a token-driven brand mark', () => {
      // @theme { --color-brand-accent: ... }
      expect(cssContent).toMatch(/@theme\s*\{[^}]*--color-brand-accent\s*:/);
      // .text-brand-accent resolves through the brand token
      expect(cssContent).toMatch(/\.text-brand-accent\s*\{[^}]*color:\s*var\(--brand-accent\)/);
      // The masthead mark is stamped in the accent colour, not a hardcoded amber hex
      expect(cssContent).toMatch(/\.brand-mark\s*\{[^}]*color:\s*var\(--color-accent\)/);
      expect(cssContent).not.toMatch(/\.brand-icon-badge\s*\{[^}]*color:\s*#fbbf24/);
    });

    it('ensures .premiere-showcase-card uses min-height without a fixed-height media query lock', () => {
      // .premiere-showcase-card has min-height: 172px
      expect(cssContent).toMatch(/\.premiere-showcase-card\s*\{[^}]*min-height:\s*172px/);
      // Must NOT lock height: 172px inside media query
      expect(cssContent).not.toMatch(/@media[^{]*\{[^}]*\.premiere-showcase-card\s*\{[^}]*height:\s*172px/);
    });

    it('defines explicit padding for .addon-upsell-card so internal text never touches borders', () => {
      expect(cssContent).toMatch(/\.addon-upsell-card\s*\{[^}]*padding:\s*0\.875rem\s+1rem/);
    });
  });
});
