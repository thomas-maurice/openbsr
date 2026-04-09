<!--
  Login.vue — Login / Register page.
  Supports toggle between login and register mode.
  On success, redirects to the returnTo query param or home.
-->
<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const error = ref('')
const registerMode = ref(false)

async function submit() {
  error.value = ''
  try {
    if (registerMode.value) {
      await auth.register(username.value, password.value)
    }
    await auth.login(username.value, password.value)
    router.push(route.query.returnTo || '/')
  } catch (e) {
    error.value = e.message
  }
}
</script>

<template>
  <div class="row justify-content-center">
    <div class="col-md-4">
      <h2><i class="fas fa-sign-in-alt"></i> {{ registerMode ? 'Register' : 'Login' }}</h2>
      <div v-if="error" class="alert alert-danger">{{ error }}</div>
      <form @submit.prevent="submit">
        <div class="mb-3">
          <label class="form-label">Username</label>
          <input type="text" class="form-control" v-model="username"
                 autocomplete="username" required />
        </div>
        <div class="mb-3">
          <label class="form-label">Password</label>
          <input type="password" class="form-control" v-model="password"
                 :autocomplete="registerMode ? 'new-password' : 'current-password'" required />
        </div>
        <button type="submit" class="btn btn-primary w-100">
          {{ registerMode ? 'Register & Login' : 'Login' }}
        </button>
      </form>
      <p class="mt-3 text-center">
        <a href="#" @click.prevent="registerMode = !registerMode">
          {{ registerMode ? 'Already have an account? Login' : 'Need an account? Register' }}
        </a>
      </p>
    </div>
  </div>
</template>
