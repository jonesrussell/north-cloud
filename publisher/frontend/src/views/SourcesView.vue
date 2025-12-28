<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Sources</h1>
      <p class="page-description">Manage Elasticsearch indexes as content sources</p>
    </div>

    <div class="card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <div>
          <label class="form-checkbox">
            <input type="checkbox" v-model="enabledOnly" @change="loadSources">
            Show enabled only
          </label>
        </div>
        <button class="btn btn-primary" @click="openCreateModal">
          + Add Source
        </button>
      </div>

      <div v-if="loading" class="loading">Loading sources...</div>

      <div v-else-if="error" class="alert alert-error">{{ error }}</div>

      <table v-else class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Index Pattern</th>
            <th>Status</th>
            <th>Created</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="source in sources" :key="source.id">
            <td><strong>{{ source.name }}</strong></td>
            <td><code>{{ source.index_pattern }}</code></td>
            <td>
              <span :class="source.enabled ? 'badge badge-success' : 'badge badge-danger'">
                {{ source.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </td>
            <td>{{ formatDate(source.created_at) }}</td>
            <td>
              <button class="btn btn-sm btn-primary" @click="openEditModal(source)" style="margin-right: 0.5rem;">
                Edit
              </button>
              <button class="btn btn-sm btn-danger" @click="deleteSource(source)">
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="!loading && sources.length === 0" style="text-align: center; padding: 2rem; color: #666;">
        No sources found. Click "Add Source" to create one.
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <div class="modal-header">
          <h2 class="modal-title">{{ isEditing ? 'Edit Source' : 'Create Source' }}</h2>
          <button class="modal-close" @click="closeModal">&times;</button>
        </div>

        <div v-if="modalError" class="alert alert-error">{{ modalError }}</div>

        <form @submit.prevent="saveSource">
          <div class="form-group">
            <label class="form-label">Name *</label>
            <input
              type="text"
              class="form-input"
              v-model="formData.name"
              placeholder="e.g., sudbury_com"
              required
            >
          </div>

          <div class="form-group">
            <label class="form-label">Index Pattern *</label>
            <input
              type="text"
              class="form-input"
              v-model="formData.index_pattern"
              placeholder="e.g., sudbury_com_classified_content"
              required
            >
            <small style="color: #666;">Elasticsearch index pattern to query</small>
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
import { ref, onMounted } from 'vue'
import { sourcesAPI } from '../api/publisher'

const sources = ref([])
const loading = ref(false)
const error = ref(null)
const enabledOnly = ref(false)

const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref(null)
const saving = ref(false)
const formData = ref({
  name: '',
  index_pattern: '',
  enabled: true
})
const currentSource = ref(null)

const loadSources = async () => {
  loading.value = true
  error.value = null
  try {
    const response = await sourcesAPI.list(enabledOnly.value)
    sources.value = response.data.sources || []
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to load sources'
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  isEditing.value = false
  formData.value = {
    name: '',
    index_pattern: '',
    enabled: true
  }
  currentSource.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (source) => {
  isEditing.value = true
  formData.value = {
    name: source.name,
    index_pattern: source.index_pattern,
    enabled: source.enabled
  }
  currentSource.value = source
  modalError.value = null
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  formData.value = { name: '', index_pattern: '', enabled: true }
  currentSource.value = null
  modalError.value = null
}

const saveSource = async () => {
  saving.value = true
  modalError.value = null
  try {
    if (isEditing.value) {
      await sourcesAPI.update(currentSource.value.id, formData.value)
    } else {
      await sourcesAPI.create(formData.value)
    }
    closeModal()
    await loadSources()
  } catch (err) {
    modalError.value = err.response?.data?.error || 'Failed to save source'
  } finally {
    saving.value = false
  }
}

const deleteSource = async (source) => {
  if (!confirm(`Are you sure you want to delete source "${source.name}"?`)) {
    return
  }

  try {
    await sourcesAPI.delete(source.id)
    await loadSources()
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to delete source'
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString()
}

onMounted(() => {
  loadSources()
})
</script>
