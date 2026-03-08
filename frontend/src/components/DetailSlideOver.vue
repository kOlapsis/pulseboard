<!--
  Copyright 2026 Benjamin Touchard (kOlapsis)

  Licensed under the GNU Affero General Public License v3.0 (AGPL-3.0)
  or a commercial license. You may not use this file except in compliance
  with one of these licenses.

  AGPL-3.0: https://www.gnu.org/licenses/agpl-3.0.html
  Commercial: See LICENSE-COMMERCIAL.md

  Source: https://github.com/kolapsis/maintenant
-->

<script setup lang="ts">
import { computed, inject, watch } from 'vue'
import SlideOverPanel from './ui/SlideOverPanel.vue'
import ContainerDetail from './ContainerDetail.vue'
import HeartbeatDetail from './HeartbeatDetail.vue'
import CertificateDetail from './CertificateDetail.vue'
import { detailSlideOverKey, type EntityType } from '@/composables/useDetailSlideOver'
import { useContainersStore } from '@/stores/containers'
import { useHeartbeatsStore } from '@/stores/heartbeats'
import { useCertificatesStore } from '@/stores/certificates'

const detail = inject(detailSlideOverKey)!

const containersStore = useContainersStore()
const heartbeatsStore = useHeartbeatsStore()
const certificatesStore = useCertificatesStore()

// Ensure store data is loaded when opening a detail for an entity type
watch(
  () => [detail.entityType.value, detail.entityId.value] as const,
  ([type]) => {
    if (!type) return
    if (type === 'container' && containersStore.allContainers.length === 0) {
      containersStore.fetchContainers()
    } else if (type === 'heartbeat' && heartbeatsStore.heartbeats.length === 0) {
      heartbeatsStore.fetchHeartbeats()
    } else if (type === 'certificate' && certificatesStore.certificates.length === 0) {
      certificatesStore.fetchCertificates()
    }
  },
)

const panelOpen = computed({
  get: () => detail.isOpen.value,
  set: (val: boolean) => {
    if (!val) detail.close()
  },
})

const panelTitle = computed(() => {
  const type = detail.entityType.value
  const id = detail.entityId.value
  if (!type || !id) return ''
  return resolveTitle(type, id)
})

const panelWidth = computed(() => {
  return detail.entityType.value === 'container' ? 'max-w-2xl' : 'max-w-lg'
})

function resolveTitle(type: EntityType, id: number): string {
  switch (type) {
    case 'container': {
      const c = containersStore.allContainers.find(ct => ct.id === id)
      return c?.name ?? ''
    }
    case 'heartbeat': {
      const h = heartbeatsStore.heartbeats.find(hb => hb.id === id)
      return h?.name ?? ''
    }
    case 'certificate': {
      const cert = certificatesStore.certificates.find(c => c.id === id)
      return cert ? `${cert.hostname}:${cert.port}` : ''
    }
    case 'endpoint':
      return ''
  }
}

function handleClose() {
  detail.close()
}

function handleDeleted() {
  detail.close()
  containersStore.fetchContainers()
}
</script>

<template>
  <SlideOverPanel v-model:open="panelOpen" :title="panelTitle" :width="panelWidth">
    <template #header>
      <span></span>
    </template>
    <ContainerDetail
      v-if="detail.entityType.value === 'container' && detail.entityId.value"
      :container-id="detail.entityId.value"
      @close="handleClose"
      @deleted="handleDeleted"
    />
    <HeartbeatDetail
      v-if="detail.entityType.value === 'heartbeat' && detail.entityId.value"
      :heartbeat-id="detail.entityId.value"
      @close="handleClose"
    />
    <CertificateDetail
      v-if="detail.entityType.value === 'certificate' && detail.entityId.value"
      :certificate-id="detail.entityId.value"
      @close="handleClose"
    />
  </SlideOverPanel>
</template>
