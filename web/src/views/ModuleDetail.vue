<!--
  ModuleDetail.vue — Module detail page.
  Shows labels, file tree, proto source with syntax highlighting, and commit history.
  Auto-loads the first file on mount.
-->
<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import * as api from '../lib/api.js'
import hljs from 'highlight.js/lib/core'
import protobuf from 'highlight.js/lib/languages/protobuf'
import 'highlight.js/styles/stackoverflow-dark.min.css'

// Register protobuf language for highlight.js
hljs.registerLanguage('protobuf', protobuf)

const props = defineProps({ owner: String, repo: String })

const mod = ref(null)
const commits = ref([])
const labels = ref([])
const files = ref([])
const selectedFile = ref(null)
const fileContent = ref('')
const error = ref('')

// Syntax-highlighted HTML for the selected file
const highlightedHtml = computed(() => {
  if (!fileContent.value) return ''
  const result = hljs.highlight(fileContent.value, { language: 'protobuf' })
  // Add line numbers
  return result.value.split('\n').map((line, i) =>
    `<span class="hljs-line-num" style="color:#636d83;display:inline-block;width:3em;text-align:right;margin-right:1.5em;user-select:none">${i + 1}</span>${line}`
  ).join('\n')
})

async function selectFile(path) {
  if (!commits.value.length) return
  selectedFile.value = path
  try {
    const data = await api.getFileContent(props.owner, props.repo, commits.value[0].id, path)
    fileContent.value = atob(data.content)
  } catch (e) {
    fileContent.value = 'Failed to load: ' + e.message
  }
}

onMounted(async () => {
  try {
    mod.value = await api.getModule(props.owner, props.repo)
    commits.value = await api.listCommits(props.owner, props.repo)
    labels.value = await api.listLabels(props.owner, props.repo)
    if (commits.value.length > 0) {
      try {
        files.value = await api.listFiles(props.owner, props.repo, commits.value[0].id)
        if (files.value.length > 0) await selectFile(files.value[0].path)
      } catch { /* no files yet */ }
    }
  } catch (e) { error.value = e.message }
})
</script>

<template>
  <div>
    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <div v-if="mod">
      <h2>
        <i class="fas fa-cube"></i>
        <router-link :to="'/owner/' + mod.owner">{{ mod.owner }}</router-link> / {{ mod.name }}
      </h2>
      <span class="badge mb-3" :class="mod.visibility === 'public' ? 'bg-success' : 'bg-secondary'">
        {{ mod.visibility }}
      </span>

      <!-- File browser + viewer -->
      <div class="row mt-4">
        <!-- Sidebar: files and labels -->
        <div class="col-md-3">
          <h5><i class="fas fa-folder-tree"></i> Files</h5>
          <div v-if="files.length === 0" class="text-muted">No files yet.</div>
          <div class="list-group list-group-flush">
            <a v-for="f in files" :key="f.path" href="#"
               @click.prevent="selectFile(f.path)"
               class="list-group-item list-group-item-action py-1 px-2 small"
               :class="{ active: selectedFile === f.path }">
              <i class="fas fa-file-code"></i> {{ f.path }}
            </a>
          </div>

          <h5 class="mt-4"><i class="fas fa-tags"></i> Labels</h5>
          <div v-if="labels.length === 0" class="text-muted">No labels yet.</div>
          <div v-for="l in labels" :key="l.name" class="small mb-1">
            <i class="fas fa-tag"></i> {{ l.name }}
            <code class="ms-1">{{ l.commit_id.substring(0, 12) }}</code>
          </div>
        </div>

        <!-- Main: file viewer with syntax highlighting -->
        <div class="col-md-9">
          <div v-if="selectedFile">
            <h5><i class="fas fa-file-code"></i> {{ selectedFile }}</h5>
            <pre class="rounded p-3 hljs" style="overflow-x:auto"><code v-html="highlightedHtml"></code></pre>
          </div>
          <div v-else class="text-muted">Select a file to view.</div>
        </div>
      </div>

      <!-- Commit history -->
      <h4 class="mt-4"><i class="fas fa-clock-rotate-left"></i> Commits</h4>
      <div v-if="commits.length === 0" class="text-muted">No commits yet.</div>
      <table v-else class="table table-sm">
        <thead><tr><th>ID</th><th>Created</th></tr></thead>
        <tbody>
          <tr v-for="c in commits" :key="c.id">
            <td><code>{{ c.id.substring(0, 16) }}</code></td>
            <td>{{ c.created_at }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
