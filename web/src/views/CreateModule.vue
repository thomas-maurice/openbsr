<!--
  CreateModule.vue — Form to create a new protobuf module.
  Owner defaults to the current user's username.
-->
<script setup>
import { ref } from 'vue'
import { useAuthStore } from '../stores/auth.js'
import * as api from '../lib/api.js'

const auth = useAuthStore()
const owner = ref('')
const name = ref('')
const visibility = ref('public')
const error = ref('')
const success = ref('')

async function submit() {
  error.value = ''
  success.value = ''
  try {
    const data = await api.createModule(owner.value || auth.user?.username, name.value, visibility.value)
    success.value = 'Module created: ' + data.owner + '/' + data.name
    name.value = ''
  } catch (e) { error.value = e.message }
}
</script>

<template>
  <div class="row justify-content-center">
    <div class="col-md-6">
      <h2><i class="fas fa-plus-circle"></i> Create Module</h2>
      <div v-if="error" class="alert alert-danger">{{ error }}</div>
      <div v-if="success" class="alert alert-success">{{ success }}</div>
      <form @submit.prevent="submit">
        <div class="mb-3">
          <label class="form-label">Owner</label>
          <input type="text" class="form-control" v-model="owner"
                 :placeholder="auth.user?.username || 'owner'" />
        </div>
        <div class="mb-3">
          <label class="form-label">Module Name</label>
          <input type="text" class="form-control" v-model="name" required />
        </div>
        <div class="mb-3">
          <label class="form-label">Visibility</label>
          <select class="form-select" v-model="visibility">
            <option value="public">Public</option>
            <option value="private">Private</option>
          </select>
        </div>
        <button type="submit" class="btn btn-primary">Create</button>
      </form>
    </div>
  </div>
</template>
