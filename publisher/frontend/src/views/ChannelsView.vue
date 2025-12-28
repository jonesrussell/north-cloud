<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Channels</h1>
      <p class="page-description">Manage Redis pub/sub channels for article distribution</p>
    </div>

    <div class="card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <div>
          <label class="form-checkbox">
            <input type="checkbox" v-model="enabledOnly" @change="loadChannels">
            Show enabled only
          </label>
        </div>
        <button class="btn btn-primary" @click="openCreateModal">
          + Add Channel
        </button>
      </div>

      <div v-if="loading" class="loading">Loading channels...</div>

      <div v-else-if="error" class="alert alert-error">{{ error }}</div>

      <table v-else class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Description</th>
            <th>Status</th>
            <th>Created</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="channel in channels" :key="channel.id">
            <td><code>{{ channel.name }}</code></td>
            <td>{{ channel.description }}</td>
            <td>
              <span :class="channel.enabled ? 'badge badge-success' : 'badge badge-danger'">
                {{ channel.enabled ? 'Enabled' : 'Disabled' }}
              </span>
            </td>
            <td>{{ formatDate(channel.created_at) }}</td>
            <td>
              <button class="btn btn-sm btn-primary" @click="openEditModal(channel)" style="margin-right: 0.5rem;">
                Edit
              </button>
              <button class="btn btn-sm btn-danger" @click="deleteChannel(channel)">
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="!loading && channels.length === 0" style="text-align: center; padding: 2rem; color: #666;">
        No channels found. Click "Add Channel" to create one.
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal">
        <div class="modal-header">
          <h2 class="modal-title">{{ isEditing ? 'Edit Channel' : 'Create Channel' }}</h2>
          <button class="modal-close" @click="closeModal">&times;</button>
        </div>

        <div v-if="modalError" class="alert alert-error">{{ modalError }}</div>

        <form @submit.prevent="saveChannel">
          <div class="form-group">
            <label class="form-label">Name *</label>
            <input
              type="text"
              class="form-input"
              v-model="formData.name"
              placeholder="e.g., articles:crime"
              required
            >
            <small style="color: #666;">Redis pub/sub channel name (e.g., articles:crime, articles:news)</small>
          </div>

          <div class="form-group">
            <label class="form-label">Description</label>
            <textarea
              class="form-textarea"
              v-model="formData.description"
              placeholder="Description of what content this channel contains"
              rows="3"
            ></textarea>
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
import { channelsAPI } from '../api/publisher'

const channels = ref([])
const loading = ref(false)
const error = ref(null)
const enabledOnly = ref(false)

const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref(null)
const saving = ref(false)
const formData = ref({
  name: '',
  description: '',
  enabled: true
})
const currentChannel = ref(null)

const loadChannels = async () => {
  loading.value = true
  error.value = null
  try {
    const response = await channelsAPI.list(enabledOnly.value)
    channels.value = response.data.channels || []
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to load channels'
  } finally {
    loading.value = false
  }
}

const openCreateModal = () => {
  isEditing.value = false
  formData.value = {
    name: '',
    description: '',
    enabled: true
  }
  currentChannel.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (channel) => {
  isEditing.value = true
  formData.value = {
    name: channel.name,
    description: channel.description,
    enabled: channel.enabled
  }
  currentChannel.value = channel
  modalError.value = null
  showModal.value = true
}

const closeModal = () => {
  showModal.value = false
  formData.value = { name: '', description: '', enabled: true }
  currentChannel.value = null
  modalError.value = null
}

const saveChannel = async () => {
  saving.value = true
  modalError.value = null
  try {
    if (isEditing.value) {
      await channelsAPI.update(currentChannel.value.id, formData.value)
    } else {
      await channelsAPI.create(formData.value)
    }
    closeModal()
    await loadChannels()
  } catch (err) {
    modalError.value = err.response?.data?.error || 'Failed to save channel'
  } finally {
    saving.value = false
  }
}

const deleteChannel = async (channel) => {
  if (!confirm(`Are you sure you want to delete channel "${channel.name}"?`)) {
    return
  }

  try {
    await channelsAPI.delete(channel.id)
    await loadChannels()
  } catch (err) {
    error.value = err.response?.data?.error || 'Failed to delete channel'
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString()
}

onMounted(() => {
  loadChannels()
})
</script>
