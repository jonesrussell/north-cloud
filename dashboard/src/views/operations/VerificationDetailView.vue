<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { CheckCircle, XCircle, ArrowLeft, ExternalLink } from 'lucide-vue-next'
import { verificationApi, type PendingItem, type EntityType } from '@/api/verification'
import { useToast } from '@/composables/useToast'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { LoadingSpinner, ErrorAlert } from '@/components/common'
import { formatDate } from '@/lib/utils'

const props = defineProps<{
  type: string
  id: string
}>()

const router = useRouter()
const { toast } = useToast()

const loading = ref(true)
const error = ref<string | null>(null)
const item = ref<PendingItem | null>(null)
const actionPending = ref(false)

const entityType = computed(() => props.type as EntityType)
const person = computed(() => item.value?.person)
const office = computed(() => item.value?.band_office)

const confidence = computed(() => {
  const val = props.type === 'person' ? person.value?.verification_confidence : office.value?.verification_confidence
  return val
})

const confidenceClass = computed(() => {
  if (confidence.value === undefined) return 'text-gray-400'
  if (confidence.value >= 0.9) return 'text-green-600 font-bold text-xl'
  if (confidence.value >= 0.5) return 'text-yellow-600 font-bold text-xl'
  return 'text-red-600 font-bold text-xl'
})

const issues = computed(() => {
  return props.type === 'person' ? person.value?.verification_issues : office.value?.verification_issues
})

async function loadItem() {
  loading.value = true
  error.value = null
  try {
    const resp = await verificationApi.listPending({ type: props.type, limit: 200 })
    const found = resp.items.find((i) => {
      const id = i.type === 'person' ? i.person?.id : i.band_office?.id
      return id === props.id
    })
    if (!found) {
      error.value = 'Record not found — it may have already been verified or rejected.'
      return
    }
    item.value = found
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Failed to load record'
  } finally {
    loading.value = false
  }
}

async function verify() {
  actionPending.value = true
  try {
    await verificationApi.verify(props.id, entityType.value)
    toast({ title: 'Verified', description: 'Record has been approved.' })
    router.push({ name: 'operations-verification' })
  } catch (e: unknown) {
    toast({ title: 'Error', description: e instanceof Error ? e.message : 'Verify failed', variant: 'destructive' })
  } finally {
    actionPending.value = false
  }
}

async function reject() {
  actionPending.value = true
  try {
    await verificationApi.reject(props.id, entityType.value)
    toast({ title: 'Rejected', description: 'Record has been removed.' })
    router.push({ name: 'operations-verification' })
  } catch (e: unknown) {
    toast({ title: 'Error', description: e instanceof Error ? e.message : 'Reject failed', variant: 'destructive' })
  } finally {
    actionPending.value = false
  }
}

onMounted(loadItem)
</script>

