import { Command as CommandPrimitive } from 'cmdk'
import { Check, Plus } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'

export type PickerOption = { value: string; label: string; icon?: ReactNode; hint?: ReactNode }

// Picker is Linear's property menu: a filter field over a keyboard-driven
// list. Number keys pick the first nine options when the filter is empty.
// With onSearch the caller filters, for options searched on the server.
// Actions follow the options, apart from them.
export function Picker({
  trigger,
  placeholder,
  options,
  selected = [],
  onSelect,
  onSearch,
  onCreate,
  onOpenChange,
  actions = [],
  disabled = false,
  multiple = false,
  align = 'start',
}: {
  trigger: ReactNode
  placeholder: string
  options: PickerOption[]
  selected?: string[]
  onSelect: (value: string) => void
  onSearch?: (search: string) => void
  onCreate?: (value: string) => void
  onOpenChange?: (open: boolean) => void
  actions?: { label: string; onSelect: () => void }[]
  disabled?: boolean
  multiple?: boolean
  align?: 'start' | 'end'
}) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const newValue = search.trim()
  const canCreate = onCreate && newValue && !options.some((o) => o.label.toLowerCase() === newValue.toLowerCase())
  const pick = (value: string) => {
    onSelect(value)
    if (!multiple) {
      setOpen(false)
    }
  }
  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        setSearch('')
        onSearch?.('')
        onOpenChange?.(next)
      }}
    >
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent align={align} sideOffset={6} className="w-64 overflow-hidden p-0" onClick={(e) => e.stopPropagation()}>
        <CommandPrimitive
          loop
          shouldFilter={!onSearch}
          onKeyDown={(e) => {
            const digit = Number(e.key)
            if (!onCreate && !disabled && !e.metaKey && !e.ctrlKey && digit >= 1 && digit <= 9 && !search && options[digit - 1]) {
              e.preventDefault()
              pick(options[digit - 1].value)
            }
          }}
        >
          <CommandPrimitive.Input
            asChild
            autoFocus
            aria-label={placeholder}
            value={search}
            onValueChange={(next) => {
              setSearch(next)
              onSearch?.(next)
            }}
            placeholder={placeholder}
            className="h-10 w-full border-b border-border bg-transparent px-3.5 text-[13px] text-ink outline-none placeholder:text-ink-3"
          >
            <input type="search" />
          </CommandPrimitive.Input>
          <CommandPrimitive.List className="scrollbar-quiet max-h-72 overflow-y-auto p-1.5">
            <CommandPrimitive.Empty className="px-2 py-3 text-center text-[12px] text-ink-3">No results</CommandPrimitive.Empty>
            {options.map((option, index) => {
              const active = selected.includes(option.value)
              return (
                <CommandPrimitive.Item
                  key={option.value}
                  disabled={disabled}
                  value={`${option.label} ${option.value}`}
                  onSelect={() => pick(option.value)}
                  className="flex h-8 cursor-default items-center gap-2.5 rounded-[var(--radius-control)] px-2 text-[13px] text-ink outline-none data-[selected=true]:bg-list-active data-[disabled=true]:opacity-50"
                >
                  {multiple && (
                    <span className={cn('flex size-4 shrink-0 items-center justify-center rounded-[4px] border border-ink-3/60', active && 'border-primary bg-primary text-on-primary')}>
                      {active && <Check className="size-3" strokeWidth={3} />}
                    </span>
                  )}
                  {option.icon}
                  <span className="min-w-0 flex-1 truncate">{option.label}</span>
                  {!multiple && active && <Check className="size-4 shrink-0 text-ink" />}
                  {option.hint !== undefined ? <span className="text-[12px] tabular-nums text-ink-3">{option.hint}</span> : !onCreate && index < 9 && <span className="w-3 text-right text-[12px] tabular-nums text-ink-3">{index + 1}</span>}
                </CommandPrimitive.Item>
              )
            })}
            {canCreate && (
              <CommandPrimitive.Item
                value={`Create ${newValue}`}
                disabled={disabled}
                onSelect={() => {
                  onCreate(newValue)
                  setSearch('')
                  if (!multiple) {
                    setOpen(false)
                  }
                }}
                className="flex h-8 cursor-default items-center gap-2.5 rounded-[var(--radius-control)] px-2 text-[13px] text-ink outline-none data-[selected=true]:bg-list-active data-[disabled=true]:opacity-50"
              >
                <Plus className="size-4 text-ink-2" />
                <span className="truncate">Create “{newValue}”</span>
              </CommandPrimitive.Item>
            )}
            {actions.length > 0 && <CommandPrimitive.Group className="-mx-1.5 mt-1.5 border-t border-border px-1.5 pt-1.5">
              {actions.map((action) => (
                <CommandPrimitive.Item
                  key={action.label}
                  value={action.label}
                  onSelect={() => {
                    setOpen(false)
                    onOpenChange?.(false)
                    action.onSelect()
                  }}
                  className="flex h-8 cursor-default items-center rounded-[var(--radius-control)] px-2 text-[12.5px] text-ink-2 outline-none data-[selected=true]:bg-list-active data-[selected=true]:text-ink"
                >
                  {action.label}
                </CommandPrimitive.Item>
              ))}
            </CommandPrimitive.Group>}
          </CommandPrimitive.List>
        </CommandPrimitive>
      </PopoverContent>
    </Popover>
  )
}
