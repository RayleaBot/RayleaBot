import { notifyError, notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'

// Copy failures (denied permission, insecure context) fall back to asking for a manual copy.
export async function copyText(text: string, successMessage: string, failureMessage = t('ui.clipboard.copyFailed')) {
  try {
    await navigator.clipboard.writeText(text)
    notifySuccess(successMessage)
  } catch {
    notifyError(failureMessage)
  }
}
