<script setup lang="ts">
import type { SourceMetadata } from '../types'

defineProps<{
  metadata: SourceMetadata
}>()
</script>

<template>
  <div class="bg-slate-800/50 border border-slate-700 rounded-lg p-4 text-sm">
    <h4 class="text-xs font-medium text-slate-400 uppercase mb-2">Detected Metadata</h4>
    <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
      <template v-if="metadata.title">
        <dt class="text-slate-400">Title</dt>
        <dd class="text-slate-200">{{ metadata.title }}</dd>
      </template>
      <template v-if="metadata.description">
        <dt class="text-slate-400">Description</dt>
        <dd class="text-slate-200 line-clamp-2">{{ metadata.description }}</dd>
      </template>
      <template v-if="metadata.feed_url">
        <dt class="text-slate-400">Feed</dt>
        <dd class="text-slate-200 truncate">{{ metadata.feed_url }}</dd>
      </template>
      <template v-if="metadata.sitemap_url">
        <dt class="text-slate-400">Sitemap</dt>
        <dd class="text-slate-200 truncate">{{ metadata.sitemap_url }}</dd>
      </template>
    </dl>
  </div>
</template>
