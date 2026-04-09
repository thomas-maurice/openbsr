<!--
  OwnerProfile.vue — Shows a user or organization profile with their modules.
  Auto-detects whether the owner is a user or org.
-->
<script setup>
import { ref, onMounted } from 'vue'
import * as api from '../lib/api.js'

const props = defineProps({ name: String })
const owner = ref(null)
const modules = ref([])
const isOrg = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    owner.value = await api.getUser(props.name)
  } catch {
    try {
      owner.value = await api.getOrg(props.name)
      isOrg.value = true
    } catch (e) {
      error.value = 'Owner not found'
      return
    }
  }
  try { modules.value = await api.listModules(props.name) } catch { /* empty is fine */ }
})
</script>

<template>
  <div>
    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <div v-if="owner">
      <h2>
        <i :class="isOrg ? 'fas fa-building' : 'fas fa-user'"></i>
        {{ owner.username || owner.name }}
      </h2>
      <span class="badge bg-info mb-3">{{ isOrg ? 'Organization' : 'User' }}</span>

      <h4 class="mt-3"><i class="fas fa-cubes"></i> Modules</h4>
      <div v-if="modules.length === 0" class="text-muted">No modules.</div>
      <div class="list-group">
        <router-link v-for="m in modules" :key="m.id"
                     :to="'/modules/' + m.owner + '/' + m.name"
                     class="list-group-item list-group-item-action">
          <i class="fas fa-cube"></i> {{ m.name }}
          <span class="badge float-end" :class="m.visibility === 'public' ? 'bg-success' : 'bg-secondary'">
            {{ m.visibility }}
          </span>
        </router-link>
      </div>
    </div>
  </div>
</template>
