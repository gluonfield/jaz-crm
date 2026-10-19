import { toast } from 'sonner'

export type CopyAction = 'copyId' | 'copyUrl'

export async function copyRecord(id: string, action: CopyAction) {
  const value = action === 'copyId' ? id : `https://crm.jaz.chat/r/${encodeURIComponent(id)}`
  try {
    await navigator.clipboard.writeText(value)
    toast.success('Copied')
  } catch {
    toast.error('Unable to copy to clipboard')
  }
}
