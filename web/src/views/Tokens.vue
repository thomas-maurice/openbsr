<!--
  Tokens.vue — API token management.
  List, create, and revoke API tokens. Login session tokens are filtered out server-side.
-->
<script setup>
import { ref, onMounted } from 'vue'
import * as api from '../lib/api.js'

const tokens = ref([])
const note = ref('')
const newToken = ref('')
const error = ref('')

async function load() {
  try { tokens.value = await api.listTokens() } catch (e) { error.value = e.message }
}

async function create() {
  error.value = ''
  newToken.value = ''
  try {
    const data = await api.createToken(note.value)
    newToken.value = data.token
    note.value = ''
    await load()
  } catch (e) { error.value = e.message }
}

async function revoke(id) {
  try { await api.revokeToken(id); await load() } catch (e) { error.value = e.message }
}

onMounted(load)
</script>

<template>
  <div>
    <h2><i class="fas fa-key"></i> API Tokens</h2>
    <div v-if="error" class="alert alert-danger">{{ error }}</div>

    <div class="card mb-3">
      <div class="card-body">
        <h5>Create Token</h5>
        <div class="input-group">
          <input type="text" class="form-control" placeholder="Note (optional)" v-model="note" />
          <button class="btn btn-primary" @click="create">Create</button>
        </div>
        <div v-if="newToken" class="alert alert-success mt-2">
          <strong>New token:</strong> <code>{{ newToken }}</code>
          <br /><small>Copy it now — it won't be shown again.</small>
        </div>
      </div>
    </div>

    <table class="table">
      <thead><tr><th>ID</th><th>Note</th><th></th></tr></thead>
      <tbody>
        <tr v-for="t in tokens" :key="t.id">
          <td><code>{{ t.id.substring(0, 12) }}</code></td>
          <td>{{ t.note || '—' }}</td>
          <td>
            <button class="btn btn-sm btn-danger" @click="revoke(t.id)">
              <i class="fas fa-trash"></i> Revoke
            </button>
          </td>
        </tr>
      </tbody>
    </table>
    <div v-if="tokens.length === 0" class="text-muted">No tokens.</div>
  </div>
</template>