<template>
  <div class="p-6 space-y-6 max-w-3xl">
    <div class="flex items-center gap-3">
      <Button
        variant="ghost"
        size="sm"
        @click="router.push({ name: 'operations-verification' })"
      >
        <ArrowLeft class="h-4 w-4 mr-1" />
        Back to Queue
      </Button>
    </div>

    <LoadingSpinner v-if="loading" />
    <ErrorAlert
      v-else-if="error"
      :message="error"
    />

    <template v-else-if="item">
      <!-- Header card -->
      <Card>
        <CardHeader>
          <div class="flex items-start justify-between">
            <div>
              <Badge
                :variant="props.type === 'person' ? 'default' : 'secondary'"
                class="mb-2"
              >
                {{ props.type === 'person' ? 'Person' : 'Band Office' }}
              </Badge>
              <CardTitle class="text-xl">
                {{ props.type === 'person' ? person?.name : (office?.city ?? 'Band Office') }}
              </CardTitle>
              <p
                v-if="props.type === 'person' && person?.role"
                class="text-gray-500 mt-1"
              >
                {{ person?.role }}
              </p>
            </div>
            <div class="text-right">
              <div class="text-sm text-gray-500 mb-1">
                AI Confidence
              </div>
              <div :class="confidenceClass">
                {{ confidence !== undefined ? (confidence * 100).toFixed(0) + '%' : 'Unscored' }}
              </div>
            </div>
          </div>
        </CardHeader>

        <CardContent class="space-y-4">
          <!-- Issues -->
          <div
            v-if="issues"
            class="bg-yellow-50 border border-yellow-200 rounded p-3"
          >
            <div class="text-sm font-medium text-yellow-800 mb-1">
              AI Issues
            </div>
            <div class="text-sm text-yellow-700">
              {{ issues }}
            </div>
          </div>

          <!-- Person fields -->
          <template v-if="props.type === 'person' && person">
            <dl class="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
              <div>
                <dt class="text-gray-500">
                  Community ID
                </dt>
                <dd class="font-mono text-xs">
                  {{ person.community_id }}
                </dd>
              </div>
              <div>
                <dt class="text-gray-500">
                  Data Source
                </dt>
                <dd>{{ person.data_source }}</dd>
              </div>
              <div v-if="person.email">
                <dt class="text-gray-500">
                  Email
                </dt>
                <dd>{{ person.email }}</dd>
              </div>
              <div v-if="person.phone">
                <dt class="text-gray-500">
                  Phone
                </dt>
                <dd>{{ person.phone }}</dd>
              </div>
              <div>
                <dt class="text-gray-500">
                  Current
                </dt>
                <dd>{{ person.is_current ? 'Yes' : 'No' }}</dd>
              </div>
              <div>
                <dt class="text-gray-500">
                  Added
                </dt>
                <dd>{{ formatDate(person.created_at) }}</dd>
              </div>
            </dl>
          </template>

          <!-- Band office fields -->
          <template v-if="props.type === 'band_office' && office">
            <dl class="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
              <div>
                <dt class="text-gray-500">
                  Community ID
                </dt>
                <dd class="font-mono text-xs">
                  {{ office.community_id }}
                </dd>
              </div>
              <div>
                <dt class="text-gray-500">
                  Data Source
                </dt>
                <dd>{{ office.data_source }}</dd>
              </div>
              <div v-if="office.address_line1">
                <dt class="text-gray-500">
                  Address
                </dt>
                <dd>
                  {{ office.address_line1 }}
                  <span v-if="office.address_line2">, {{ office.address_line2 }}</span>
                </dd>
              </div>
              <div v-if="office.city">
                <dt class="text-gray-500">
                  City / Province
                </dt>
                <dd>{{ office.city }}<span v-if="office.province">, {{ office.province }}</span></dd>
              </div>
              <div v-if="office.postal_code">
                <dt class="text-gray-500">
                  Postal Code
                </dt>
                <dd>{{ office.postal_code }}</dd>
              </div>
              <div v-if="office.phone">
                <dt class="text-gray-500">
                  Phone
                </dt>
                <dd>{{ office.phone }}</dd>
              </div>
              <div v-if="office.fax">
                <dt class="text-gray-500">
                  Fax
                </dt>
                <dd>{{ office.fax }}</dd>
              </div>
              <div v-if="office.email">
                <dt class="text-gray-500">
                  Email
                </dt>
                <dd>{{ office.email }}</dd>
              </div>
              <div v-if="office.toll_free">
                <dt class="text-gray-500">
                  Toll Free
                </dt>
                <dd>{{ office.toll_free }}</dd>
              </div>
              <div v-if="office.office_hours">
                <dt class="text-gray-500">
                  Office Hours
                </dt>
                <dd>{{ office.office_hours }}</dd>
              </div>
              <div>
                <dt class="text-gray-500">
                  Added
                </dt>
                <dd>{{ formatDate(office.created_at) }}</dd>
              </div>
            </dl>
          </template>

          <!-- Source URL -->
          <div
            v-if="(props.type === 'person' ? person?.source_url : office?.source_url)"
            class="pt-2"
          >
            <a
              :href="props.type === 'person' ? person?.source_url : office?.source_url"
              target="_blank"
              rel="noopener noreferrer"
              class="text-sm text-blue-600 hover:underline inline-flex items-center gap-1"
            >
              <ExternalLink class="h-4 w-4" />
              View source page
            </a>
          </div>
        </CardContent>
      </Card>

      <!-- Actions -->
      <div class="flex items-center gap-4">
        <Button
          class="bg-green-600 hover:bg-green-700 text-white"
          :disabled="actionPending"
          @click="verify"
        >
          <CheckCircle class="h-4 w-4 mr-2" />
          Approve
        </Button>
        <Button
          variant="outline"
          class="text-red-700 border-red-300 hover:bg-red-50"
          :disabled="actionPending"
          @click="reject"
        >
          <XCircle class="h-4 w-4 mr-2" />
          Reject
        </Button>
      </div>
    </template>
  </div>
</template>
