import { shortcuts, shortcutIds, editorShortcuts } from '@/lib/shortcut-definitions'
import { shortcutLabel } from '@/lib/shortcuts'
import { setUI, useUI } from '@/lib/ui'
import { Kbd } from './kbd'
import { Dialog, DialogContent, DialogTitle } from './ui/dialog'

export function ShortcutsHelp() {
  const open = useUI((state) => state.helpOpen)
  return <Dialog open={open} onOpenChange={(helpOpen) => setUI({ helpOpen })}>
    <DialogContent className="flex max-h-[85dvh] w-[600px] max-w-[calc(100vw-2rem)] flex-col gap-0 overflow-hidden rounded-[var(--radius-card)] bg-raised p-0 md:max-w-[600px]">
      <DialogTitle className="shrink-0 px-5 py-4 text-[14px] font-medium">Keyboard Shortcuts</DialogTitle>
      <div className="scrollbar-quiet min-h-0 overflow-y-auto px-5 pb-4">
        {['Record', 'General', 'Navigation', 'Triage'].map((group) => <section key={group} className="mb-4">
          <h3 className="mb-1 text-[12px] font-medium text-ink-3">{group}</h3>
          {shortcutIds.filter((id) => shortcuts[id].group === group && shortcuts[id].keys.length && !/^status[2-9]$/.test(id)).map((id) => <div key={id} className="flex min-h-8 items-center justify-between gap-4 text-[13px]">
            <span>{shortcuts[id].label.replace(/ 1$/, ' 1–9')}</span>
            <span className="shrink-0 text-ink-3">{shortcuts[id].keys.map((binding, index) => <span key={binding}>{index > 0 && ' / '}<Kbd>{shortcutLabel(binding).replace(/ 1$/, ' 1–9')}</Kbd></span>)}</span>
          </div>)}
        </section>)}
        <section>
          <h3 className="mb-1 text-[12px] font-medium text-ink-3">Editing</h3>
          {Object.values(editorShortcuts).map(({ label, keys }) => <div key={label} className="flex min-h-8 items-center justify-between gap-4 text-[13px]">{label}<Kbd>{shortcutLabel(keys)}</Kbd></div>)}
        </section>
      </div>
    </DialogContent>
  </Dialog>
}
