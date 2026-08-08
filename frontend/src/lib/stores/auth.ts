import { reactive } from 'vue';
import { setTokens, clearTokens, loadTokens, getAccessToken } from '@/lib/api/transport';

const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

interface User {
  userId: string;
  email: string;
  name: string;
}

interface AuthState {
  isAuthenticated: boolean;
  user: User | null;
  loading: boolean;
}

const state = reactive<AuthState>({
  isAuthenticated: false,
  user: null,
  loading: true
});

export function useAuthStore() {
  async function login(email: string, password: string): Promise<boolean> {
    try {
      const resp = await fetch(API_BASE + '/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password })
      });
      if (!resp.ok) return false;
      const data = await resp.json();
      setTokens(data.accessToken, data.refreshToken);
      state.isAuthenticated = true;
      state.user = { userId: data.userId, email: data.email, name: data.name };
      state.loading = false;
      return true;
    } catch {
      return false;
    }
  }

  async function signup(email: string, password: string, name: string): Promise<boolean> {
    try {
      const resp = await fetch(API_BASE + '/api/auth/signup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password, name })
      });
      if (!resp.ok) return false;
      const data = await resp.json();
      setTokens(data.accessToken, data.refreshToken);
      state.isAuthenticated = true;
      state.user = { userId: data.userId, email: data.email, name: data.name };
      state.loading = false;
      return true;
    } catch {
      return false;
    }
  }

  function logout() {
    clearTokens();
    state.isAuthenticated = false;
    state.user = null;
    state.loading = false;
  }

  function setUser(user: User) {
    state.user = user;
  }

  async function checkAuth() {
    const { accessToken } = loadTokens();
    if (!accessToken) {
      state.isAuthenticated = false;
      state.user = null;
      state.loading = false;
      return;
    }
    try {
      const resp = await fetch(API_BASE + '/api/me/profile', {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${getAccessToken()}`
        }
      });
      if (!resp.ok) {
        clearTokens();
        state.isAuthenticated = false;
        state.user = null;
        state.loading = false;
        return;
      }
      const data = await resp.json();
      state.isAuthenticated = true;
      state.user = { userId: data.userId, email: data.email, name: data.name };
      state.loading = false;
    } catch {
      clearTokens();
      state.isAuthenticated = false;
      state.user = null;
      state.loading = false;
    }
  }

  return { state, login, signup, logout, setUser, checkAuth };
}