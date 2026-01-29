<template>
  <div>
    <PageHeader
      title="Classification Rules"
      subtitle="Manage rules for topic classification"
    >
      <template #actions>
        <button
          class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
          @click="showCreateModal = true"
        >
          <PlusIcon class="h-5 w-5 mr-2" />
          New Rule
        </button>
      </template>
    </PageHeader>

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading rules..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Rules Content -->
    <div v-else>
      <!-- Rules Table -->
      <div class="bg-white shadow rounded-lg overflow-hidden">
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Topic
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Keywords
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Pattern
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Priority
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Enabled
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr v-if="rules.length === 0">
              <td
                colspan="6"
                class="px-6 py-8 text-center text-sm text-gray-500"
              >
                No rules found. Create your first rule to get started.
              </td>
            </tr>
            <tr
              v-for="rule in rules"
              :key="rule.id"
              class="hover:bg-gray-50"
            >
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm font-medium text-gray-900 capitalize">{{ rule.topic }}</span>
              </td>
              <td class="px-6 py-4">
                <div class="text-sm text-gray-500">
                  <span v-if="rule.keywords && rule.keywords.length > 0">
                    {{ rule.keywords.slice(0, 3).join(', ') }}
                    <span
                      v-if="rule.keywords.length > 3"
                      class="text-gray-400"
                    >
                      +{{ rule.keywords.length - 3 }} more
                    </span>
                  </span>
                  <span
                    v-else
                    class="text-gray-400"
                  >None</span>
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm text-gray-500">{{ rule.pattern || 'N/A' }}</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span
                  class="px-2 py-1 text-xs font-semibold rounded"
                  :class="getPriorityClass(rule.priority)"
                >
                  {{ rule.priority || 'normal' }}
                </span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <StatusBadge :status="rule.enabled ? 'active' : 'inactive'" />
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                <button
                  class="text-green-600 hover:text-green-900 mr-4"
                  @click="testRule(rule)"
                >
                  Test
                </button>
                <button
                  class="text-blue-600 hover:text-blue-900 mr-4"
                  @click="editRule(rule)"
                >
                  Edit
                </button>
                <button
                  class="text-red-600 hover:text-red-900"
                  @click="deleteRule(rule)"
                >
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div
      v-if="showCreateModal || editingRule"
      class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50"
      @click.self="closeModal"
    >
      <div class="relative top-20 mx-auto p-5 border w-96 shadow-lg rounded-md bg-white">
        <h3 class="text-lg font-medium text-gray-900 mb-4">
          {{ editingRule ? 'Edit Rule' : 'Create New Rule' }}
        </h3>
        <form
          class="space-y-4"
          @submit.prevent="saveRule"
        >
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Topic</label>
            <input
              v-model="ruleForm.topic"
              type="text"
              required
              class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              placeholder="e.g., crime, sports, politics"
            >
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Keywords (comma-separated)</label>
            <input
              v-model="ruleForm.keywords"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              placeholder="murder, robbery, assault"
            >
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Pattern (regex)</label>
            <input
              v-model="ruleForm.pattern"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              placeholder="Optional regex pattern"
            >
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Priority</label>
            <select
              v-model="ruleForm.priority"
              class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
            >
              <option value="high">
                High
              </option>
              <option value="normal">
                Normal
              </option>
              <option value="low">
                Low
              </option>
            </select>
          </div>
          <div class="flex items-center">
            <input
              v-model="ruleForm.enabled"
              type="checkbox"
              class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
            >
            <label class="ml-2 block text-sm text-gray-900">Enabled</label>
          </div>
          <div class="flex justify-end space-x-3 pt-4">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
              @click="closeModal"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700"
            >
              {{ editingRule ? 'Update' : 'Create' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Test Rule Modal -->
    <div
      v-if="testingRule"
      class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50"
      @click.self="closeTestModal"
    >
      <div class="relative top-10 mx-auto p-5 border w-[600px] shadow-lg rounded-md bg-white">
        <h3 class="text-lg font-medium text-gray-900 mb-4">
          Test Rule: {{ testingRule.topic }}
        </h3>
        <form
          class="space-y-4"
          @submit.prevent="runTest"
        >
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Title (optional)</label>
            <input
              v-model="testForm.title"
              type="text"
              class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              placeholder="Enter article title to test..."
            >
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Body Content</label>
            <textarea
              v-model="testForm.body"
              required
              rows="6"
              class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              placeholder="Enter article body text to test against the rule keywords..."
            />
          </div>
          <div class="flex justify-between items-center pt-2">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-50"
              @click="closeTestModal"
            >
              Close
            </button>
            <button
              type="submit"
              :disabled="testLoading"
              class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-green-600 hover:bg-green-700 disabled:opacity-50"
            >
              {{ testLoading ? 'Testing...' : 'Run Test' }}
            </button>
          </div>
        </form>

        <!-- Test Results -->
        <div
          v-if="testResult"
          class="mt-6 pt-4 border-t"
        >
          <h4 class="text-md font-medium text-gray-900 mb-3">
            Test Results
          </h4>
          <div
            class="p-4 rounded-md"
            :class="testResult.matched ? 'bg-green-50' : 'bg-red-50'"
          >
            <div class="flex items-center mb-2">
              <span
                class="px-2 py-1 text-sm font-semibold rounded"
                :class="testResult.matched ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'"
              >
                {{ testResult.matched ? 'MATCHED' : 'NOT MATCHED' }}
              </span>
            </div>
            <div class="grid grid-cols-2 gap-4 mt-3 text-sm">
              <div>
                <span class="text-gray-600">Score:</span>
                <span class="ml-2 font-medium">{{ (testResult.score * 100).toFixed(1) }}%</span>
              </div>
              <div>
                <span class="text-gray-600">Coverage:</span>
                <span class="ml-2 font-medium">{{ (testResult.coverage * 100).toFixed(1) }}%</span>
              </div>
              <div>
                <span class="text-gray-600">Match Count:</span>
                <span class="ml-2 font-medium">{{ testResult.match_count }}</span>
              </div>
              <div>
                <span class="text-gray-600">Unique Matches:</span>
                <span class="ml-2 font-medium">{{ testResult.unique_matches }}</span>
              </div>
            </div>
            <div
              v-if="testResult.matched_keywords && testResult.matched_keywords.length > 0"
              class="mt-3"
            >
              <span class="text-gray-600 text-sm">Matched Keywords:</span>
              <div class="mt-1 flex flex-wrap gap-1">
                <span
                  v-for="kw in testResult.matched_keywords"
                  :key="kw"
                  class="px-2 py-1 text-xs bg-blue-100 text-blue-800 rounded"
                >
                  {{ kw }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <ConfirmModal
      v-if="ruleToDelete"
      :show="!!ruleToDelete"
      title="Delete Rule"
      :message="`Are you sure you want to delete the rule for topic '${ruleToDelete.topic}'?`"
      confirm-text="Delete"
      cancel-text="Cancel"
      @confirm="confirmDelete"
      @cancel="ruleToDelete = null"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { PlusIcon } from '@heroicons/vue/24/outline'
import { classifierApi } from '../../api/client'
import {
  PageHeader,
  LoadingSpinner,
  ErrorAlert,
  StatusBadge,
  ConfirmModal,
} from '../../components/common'

const loading = ref(true)
const error = ref(null)
const rules = ref([])
const showCreateModal = ref(false)
const editingRule = ref(null)
const ruleToDelete = ref(null)

// Test rule state
const testingRule = ref(null)
const testLoading = ref(false)
const testResult = ref(null)
const testForm = ref({
  title: '',
  body: '',
})

const ruleForm = ref({
  topic: '',
  keywords: '',
  pattern: '',
  priority: 'normal',
  enabled: true,
})

const getPriorityClass = (priority) => {
  switch (priority) {
    case 'high':
      return 'bg-red-100 text-red-800'
    case 'normal':
      return 'bg-yellow-100 text-yellow-800'
    case 'low':
      return 'bg-gray-100 text-gray-800'
    default:
      return 'bg-gray-100 text-gray-800'
  }
}

const loadRules = async () => {
  try {
    loading.value = true
    error.value = null

    const res = await classifierApi.rules.list()
    rules.value = res.data?.rules || res.data || []
  } catch (err) {
    error.value = 'Unable to load rules. Classifier API may not be available yet.'
    console.error('[ClassifierRulesView] Error loading rules:', err)
  } finally {
    loading.value = false
  }
}

const editRule = (rule) => {
  editingRule.value = rule
  ruleForm.value = {
    topic: rule.topic || '',
    keywords: (rule.keywords || []).join(', '),
    pattern: rule.pattern || '',
    priority: rule.priority || 'normal',
    enabled: rule.enabled !== false,
  }
}

const closeModal = () => {
  showCreateModal.value = false
  editingRule.value = null
  ruleForm.value = {
    topic: '',
    keywords: '',
    pattern: '',
    priority: 'normal',
    enabled: true,
  }
}

const saveRule = async () => {
  try {
    const keywords = ruleForm.value.keywords
      .split(',')
      .map((k) => k.trim())
      .filter((k) => k.length > 0)

    const ruleData = {
      topic: ruleForm.value.topic,
      keywords,
      pattern: ruleForm.value.pattern || undefined,
      priority: ruleForm.value.priority,
      enabled: ruleForm.value.enabled,
    }

    if (editingRule.value) {
      await classifierApi.rules.update(editingRule.value.id, ruleData)
    } else {
      await classifierApi.rules.create(ruleData)
    }

    closeModal()
    await loadRules()
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to save rule'
    console.error('[ClassifierRulesView] Error saving rule:', err)
  }
}

const deleteRule = (rule) => {
  ruleToDelete.value = rule
}

const confirmDelete = async () => {
  if (!ruleToDelete.value) return

  try {
    await classifierApi.rules.delete(ruleToDelete.value.id)
    ruleToDelete.value = null
    await loadRules()
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to delete rule'
    console.error('[ClassifierRulesView] Error deleting rule:', err)
  }
}

// Test rule functions
const testRule = (rule) => {
  testingRule.value = rule
  testResult.value = null
  testForm.value = {
    title: '',
    body: '',
  }
}

const closeTestModal = () => {
  testingRule.value = null
  testResult.value = null
  testForm.value = {
    title: '',
    body: '',
  }
}

const runTest = async () => {
  if (!testingRule.value || !testForm.value.body) return

  try {
    testLoading.value = true
    testResult.value = null

    const res = await classifierApi.rules.test(testingRule.value.id, {
      title: testForm.value.title || '',
      body: testForm.value.body,
    })
    testResult.value = res.data
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to test rule'
    console.error('[ClassifierRulesView] Error testing rule:', err)
  } finally {
    testLoading.value = false
  }
}

onMounted(() => {
  loadRules()
})
</script>

