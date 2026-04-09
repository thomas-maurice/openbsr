// lib/api.js — HTTP client for the OpenBSR REST + ConnectRPC API.
// All functions return parsed JSON. Throws on non-2xx responses.

const API = '/api/v1'

function getToken() {
  return localStorage.getItem('bsr_token')
}

function headers() {
  const h = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) h['Authorization'] = 'Bearer ' + token
  return h
}

function enc(s) {
  return encodeURIComponent(s)
}

async function request(method, path, body) {
  const opts = { method, headers: headers() }
  if (body) opts.body = JSON.stringify(body)
  const res = await fetch(API + path, opts)
  let data
  try { data = await res.json() } catch { data = { error: res.statusText } }
  if (!res.ok) throw new Error(data.error || 'request failed')
  return data
}

// --- Auth ---
export const login = (username, password) => request('POST', '/auth/login', { username, password })
export const register = (username, password) => request('POST', '/auth/register', { username, password })
export const getMe = () => request('GET', '/auth/me')
export const listTokens = () => request('GET', '/auth/tokens')
export const createToken = (note) => request('POST', '/auth/tokens', { note })
export const revokeToken = (id) => request('DELETE', '/auth/tokens/' + enc(id))

// --- Users & Orgs ---
export const getUser = (username) => request('GET', '/users/' + enc(username))
export const getOrg = (name) => request('GET', '/orgs/' + enc(name))
export const createOrg = (name) => request('POST', '/orgs', { name })

// --- Modules ---
export const searchModules = (query) => request('GET', '/modules' + (query ? '?q=' + enc(query) : ''))
export const listModules = (owner) => request('GET', '/modules?owner=' + enc(owner))
export const getModule = (owner, repo) => request('GET', '/modules/' + enc(owner) + '/' + enc(repo))
export const createModule = (owner, name, visibility) => request('POST', '/modules', { owner, name, visibility })

// --- Commits & Labels ---
export const listCommits = (owner, repo) => request('GET', '/modules/' + enc(owner) + '/' + enc(repo) + '/commits')
export const listLabels = (owner, repo) => request('GET', '/modules/' + enc(owner) + '/' + enc(repo) + '/labels')
export const listFiles = (owner, repo, commitId) => request('GET', '/modules/' + enc(owner) + '/' + enc(repo) + '/commits/' + enc(commitId) + '/files')
export const getFileContent = (owner, repo, commitId, path) => request('GET', '/modules/' + enc(owner) + '/' + enc(repo) + '/commits/' + enc(commitId) + '/file?path=' + enc(path))
