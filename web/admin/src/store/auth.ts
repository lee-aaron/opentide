import { create } from 'zustand'

interface AuthState {
  authenticated: boolean
  demo: boolean
  loading: boolean
  setAuth: (authenticated: boolean, demo?: boolean) => void
  setLoading: (loading: boolean) => void
}

export const useAuthStore = create<AuthState>((set) => ({
  authenticated: false,
  demo: false,
  loading: true,
  setAuth: (authenticated, demo = false) => set({ authenticated, demo, loading: false }),
  setLoading: (loading) => set({ loading }),
}))
