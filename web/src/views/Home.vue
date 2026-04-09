<!--
  Home.vue — Landing page.
  Shows all public modules. Search by module name or owner.
  Loads modules on mount (no auth required).
-->
<script setup>
import { ref, onMounted } from 'vue'
import * as api from '../lib/api.js'

const query = ref('')
const modules = ref([])
const error = ref('')

async function search() {
  error.value = ''
  try {
    modules.value = await api.searchModules(query.value.trim())
  } catch (e) {
    error.value = e.message
  }
}

onMounted(search)
</script>

<template>
  <div>
    <h2><i class="fas fa-cubes"></i> Modules</h2>
    <div class="row mb-3">
      <div class="col-md-6">
        <div class="input-group">
          <input type="text" class="form-control" placeholder="Search modules..."
                 v-model="query" @keyup.enter="search" />
          <button class="btn btn-primary" @click="search">
            <i class="fas fa-search"></i> Search
          </button>
        </div>
      </div>
    </div>
    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <div v-if="modules.length === 0" class="text-muted">No modules found.</div>
    <div class="list-group">
      <router-link v-for="m in modules" :key="m.id"
                   :to="'/modules/' + m.owner + '/' + m.name"
                   class="list-group-item list-group-item-action">
        <div class="d-flex justify-content-between align-items-center">
          <div><i class="fas fa-cube"></i> <strong>{{ m.owner }}/{{ m.name }}</strong></div>
          <span class="badge" :class="m.visibility === 'public' ? 'bg-success' : 'bg-secondary'">
            {{ m.visibility }}
          </span>
        </div>
      </router-link>
    </div>
  </div>
</template>
