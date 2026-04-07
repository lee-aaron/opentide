import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './auth'

describe('auth store', () => {
  beforeEach(() => {
    useAuthStore.setState({ authenticated: false, demo: false, loading: true })
  })

  it('starts unauthenticated and loading', () => {
    const state = useAuthStore.getState()
    expect(state.authenticated).toBe(false)
    expect(state.loading).toBe(true)
  })

  it('setAuth updates state', () => {
    useAuthStore.getState().setAuth(true, true)
    const state = useAuthStore.getState()
    expect(state.authenticated).toBe(true)
    expect(state.demo).toBe(true)
    expect(state.loading).toBe(false)
  })

  it('setAuth defaults demo to false', () => {
    useAuthStore.getState().setAuth(true)
    expect(useAuthStore.getState().demo).toBe(false)
  })

  it('setLoading updates loading', () => {
    useAuthStore.getState().setLoading(false)
    expect(useAuthStore.getState().loading).toBe(false)
  })
})
