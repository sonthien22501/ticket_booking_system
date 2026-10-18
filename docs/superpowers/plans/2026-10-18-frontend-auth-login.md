# Frontend Authentication & User Profile Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate complete user authentication (Login, Registration, Token Persistence, and Profile Header UI) into the React frontend, connecting to the existing Go `auth-service` and API Gateway.

**Architecture:** Create an `auth.ts` service for JWT token management and API integration (`/api/auth/login`, `/api/auth/register`, `/api/auth/verify`), an accessible `AuthModal.tsx` supporting Sign In and Sign Up tabs, update `Header.tsx` with user state and logout controls, and wire attendee auto-fill into `CheckoutForm.tsx` and `App.tsx`.

**Tech Stack:** React 18, TypeScript 5, Vitest, CSS3 variables (Light/Dark mode compliant).

**Spec:** Requirement R3.1 (Stateless Authentication & Authorization), Requirement R1 (React Frontend), Phase 2 in `TODO.md`.

## Global Constraints
- Token key in `localStorage` must be `ticketverse_auth_token`.
- User key in `localStorage` must be `ticketverse_auth_user`.
- Must support both `/api/auth/*` and direct API Gateway routes.
- Fully compatible with Dark Mode and Light Mode theme tokens.
- No external state libraries; use lightweight React hooks and services.

## Review Focus
1. Invalid credentials return informative error messages without unhandled exceptions.
2. Malformed or expired JWT tokens in `localStorage` gracefully clear storage and reset to guest state.
3. Form validation prevents empty email, malformed email, or short password submissions before API calls.
4. Successful login/registration immediately updates `Header.tsx` without needing a full page reload.
5. Logging in auto-populates attendee details in `CheckoutForm.tsx` while allowing manual edits if desired.

---

### Task 1: Auth Types & Storage Service

**Files:**
- Modify: `frontend/src/types/index.ts`
- Create: `frontend/src/services/auth.ts`
- Test: `frontend/src/test/frontend_auth.test.ts`

**Interfaces:**
- Produces: `getStoredToken()`, `getStoredUser()`, `setStoredSession()`, `clearStoredSession()`, `login()`, `register()`, `verifyStoredSession()`

- [ ] **Step 1: Write the failing unit tests for auth service and storage in `frontend/src/test/frontend_auth.test.ts`**
- [ ] **Step 2: Run test to verify it fails (RED)**
- [ ] **Step 3: Implement `types/index.ts` and `services/auth.ts`**
- [ ] **Step 4: Run test to verify it passes (GREEN)**
- [ ] **Step 5: Format & Commit**

---

### Task 2: AuthModal Component (Login & Register Tabs)

**Files:**
- Create: `frontend/src/components/AuthModal.tsx`
- Modify: `frontend/src/components/Icons.tsx` (add UserIcon, LockIcon, LogInIcon, LogOutIcon if missing)
- Modify: `frontend/src/index.css` (add modal, tab, and form styling)
- Test: `frontend/src/test/frontend_auth.test.ts`

**Interfaces:**
- Props: `isOpen: boolean`, `onClose: () => void`, `onAuthSuccess: (user: User, token: string) => void`

- [ ] **Step 1: Write failing tests for modal tab switching, field validation, and error display**
- [ ] **Step 2: Run test to verify failure (RED)**
- [ ] **Step 3: Implement `AuthModal.tsx` with Sign In / Register toggle, form handlers, and accessible modal markup**
- [ ] **Step 4: Run test to verify it passes (GREEN)**
- [ ] **Step 5: Format & Commit**

---

### Task 3: Header & App Integration (Profile Badge & Auto-fill Checkout)

**Files:**
- Modify: `frontend/src/components/Header.tsx`
- Modify: `frontend/src/components/CheckoutForm.tsx`
- Modify: `frontend/src/App.tsx`
- Test: `frontend/src/test/frontend_auth.test.ts`

**Interfaces:**
- `HeaderProps`: include `currentUser: User | null`, `onOpenAuth: () => void`, `onLogout: () => void`
- `CheckoutForm`: pre-fills customer name and email from `currentUser`

- [ ] **Step 1: Write failing tests for header authentication states and checkout pre-fill**
- [ ] **Step 2: Run test to verify failure (RED)**
- [ ] **Step 3: Implement Header profile pill, logout handler, App state management, and CheckoutForm pre-fill**
- [ ] **Step 4: Run test to verify it passes (GREEN)**
- [ ] **Step 5: Format & Commit**

---

### Task 4: Docker Container Rebuild & Live Acceptance Verification

**Files:**
- Command: `docker compose build frontend && docker compose up -d frontend`
- Verification: cURL and HTTP check on `http://localhost:3000`

- [ ] **Step 1: Build production frontend bundle and Docker image**
- [ ] **Step 2: Re-deploy `ticket_frontend` container**
- [ ] **Step 3: Verify frontend health and live HTML output**
