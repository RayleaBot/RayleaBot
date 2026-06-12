import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { computed, defineComponent, h, nextTick, ref } from 'vue'

import { useDashboardRefresh } from '@/views/dashboard/useDashboardRefresh'

describe('dashboard state sync', () => {
  it('loads the dashboard snapshots when the page mounts', async () => {
    const refreshAll = vi.fn().mockResolvedValue(undefined)
    const refreshProtocols = vi.fn().mockResolvedValue(undefined)

    const Harness = defineComponent({
      setup() {
        useDashboardRefresh({
          protocolsStore: {
            refresh: refreshProtocols,
          },
          recoveryConfirmNote: ref(''),
          recoverySummary: computed(() => null),
          selectedRecoveryReviewIds: ref<string[]>([]),
          systemStore: {
            refreshAll,
          },
        })

        return () => h('div')
      },
    })

    mount(Harness)
    await Promise.resolve()

    expect(refreshAll).toHaveBeenCalledTimes(1)
    expect(refreshProtocols).toHaveBeenCalledTimes(1)
  })

  it('keeps selected recovery review ids aligned with the current summary', async () => {
    const selectedRecoveryReviewIds = ref(['review_pending', 'review_confirmed'])
    const recoveryConfirmNote = ref('确认备注')
    const recoverySummary = ref<any>({
      skipped_plugins: [],
    })

    const Harness = defineComponent({
      setup() {
        useDashboardRefresh({
          protocolsStore: {
            refresh: vi.fn().mockResolvedValue(undefined),
          },
          recoveryConfirmNote,
          recoverySummary: computed(() => recoverySummary.value),
          selectedRecoveryReviewIds,
          systemStore: {
            refreshAll: vi.fn().mockResolvedValue(undefined),
          },
        })

        return () => h('div')
      },
    })

    mount(Harness)
    recoverySummary.value = {
      skipped_plugins: [
        { review_id: 'review_pending', review_status: 'pending' },
        { review_id: 'review_confirmed', review_status: 'confirmed' },
      ],
    }
    await nextTick()

    expect(selectedRecoveryReviewIds.value).toEqual(['review_pending'])
    expect(recoveryConfirmNote.value).toBe('确认备注')

    recoverySummary.value = {
      skipped_plugins: [
        { review_id: 'review_pending', review_status: 'confirmed' },
      ],
    }
    await nextTick()

    expect(selectedRecoveryReviewIds.value).toEqual([])
    expect(recoveryConfirmNote.value).toBe('')
  })
})
