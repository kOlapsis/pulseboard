<script setup lang="ts">
import type { CertStatus } from '@/services/certificateApi'

defineProps<{
  status: CertStatus
}>()

const statusColors: Record<CertStatus, string> = {
  valid: 'bg-green-100 text-green-800',
  expiring: 'bg-yellow-100 text-yellow-800',
  expired: 'bg-red-100 text-red-800',
  error: 'bg-orange-100 text-orange-800',
  unknown: 'bg-gray-100 text-slate-600',
}
</script>

<template>
  <span
    class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
    :class="statusColors[status] || 'bg-gray-100 text-slate-600'"
  >
    {{ status }}
  </span>
</template>
