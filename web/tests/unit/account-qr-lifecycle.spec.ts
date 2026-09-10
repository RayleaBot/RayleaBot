import { defineComponent } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { useAccountQRLogins } from '@/views/builtin/useAccountQRLogins'
import { useThirdPartyAccountsStore } from '@/stores/third-party-accounts'
import type { ThirdPartyQRCodeLoginCreateResponse } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({ notifyError: vi.fn() }))

function session(loginId: string): ThirdPartyQRCodeLoginCreateResponse {
  return { platform: 'douyin', login_id: loginId, qrcode_url: 'https://example.test/qr', expires_at: '2099-01-01T00:00:00Z', state: 'pending_scan' }
}

function harness() {
  const store = useThirdPartyAccountsStore()
  const cancel = vi.spyOn(store, 'cancelQRCodeLogin').mockResolvedValue()
  const poll = vi.spyOn(store, 'pollQRCodeLogin')
  let qr!: ReturnType<typeof useAccountQRLogins>
  const view = mount(defineComponent({ setup() { qr = useAccountQRLogins(key => key); return () => null } }))
  return { store, cancel, poll, view, qr }
}

beforeEach(() => { setActivePinia(createPinia()); vi.useFakeTimers() })
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

it('retires a remote session created after the page was closed', async () => {
  const { store, qr, view, cancel, poll } = harness()
  const pending = Promise.withResolvers<ThirdPartyQRCodeLoginCreateResponse>()
  vi.spyOn(store, 'createQRCodeLogin').mockReturnValue(pending.promise)
  const starting = qr.start('draft', 'douyin')
  await flushPromises()
  view.unmount()
  pending.resolve(session('late-session'))
  await starting
  await vi.advanceTimersByTimeAsync(4000)
  expect(cancel).toHaveBeenCalledWith('douyin', 'late-session', true)
  expect(Object.keys(qr.qrLogins)).toEqual([])
  expect(poll).not.toHaveBeenCalled()
})

it('keeps the replacement when an older create finishes later', async () => {
  const { store, qr, view, cancel } = harness()
  const pending = Promise.withResolvers<ThirdPartyQRCodeLoginCreateResponse>()
  vi.spyOn(store, 'createQRCodeLogin').mockReturnValueOnce(pending.promise).mockResolvedValueOnce(session('replacement'))
  const old = qr.start('draft', 'douyin')
  await flushPromises()
  await qr.start('draft', 'douyin')
  pending.resolve(session('retired'))
  await old
  expect(qr.qrLogins.draft.loginId).toBe('replacement')
  expect(cancel).toHaveBeenCalledWith('douyin', 'retired', true)
  view.unmount()
  await flushPromises()
  expect(cancel).toHaveBeenCalledWith('douyin', 'replacement', true)
})

it('does not restart an older request after its cancellation finishes', async () => {
  const { store, qr, view, cancel } = harness()
  const create = vi.spyOn(store, 'createQRCodeLogin').mockResolvedValueOnce(session('initial')).mockResolvedValueOnce(session('latest'))
  await qr.start('draft', 'douyin')
  const pendingCancel = Promise.withResolvers<void>()
  cancel.mockReturnValueOnce(pendingCancel.promise)
  const older = qr.start('draft', 'douyin')
  await qr.start('draft', 'douyin')
  pendingCancel.resolve()
  await older
  expect(create).toHaveBeenCalledTimes(2)
  expect(qr.qrLogins.draft.loginId).toBe('latest')
  view.unmount()
})
