<!--
  App.vue — Root component.
  Renders the navbar and the current route view.
  On mount, tries to restore the user session from localStorage.
-->
<script setup>
import { onMounted } from 'vue'
import { useAuthStore } from './stores/auth.js'
import { useRouter } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()

onMounted(() => auth.restore())

function logout() {
  auth.logout()
  router.push('/')
}
</script>

<template>
  <nav class="navbar navbar-expand-lg navbar-dark bg-dark">
    <div class="container">
      <router-link class="navbar-brand" to="/">
        <i class="fas fa-cube"></i> OpenBSR
      </router-link>
      <div class="navbar-nav ms-auto">
        <router-link class="nav-link" to="/"><i class="fas fa-home"></i> Home</router-link>
        <router-link class="nav-link" to="/getting-started"><i class="fas fa-rocket"></i> Docs</router-link>
        <template v-if="auth.user">
          <router-link class="nav-link" to="/create-module"><i class="fas fa-plus"></i> Module</router-link>
          <router-link class="nav-link" to="/create-org"><i class="fas fa-building"></i> Org</router-link>
          <router-link class="nav-link" to="/tokens"><i class="fas fa-key"></i> Tokens</router-link>
          <router-link class="nav-link" :to="'/owner/' + auth.user.username">
            <i class="fas fa-user"></i> {{ auth.user.username }}
          </router-link>
          <a class="nav-link" href="#" @click.prevent="logout">
            <i class="fas fa-sign-out-alt"></i> Logout
          </a>
        </template>
        <template v-else>
          <router-link class="nav-link" to="/login"><i class="fas fa-sign-in-alt"></i> Login</router-link>
        </template>
      </div>
    </div>
  </nav>
  <div class="container mt-4">
    <router-view />
  </div>
</template>
