import { useState, useCallback } from 'react';

const TOKEN_KEY = 'iss_token';
const USER_KEY = 'iss_user';

export interface AuthUser {
  id: number;
  username: string;
  name: string;
  role: string;
}

export function getToken(): string | null {
  return sessionStorage.getItem(TOKEN_KEY);
}

export function getUser(): AuthUser | null {
  const raw = sessionStorage.getItem(USER_KEY);
  return raw ? JSON.parse(raw) : null;
}

export function setAuth(token: string, user: AuthUser) {
  sessionStorage.setItem(TOKEN_KEY, token);
  sessionStorage.setItem(USER_KEY, JSON.stringify(user));
}

export function clearAuth() {
  sessionStorage.removeItem(TOKEN_KEY);
  sessionStorage.removeItem(USER_KEY);
}

export function useAuth() {
  const [user, setUser] = useState<AuthUser | null>(getUser);
  const [token, setToken] = useState<string | null>(getToken);

  const login = useCallback((t: string, u: AuthUser) => {
    setAuth(t, u);
    setToken(t);
    setUser(u);
  }, []);

  const logout = useCallback(() => {
    clearAuth();
    setToken(null);
    setUser(null);
  }, []);

  return { user, token, isLoggedIn: !!token, login, logout };
}
