// Copyright 2026 Benjamin Touchard (Kolapsis)
//
// Licensed under the GNU Affero General Public License v3.0 (AGPL-3.0)
// or a commercial license. You may not use this file except in compliance
// with one of these licenses.
//
// AGPL-3.0: https://www.gnu.org/licenses/agpl-3.0.html
// Commercial: See LICENSE-COMMERCIAL.md
//
// Source: https://github.com/kolapsis/maintenant

import { ref, readonly, watch, type InjectionKey, type Ref, type DeepReadonly } from 'vue'
import { useRoute, useRouter } from 'vue-router'

export type EntityType = 'container' | 'heartbeat' | 'certificate' | 'endpoint'

const VALID_ENTITY_TYPES: ReadonlySet<string> = new Set<EntityType>([
  'container',
  'heartbeat',
  'certificate',
  'endpoint',
])

export interface DetailSlideOver {
  isOpen: DeepReadonly<Ref<boolean>>
  entityType: DeepReadonly<Ref<EntityType | null>>
  entityId: DeepReadonly<Ref<number | null>>
  openDetail: (type: EntityType, id: number) => void
  close: () => void
}

export const detailSlideOverKey: InjectionKey<DetailSlideOver> = Symbol('detailSlideOver')

export function parseSelectedParam(value: unknown): { type: EntityType; id: number } | null {
  if (typeof value !== 'string') return null
  const dashIdx = value.indexOf('-')
  if (dashIdx < 1) return null
  const type = value.slice(0, dashIdx)
  const idStr = value.slice(dashIdx + 1)
  if (!VALID_ENTITY_TYPES.has(type)) return null
  const id = Number(idStr)
  if (!Number.isFinite(id) || id <= 0 || Math.floor(id) !== id) return null
  return { type: type as EntityType, id }
}

export function useDetailSlideOver(): DetailSlideOver {
  const route = useRoute()
  const router = useRouter()

  const isOpen = ref(false)
  const entityType = ref<EntityType | null>(null)
  const entityId = ref<number | null>(null)

  let updatingUrl = false

  function openDetail(type: EntityType, id: number) {
    entityType.value = type
    entityId.value = id
    isOpen.value = true
    syncToUrl(type, id)
  }

  function close() {
    isOpen.value = false
    entityType.value = null
    entityId.value = null
    removeFromUrl()
  }

  function syncToUrl(type: EntityType, id: number) {
    updatingUrl = true
    router.replace({
      query: { ...route.query, selected: `${type}-${id}` },
    }).finally(() => {
      updatingUrl = false
    })
  }

  function removeFromUrl() {
    if (!route.query.selected) return
    updatingUrl = true
    const { selected: _, ...rest } = route.query
    router.replace({ query: rest }).finally(() => {
      updatingUrl = false
    })
  }

  // Close slide-over on route path change (navigation via sidebar)
  watch(
    () => route.path,
    () => {
      if (isOpen.value) {
        isOpen.value = false
        entityType.value = null
        entityId.value = null
      }
    },
  )

  return {
    isOpen: readonly(isOpen),
    entityType: readonly(entityType),
    entityId: readonly(entityId),
    openDetail,
    close,
  }
}
