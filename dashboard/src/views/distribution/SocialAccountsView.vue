<script setup lang="ts">
import { ref } from 'vue'
import { Loader2, Users, Plus } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import AccountsTable from '@/components/domain/social-publishing/AccountsTable.vue'
import AccountFormDialog from '@/components/domain/social-publishing/AccountFormDialog.vue'
import { useAccountsTable } from '@/features/social-publishing'
import { socialPublisherApi } from '@/api/client'
import type { SocialAccount, CreateAccountRequest, UpdateAccountRequest } from '@/types/socialPublisher'

const accountsTable = useAccountsTable()
const dialogOpen = ref(false)
const editingAccount = ref<SocialAccount | null>(null)
const saving = ref(false)
const deleting = ref<string | null>(null)

function openCreate() {
  editingAccount.value = null
  dialogOpen.value = true
}

function openEdit(account: SocialAccount) {
  editingAccount.value = account
  dialogOpen.value = true
}

async function handleSave(data: CreateAccountRequest | UpdateAccountRequest) {
  saving.value = true
  try {
    if (editingAccount.value) {
      await socialPublisherApi.accounts.update(editingAccount.value.id, data as UpdateAccountRequest)
      toast.success('Account updated')
    } else {
      await socialPublisherApi.accounts.create(data as CreateAccountRequest)
      toast.success('Account created')
    }
    dialogOpen.value = false
    accountsTable.refetch()
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Failed to save account'
    toast.error(message)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: string) {
  if (!confirm('Are you sure you want to delete this account?')) return
  deleting.value = id
  try {
    await socialPublisherApi.accounts.delete(id)
    toast.success('Account deleted')
    accountsTable.refetch()
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Failed to delete account'
    toast.error(message)
  } finally {
    deleting.value = null
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Social Accounts
        </h1>
        <p class="text-muted-foreground">
          Manage social media accounts for publishing
        </p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Add Account
      </Button>
    </div>

    <div
      v-if="accountsTable.isLoading.value && accountsTable.items.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="accountsTable.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ accountsTable.error.value?.message || 'Unable to load accounts.' }}
        </p>
      </CardContent>
    </Card>

    <Card v-else-if="accountsTable.items.value.length === 0 && !accountsTable.hasActiveFilters.value">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Users class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No social accounts configured
        </h3>
        <p class="text-muted-foreground mb-4">
          Add your first social media account to start publishing.
        </p>
        <Button @click="openCreate">
          <Plus class="mr-2 h-4 w-4" />
          Add Account
        </Button>
      </CardContent>
    </Card>

    <Card v-else>
      <CardContent class="p-0">
        <AccountsTable
          :items="accountsTable.items.value"
          :total="accountsTable.total.value"
          :is-loading="accountsTable.isLoading.value"
          :page="accountsTable.page.value"
          :page-size="accountsTable.pageSize.value"
          :total-pages="accountsTable.totalPages.value"
          :allowed-page-sizes="accountsTable.allowedPageSizes"
          :sort-by="accountsTable.sortBy.value"
          :sort-order="accountsTable.sortOrder.value"
          :has-active-filters="accountsTable.hasActiveFilters.value"
          :deleting-id="deleting"
          :on-sort="accountsTable.toggleSort"
          :on-page-change="accountsTable.setPage"
          :on-page-size-change="accountsTable.setPageSize"
          :on-clear-filters="accountsTable.clearFilters"
          :on-edit="openEdit"
          :on-delete="handleDelete"
        />
      </CardContent>
    </Card>

    <AccountFormDialog
      :open="dialogOpen"
      :account="editingAccount"
      :saving="saving"
      @close="dialogOpen = false"
      @save="handleSave"
    />
  </div>
</template>
