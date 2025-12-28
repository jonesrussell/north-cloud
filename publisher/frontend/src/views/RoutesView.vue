<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Routes</h1>
      <p class="page-description">Configure routing rules from sources to channels</p>
    </div>

    <div class="card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <div>
          <label class="form-checkbox">
            <input type="checkbox" v-model="enabledOnly" @change="loadRoutes">
            Show enabled only
          </label>
        </div>
        <button class="btn btn-primary" @click="openCreateModal">
          + Add Route
        </button>
      </div>

      <div v-if="loading" class="loading">Loading routes...</div>

      <div v-else-if="error" class="alert alert-error">{{ error }}</div>

      <table v-else class="table">
        <thead>
          <tr>
            <th>Source</th>
            <th>Channel</th>
            <th>Min Quality</th>
            <th>Topics</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="route in routes" :key="route.id">
            <td>
              <strong>{{ route.source_name }}</strong><br>
              <small style="color: #666;">{{ route.source_index_pattern }}</small>
            </td>
            <td><code>{{ route.channel_name }}</code></td>
            <td>{{ route.min_quality_score }}</td>
            <td>
              <span v-if="route.topics && route.topics.length > 0">
                <span v-for="topic in route.topics" :key="topic" class="badge badge-success" style="margin-right: 0.25rem;">
                  {{ topic }}
                </span>
              </span>
              <span v-else style="color: #999;">All</span>
            </td>
            <td>
              <span :class="route.enabled ? 'badge badge-success' : 'badge badge-danger'">
                {{ route.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </td>
            <td>
              <button class="btn btn-sm btn-primary" @click="openEditModal(route)" style="margin-right: 0.5rem;">
                Edit
              </button>
              <button class="btn btn-sm btn-danger" @click="deleteRoute(route)">
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="!loading && routes.length === 0" style="text-align: center; padding: 2rem; color: #666;">
        No routes found. Click "Add Route" to create one.
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <div class="modal-header">
          <h2 class="modal-title">{{ isEditing ? 'Edit Route' : 'Create Route' }}</h2>
          <button class="modal-close" @click="closeModal">&times;</button>
        </div>

        <div v-if="modalError" class="alert alert-error">{{ modalError }}</div>

        <form @submit.prevent="saveRoute">
          <div class="form-group">
            <label class="form-label">Source *</label>
            <select class="form-select" v-model="formData.source_id" required>
              <option value="">Select a source...</option>
              <option v-for="source in sources" :key="source.id" :value="source.id">
                {{ source.name }} ({{ source.index_pattern }})
              </option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Channel *</label>
            <select class="form-select" v-model="formData.channel_id" required>
              <option value="">Select a channel...</option>
              <option v-for="channel in channels" :key="channel.id" :value="channel.id">
                {{ channel.name }}
              </option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label">Minimum Quality Score</label>
            <input
              type="number"
              class="form-input"
              v-model.number="formData.min_quality_score"
              min="0"
              max="100"
            >
            <small style="color: #666;">Only publish articles with quality score >= this value (0-100)</small>
          </div>

          <div class="form-group">
            <label class="form-label">Topics</label>
            <input
              type="text"
              class="form-input"
              v-model="topicsInput"
              placeholder="e.g., crime, news, local"
            >
            <small style="color: #666;">Comma-separated list of topics to filter (leave empty for all topics)</small>
          </div>

          <div class="form-group">
            <label class="form-checkbox">
              <input type="checkbox" v-model="formData.enabled">
              Enabled
            </label>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeModal">
              Cancel
            </button>
            <button type="submit" class="btn btn-success" :disabled="saving">
              {{ saving ? 'Saving...' : 'Save' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { routesAPI, sourcesAPI, channelsAPI } from '../api/publisher'

const routes = ref([])
const sources = ref([])
const channels = ref([])
const loading = ref(false)
const error = ref(null)
const enabledOnly = ref(false)

const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref(null)
const saving = ref(false)
const formData = ref({
  source_id: '',
  channel_id: '',
  min_quality_score: 50,
  topics: [],
  enabled: true
})
const currentRoute = ref(null)
const topicsInput = ref('')

const loadRoutes = async () => {
  loading.value = true
  error.value = null
  try {
    const response = await routesAPI.list(enabledOnly.value)
    routes.value = response.data.routes || []
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to load routes'
  } finally {
    loading.value = false
  }
}

const loadSources = async () => {
  try {
    const response = await sourcesAPI.list(true) // Only enabled sources
    sources.value = response.data.sources || []
  } catch (err) {
    console.error('Failed to load sources:', err)
  }
}

const loadChannels = async () => {
  try {
    const response = await channelsAPI.list(true) // Only enabled channels
    channels.value = response.data.channels || []
  } catch (err) {
    console.error('Failed to load channels:', err)
  }
}

const openCreateModal = () => {
  isEditing.value = false
  formData.value = {
    source_id: '',
    channel_id: '',
    min_quality_score: 50,
    topics: [],
    enabled: true
  }
  topicsInput.value = ''
  currentRoute.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (route) => {
  isEditing.value = true
  formData.value = {
    source_id: route.source_id,
    channel_id: route.channel_id,
    min_quality_score: route.min_quality_score,
    topics: route.topics || [],
    enabled: route.enabled
  }
  topicsInput.value = (route.topics || []).join(', ')
  currentRoute.value = route
  modalError.value = null
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  formData.value = {
    source_id: '',
    channel_id: '',
    min_quality_score: 50,
    topics: [],
    enabled: true
  }
  topicsInput.value = ''
  currentRoute.value = null
  modalError.value = null
}

const saveRoute = async () => {
  saving.value = true
  modalError.value = null

  // Parse topics from comma-separated input
  const topics = topicsInput.value
    .split(',')
    .map(t => t.trim())
    .filter(t => t.length > 0)

  const payload = {
    ...formData.value,
    topics: topics.length > 0 ? topics : null
  }

  try {
    if (isEditing.value) {
      await routesAPI.update(currentRoute.value.id, payload)
    } else {
      await routesAPI.create(payload)
    }
    closeModal()
    await loadRoutes()
  } catch (err) {
    modalError.value = err.response?.data?.error || 'Failed to save route'
  } finally {
    saving.value = false
  }
}

const deleteRoute = async (route) => {
  if (!confirm(`Are you sure you want to delete the route from "${route.source_name}" to "${route.channel_name}"?`)) {
    return
  }

  try {
    await routesAPI.delete(route.id)
    await loadRoutes()
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to delete route'
  }
}

onMounted(() => {
  loadRoutes()
  loadSources()
  loadChannels()
})
</script>
