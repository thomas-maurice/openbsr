// router.js — Vue Router configuration.
// Each route maps to a view in src/views/.
// Uses hash-mode routing (#/path) so the Go server doesn't need SPA fallback.

import { createRouter, createWebHashHistory } from 'vue-router'
import Home from './views/Home.vue'
import Login from './views/Login.vue'
import Tokens from './views/Tokens.vue'
import CreateModule from './views/CreateModule.vue'
import CreateOrg from './views/CreateOrg.vue'
import ModuleDetail from './views/ModuleDetail.vue'
import OwnerProfile from './views/OwnerProfile.vue'
import GettingStarted from './views/GettingStarted.vue'

const routes = [
  { path: '/', component: Home },
  { path: '/login', component: Login },
  { path: '/tokens', component: Tokens, meta: { requiresAuth: true } },
  { path: '/create-module', component: CreateModule, meta: { requiresAuth: true } },
  { path: '/create-org', component: CreateOrg, meta: { requiresAuth: true } },
  { path: '/modules/:owner/:repo', component: ModuleDetail, props: true },
  { path: '/owner/:name', component: OwnerProfile, props: true },
  { path: '/getting-started', component: GettingStarted },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// Navigation guard: redirect to /login if route requires auth and user is not logged in
router.beforeEach((to) => {
  if (to.meta.requiresAuth && !localStorage.getItem('bsr_token')) {
    return { path: '/login', query: { returnTo: to.fullPath } }
  }
})

export default router
