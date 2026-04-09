// stores/auth.js — Authentication state (Pinia).
// Manages the current user and token. Persists token in localStorage.
// On app load, tries to restore the session by calling /api/v1/auth/me.

import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as api from '../lib/api.js'

export const useAuthStore = defineStore('auth', () => {
  // Current authenticated user (null if not logged in)
  const user = ref(null)

  // Try to restore session from localStorage
  async function restore() {
    const token = localStorage.getItem('bsr_token')
    if (!token) return
    try {
      user.value = await api.getMe()
    } catch {
      localStorage.removeItem('bsr_token')
    }
  }

  // Log in with username/password. Stores token and sets user.
  async function login(username, password) {
    const data = await api.login(username, password)
    localStorage.setItem('bsr_token', data.token)
    user.value = { id: data.user_id, username: data.username }
    return data
  }

  // Register a new account.
  async function register(username, password) {
    return api.register(username, password)
  }

  // Log out. Clears token and user.
  function logout() {
    localStorage.removeItem('bsr_token')
    user.value = null
  }

  return { user, restore, login, register, logout }
})
