<!--
  CreateOrg.vue — Form to create a new organization.
-->
<script setup>
import { ref } from 'vue'
import * as api from '../lib/api.js'

const name = ref('')
const error = ref('')
const success = ref('')

async function submit() {
  error.value = ''
  success.value = ''
  try {
    const data = await api.createOrg(name.value)
    success.value = 'Organization created: ' + data.name
    name.value = ''
  } catch (e) { error.value = e.message }
}
</script>

<template>
  <div class="row justify-content-center">
    <div class="col-md-6">
      <h2><i class="fas fa-building"></i> Create Organization</h2>
      <div v-if="error" class="alert alert-danger">{{ error }}</div>
      <div v-if="success" class="alert alert-success">{{ success }}</div>
      <form @submit.prevent="submit">
        <div class="mb-3">
          <label class="form-label">Organization Name</label>
          <input type="text" class="form-control" v-model="name" required
                 placeholder="lowercase, 3-39 chars, alphanumeric and hyphens" />
        </div>
        <button type="submit" class="btn btn-primary">Create</button>
      </form>
    </div>
  </div>
</template>
