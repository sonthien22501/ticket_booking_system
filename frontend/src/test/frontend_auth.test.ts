import React from 'react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  getStoredToken,
  getStoredUser,
  setStoredSession,
  clearStoredSession,
  loginUser,
  registerUser,
  verifyStoredSession,
  AUTH_TOKEN_KEY,
  AUTH_USER_KEY,
} from '../services/auth';
import { User, EventItem } from '../types';
import { Header } from '../components/Header';
import { CheckoutForm } from '../components/CheckoutForm';


describe('Frontend Authentication & User Session Service (R3.1)', () => {
  let mockStorage: Record<string, string> = {};

  beforeEach(() => {
    mockStorage = {};
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
    vi.restoreAllMocks();
  });

  describe('Local Storage Session Persistence', () => {
    it('returns null when no token or user exists in storage', () => {
      expect(getStoredToken()).toBeNull();
      expect(getStoredUser()).toBeNull();
    });

    it('persists and retrieves user token and profile safely', () => {
      const mockUser: User = {
        id: 'usr-123',
        email: 'anna.mueller@example.de',
        name: 'Anna Müller',
        role: 'customer',
      };
      const mockToken = 'mock.jwt.token.123';

      setStoredSession(mockToken, mockUser);

      expect(getStoredToken()).toBe(mockToken);
      expect(getStoredUser()).toEqual(mockUser);
      expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe(mockToken);
      expect(JSON.parse(localStorage.getItem(AUTH_USER_KEY) || '{}')).toEqual(mockUser);
    });

    it('clears stored token and user session completely upon logout', () => {
      setStoredSession('temp-token', {
        id: 'usr-999',
        email: 'test@example.com',
        name: 'Test',
        role: 'customer',
      });
      expect(getStoredToken()).toBe('temp-token');

      clearStoredSession();

      expect(getStoredToken()).toBeNull();
      expect(getStoredUser()).toBeNull();
      expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
      expect(localStorage.getItem(AUTH_USER_KEY)).toBeNull();
    });

    it('gracefully handles corrupted JSON in user localStorage without crashing', () => {
      localStorage.setItem(AUTH_USER_KEY, '{invalid-json');
      expect(getStoredUser()).toBeNull();
    });
  });

  describe('API Authentication Calls', () => {
    it('executes loginUser successfully and saves session to storage', async () => {
      const mockUser: User = {
        id: 'usr-456',
        email: 'max@mustermann.de',
        name: 'Max Mustermann',
        role: 'customer',
      };
      const mockResponse = {
        token: 'signed-jwt-token-456',
        user: mockUser,
      };

      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => mockResponse,
      } as Response);

      const result = await loginUser({ email: 'max@mustermann.de', password: 'password123' });

      expect(result.token).toBe('signed-jwt-token-456');
      expect(result.user.name).toBe('Max Mustermann');
      expect(getStoredToken()).toBe('signed-jwt-token-456');
      expect(getStoredUser()).toEqual(mockUser);
      expect(globalThis.fetch).toHaveBeenCalledWith(
        expect.stringMatching(/\/api\/auth\/login$/),
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
          body: JSON.stringify({ email: 'max@mustermann.de', password: 'password123' }),
        })
      );
    });

    it('throws meaningful error on invalid login credentials', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({ error: 'INVALID_CREDENTIALS', message: 'Invalid email or password' }),
      } as Response);

      await expect(
        loginUser({ email: 'wrong@example.de', password: 'badpassword' })
      ).rejects.toThrow('Invalid email or password');
      expect(getStoredToken()).toBeNull();
    });

    it('executes registerUser successfully and saves session to storage', async () => {
      const mockUser: User = {
        id: 'usr-789',
        email: 'sophie@berlin.de',
        name: 'Sophie Schneider',
        role: 'customer',
      };
      const mockResponse = {
        token: 'signed-jwt-token-789',
        user: mockUser,
      };

      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 201,
        json: async () => mockResponse,
      } as Response);

      const result = await registerUser({
        name: 'Sophie Schneider',
        email: 'sophie@berlin.de',
        password: 'securepassword123',
        role: 'customer',
      });

      expect(result.token).toBe('signed-jwt-token-789');
      expect(result.user.email).toBe('sophie@berlin.de');
      expect(getStoredToken()).toBe('signed-jwt-token-789');
    });

    it('throws meaningful error when registering with an existing email', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        json: async () => ({ error: 'EMAIL_ALREADY_EXISTS', message: 'Email already registered' }),
      } as Response);

      await expect(
        registerUser({
          name: 'Duplicate User',
          email: 'exists@example.de',
          password: 'securepassword123',
        })
      ).rejects.toThrow('Email already registered');
    });
  });

  describe('Session Verification & Restoration', () => {
    it('restores user profile if token is verified by backend', async () => {
      const mockUser: User = {
        id: 'usr-verify',
        email: 'verify@example.de',
        name: 'Verified User',
        role: 'admin',
      };
      setStoredSession('valid-token', mockUser);

      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({ valid: true, user: mockUser }),
      } as Response);

      const restored = await verifyStoredSession();
      expect(restored).toEqual(mockUser);
      expect(getStoredToken()).toBe('valid-token');
    });

    it('clears session if backend token verification fails (expired or revoked)', async () => {
      setStoredSession('expired-token', {
        id: 'usr-exp',
        email: 'exp@example.de',
        name: 'Exp User',
        role: 'customer',
      });

      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 401,
        json: async () => ({ valid: false, error: 'INVALID_TOKEN' }),
      } as Response);

      const restored = await verifyStoredSession();
      expect(restored).toBeNull();
      expect(getStoredToken()).toBeNull();
      expect(getStoredUser()).toBeNull();
    });
  });

  describe('Header Authentication Controls', () => {
    it('provides onOpenAuth callback for guest users', () => {
      const onOpenAuth = vi.fn();
      const el = React.createElement(Header, {
        currentUser: null,
        onOpenAuth,
      });
      expect(el.props.currentUser).toBeNull();
      el.props.onOpenAuth?.('login');
      expect(onOpenAuth).toHaveBeenCalledWith('login');
    });

    it('provides user identity and role when authenticated', () => {
      const mockUser: User = {
        id: 'usr-100',
        email: 'admin@event.de',
        name: 'Admin User',
        role: 'admin',
      };
      const onLogout = vi.fn();
      const el = React.createElement(Header, {
        currentUser: mockUser,
        onLogout,
      });
      expect(el.props.currentUser?.name).toBe('Admin User');
      expect(el.props.currentUser?.role).toBe('admin');
      el.props.onLogout?.();
      expect(onLogout).toHaveBeenCalled();
    });
  });

  describe('CheckoutForm Attendee Pre-fill', () => {
    it('accepts currentUser prop to pre-populate attendee information', () => {
      const mockUser: User = {
        id: 'usr-buyer',
        email: 'buyer@example.de',
        name: 'Ticket Buyer',
        role: 'customer',
      };
      const mockEvent: EventItem = {
        id: 'evt-1',
        title: 'Event',
        description: '',
        category: 'CONCERT',
        venueName: 'Arena',
        venueLocation: 'Munich',
        startTime: '2026-11-15T20:00:00Z',
        endTime: '2026-11-15T23:00:00Z',
        status: 'ACTIVE',
        tiers: [],
      };
      const el = React.createElement(CheckoutForm, {
        event: mockEvent,
        selectedSeats: [],
        conflictError: null,
        conflictingSeats: [],
        submitting: false,
        onSubmit: vi.fn(),
        onRefreshMap: vi.fn(),
        currentUser: mockUser,
      });
      expect(el.props.currentUser?.name).toBe('Ticket Buyer');
      expect(el.props.currentUser?.email).toBe('buyer@example.de');
    });
  });
});

