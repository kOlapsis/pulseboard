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
import { onMounted, onUnmounted, computed } from 'vue'
import { usePostureStore } from '@/stores/posture'
import { useEdition } from '@/composables/useEdition'
import { timeAgo } from '@/utils/time'
import PostureScoreBadge from '@/components/PostureScoreBadge.vue'
import PostureContainerList from '@/components/PostureContainerList.vue'
import FeatureGate from '@/components/FeatureGate.vue'
import { ShieldCheck, AlertTriangle } from 'lucide-vue-next'

const { hasFeature } = useEdition()
const store = usePostureStore()

const isAvailable = computed(() => hasFeature('security_posture'))
const posture = computed(() => store.posture)

onMounted(() => {
  if (isAvailable.value) {
    store.fetchPosture()
    store.connectSSE()
  }
})

onUnmounted(() => {
  store.disconnectSSE()
})
</script>

<template>
  <div class="overflow-y-auto p-6">
    <div class="max-w-7xl mx-auto space-y-6 pb-12">

      <!-- Header -->
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-xl font-bold text-white">Security Posture</h1>
          <p class="text-xs text-slate-500 mt-0.5">
            Infrastructure-wide security scoring
          </p>
        </div>
        <div v-if="posture" class="flex items-center gap-3">
          <span v-if="posture.is_partial" class="flex items-center gap-1.5 text-[10px] text-amber-400 font-bold">
            <AlertTriangle :size="11" />
            Partial data
          </span>
          <span class="text-[10px] text-slate-500 font-bold">
            Updated {{ timeAgo(posture.computed_at) }}
          </span>
        </div>
      </div>

      <!-- Enterprise gate -->
      <FeatureGate
        feature="security_posture"
        title="Security Posture"
        description="View infrastructure-wide security scoring with per-container breakdown, category analysis, and risk acknowledgments."
      >

        <!-- Loading -->
        <template v-if="store.loading && !posture">
          <div class="space-y-4">
            <div class="h-32 animate-pulse rounded-xl bg-slate-800/50" />
            <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
              <div v-for="i in 5" :key="i" class="h-24 animate-pulse rounded-xl bg-slate-800/50" />
            </div>
          </div>
        </template>

        <!-- Posture dashboard -->
        <template v-else-if="posture">
          <!-- Score hero -->
          <div class="bg-[#12151C] rounded-2xl border border-slate-800 p-8 flex flex-col items-center gap-4">
            <PostureScoreBadge :score="posture.score" :color="posture.color" size="lg" label="Infrastructure Score" />
            <div class="flex items-center gap-4 text-[10px] text-slate-500 font-bold uppercase tracking-widest">
              <span>{{ posture.scored_count }}/{{ posture.container_count }} containers scored</span>
            </div>
          </div>

          <!-- Category summary -->
          <div v-if="posture.categories.length > 0">
            <h2 class="text-sm font-bold text-white mb-3">Category Summary</h2>
            <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3">
              <div
                v-for="cat in posture.categories"
                :key="cat.name"
                class="bg-[#12151C] rounded-xl p-4 border border-slate-800"
              >
                <div class="text-[10px] text-slate-500 font-bold uppercase tracking-widest mb-1">{{ cat.name.replace('_', ' ') }}</div>
                <p class="text-2xl font-black" :class="cat.total_issues > 0 ? 'text-amber-400' : 'text-slate-600'">
                  {{ cat.total_issues }}
                </p>
                <p class="text-[10px] text-slate-600 mt-0.5">{{ cat.summary }}</p>
              </div>
            </div>
          </div>

          <!-- Top risks -->
          <div v-if="posture.top_risks.length > 0">
            <h2 class="text-sm font-bold text-white mb-3">Top Risks</h2>
            <PostureContainerList :risks="posture.top_risks" />
          </div>
        </template>

        <!-- No data -->
        <div v-else class="flex flex-col items-center justify-center py-16">
          <ShieldCheck :size="40" class="text-slate-700 mb-3" />
          <p class="text-sm text-slate-600 font-medium">No posture data available</p>
          <p class="text-[10px] text-slate-700 mt-1">Make sure containers are being monitored</p>
        </div>

      </FeatureGate>
    </div>
  </div>
</template>
