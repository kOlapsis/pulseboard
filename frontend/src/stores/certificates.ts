import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  listCertificates,
  type CertMonitor,
  type CertStatus,
} from '@/services/certificateApi'
import { sseBus } from '@/services/sseBus'

export const useCertificatesStore = defineStore('certificates', () => {
  const certificates = ref<CertMonitor[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const sseConnected = sseBus.connected
  const statusFilter = ref<CertStatus | ''>('')

  const statusCounts = computed(() => {
    const counts = { valid: 0, expiring: 0, expired: 0, error: 0, unknown: 0 }
    for (const cert of certificates.value) {
      if (cert.status in counts) {
        counts[cert.status as keyof typeof counts]++
      }
    }
    return counts
  })

  const filteredCertificates = computed(() => {
    if (!statusFilter.value) return certificates.value
    return certificates.value.filter((c) => c.status === statusFilter.value)
  })

  async function fetchCertificates() {
    loading.value = true
    error.value = null
    try {
      const res = await listCertificates()
      certificates.value = res.certificates || []
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch certificates'
    } finally {
      loading.value = false
    }
  }

  function onCreated() {
    fetchCertificates()
  }

  function onCheckCompleted(e: MessageEvent) {
    let data
    try {
      data = JSON.parse(e.data)
    } catch {
      return
    }
    const idx = certificates.value.findIndex((c) => c.id === data.monitor_id)
    if (idx >= 0) {
      certificates.value[idx] = {
        ...certificates.value[idx]!,
        status: data.status,
        last_check_at: data.checked_at,
      }
    } else {
      fetchCertificates()
    }
  }

  function onStatusChanged(e: MessageEvent) {
    let data
    try {
      data = JSON.parse(e.data)
    } catch {
      return
    }
    const idx = certificates.value.findIndex((c) => c.id === data.monitor_id)
    if (idx >= 0) {
      certificates.value[idx] = {
        ...certificates.value[idx]!,
        status: data.new_status,
      }
    } else {
      fetchCertificates()
    }
  }

  function onAlert() {
    fetchCertificates()
  }

  function onRecovery() {
    fetchCertificates()
  }

  function onDeleted(e: MessageEvent) {
    let data
    try {
      data = JSON.parse(e.data)
    } catch {
      return
    }
    certificates.value = certificates.value.filter((c) => c.id !== data.monitor_id)
  }

  function connectSSE() {
    sseBus.on('certificate.created', onCreated)
    sseBus.on('certificate.check_completed', onCheckCompleted)
    sseBus.on('certificate.status_changed', onStatusChanged)
    sseBus.on('certificate.alert', onAlert)
    sseBus.on('certificate.recovery', onRecovery)
    sseBus.on('certificate.deleted', onDeleted)
    sseBus.connect()
  }

  function disconnectSSE() {
    sseBus.off('certificate.created', onCreated)
    sseBus.off('certificate.check_completed', onCheckCompleted)
    sseBus.off('certificate.status_changed', onStatusChanged)
    sseBus.off('certificate.alert', onAlert)
    sseBus.off('certificate.recovery', onRecovery)
    sseBus.off('certificate.deleted', onDeleted)
    sseBus.disconnect()
  }

  return {
    certificates,
    loading,
    error,
    sseConnected,
    statusFilter,
    statusCounts,
    filteredCertificates,
    fetchCertificates,
    connectSSE,
    disconnectSSE,
  }
})
