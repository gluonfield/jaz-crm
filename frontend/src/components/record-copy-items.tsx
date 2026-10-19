import { Copy, Link2 } from 'lucide-react'
import { copyRecord } from '@/lib/record-copy'
import { shortcuts } from '@/lib/shortcut-definitions'
import { shortcutLabel } from '@/lib/shortcuts'
import { ContextMenuItem, ContextMenuShortcut } from './ui/context-menu'
import { DropdownMenuItem, DropdownMenuShortcut } from './ui/dropdown-menu'

export function RecordCopyItems({ id, context = false }: { id: string; context?: boolean }) {
  const Item = context ? ContextMenuItem : DropdownMenuItem
  const Hint = context ? ContextMenuShortcut : DropdownMenuShortcut
  return <>
    <Item onSelect={() => void copyRecord(id, 'copyId')}><Copy />Copy ID<Hint>{shortcutLabel(shortcuts.copyId.keys[0])}</Hint></Item>
    <Item onSelect={() => void copyRecord(id, 'copyUrl')}><Link2 />Copy link<Hint>{shortcutLabel(shortcuts.copyUrl.keys[0])}</Hint></Item>
  </>
}
