import { User, AuthResponse, LoginCredentials, RegisterCredentials } from '../types';

export const AUTH_TOKEN_KEY = 'ticketverse_auth_token';
export const AUTH_USER_KEY = 'ticketverse_auth_user';

const API_BASE = import.meta.env?.VITE_API_URL || '/api';

/**
 * Retrieves the persisted JWT token from localStorage.
 */
export function getStoredToken(): string | null {
  try {
    return localStorage.getItem(AUTH_TOKEN_KEY);
  } catch {
    return null;
  }
}

/**
 * Retrieves the persisted User profile from localStorage.
 */
export function getStoredUser(): User | null {
  try {
    const raw = localStorage.getItem(AUTH_USER_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as User;
  } catch {
    return null;
  }
}

/**
 * Persists the authenticated token and user profile to localStorage.
 */
export function setStoredSession(token: string, user: User): void {
  try {
    localStorage.setItem(AUTH_TOKEN_KEY, token);
    localStorage.setItem(AUTH_USER_KEY, JSON.stringify(user));
  } catch (err) {
    console.error('Failed to save session to localStorage', err);
  }
}

/**
 * Clears the stored session from localStorage on logout or token expiration.
 */
export function clearStoredSession(): void {
  try {
    localStorage.removeItem(AUTH_TOKEN_KEY);
    localStorage.removeItem(AUTH_USER_KEY);
  } catch (err) {
    console.error('Failed to clear session from localStorage', err);
  }
}

/**
 * Logs in a user with email and password via the API Gateway.
 */
export async function loginUser(credentials: LoginCredentials): Promise<AuthResponse> {
  const response = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(credentials),
  });

  const data = await response.json().catch(() => ({}));

  if (!response.ok) {
    const errMsg = data.message || data.error || `Login failed with status ${response.status}`;
    throw new Error(errMsg);
  }

  const authRes: AuthResponse = {
    token: data.token,
    user: {
      id: data.user?.id || '',
      email: data.user?.email || credentials.email,
      name: data.user?.name || credentials.email.split('@')[0],
      role: data.user?.role || 'customer',
      createdAt: data.user?.createdAt,
    },
  };

  setStoredSession(authRes.token, authRes.user);
  return authRes;
}

/**
 * Registers a new user via the API Gateway.
 */
export async function registerUser(credentials: RegisterCredentials): Promise<AuthResponse> {
  const payload = {
    name: credentials.name,
    email: credentials.email,
    password: credentials.password,
    role: credentials.role || 'customer',
  };

  const response = await fetch(`${API_BASE}/auth/register`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const data = await response.json().catch(() => ({}));

  if (!response.ok) {
    const errMsg = data.message || data.error || `Registration failed with status ${response.status}`;
    throw new Error(errMsg);
  }

  const authRes: AuthResponse = {
    token: data.token,
    user: {
      id: data.user?.id || '',
      email: data.user?.email || credentials.email,
      name: data.user?.name || credentials.name,
      role: data.user?.role || 'customer',
      createdAt: data.user?.createdAt,
    },
  };

  setStoredSession(authRes.token, authRes.user);
  return authRes;
}

/**
 * Verifies the stored JWT token with the backend and returns the active User profile,
 * or clears the session if invalid or expired.
 */
export async function verifyStoredSession(): Promise<User | null> {
  const token = getStoredToken();
  if (!token) {
    return null;
  }

  try {
    const response = await fetch(`${API_BASE}/auth/verify`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      clearStoredSession();
      return null;
    }

    const data = await response.json().catch(() => ({}));
    if (data.valid && data.user) {
      const user: User = {
        id: data.user.id,
        email: data.user.email,
        name: data.user.name,
        role: data.user.role || 'customer',
      };
      setStoredSession(token, user);
      return user;
    } else {
      clearStoredSession();
      return null;
    }
  } catch {
    clearStoredSession();
    return null;
  }
}
