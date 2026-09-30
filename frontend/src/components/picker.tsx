import { Command as CommandPrimitive } from 'cmdk'
import { Check, Plus } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'

export type PickerOption = { value: string; label: string; icon?: ReactNode }

// Picker is Linear's property menu: a filter field over a keyboard-driven
// list. Number keys pick the first nine options when the filter is empty.
// With onSearch the caller filters, for options searched on the server.
export function Picker({
  trigger,
  placeholder,
  options,
  selected = [],
  onSelect,
  onSearch,
  onCreate,
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
      }}
    >
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent align={align} sideOffset={6} className="w-64 overflow-hidden rounded-[var(--radius-card)] p-0" onClick={(e) => e.stopPropagation()}>
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
            autoFocus
            aria-label={placeholder}
            value={search}
            onValueChange={(next) => {
              setSearch(next)
              onSearch?.(next)
            }}
            placeholder={placeholder}
            className="h-9 w-full border-b border-border bg-transparent px-3 text-[13px] text-ink outline-none placeholder:text-ink-3"
          />
          <CommandPrimitive.List className="scrollbar-quiet max-h-72 overflow-y-auto p-1">
            <CommandPrimitive.Empty className="px-2 py-3 text-center text-[12px] text-ink-3">No results</CommandPrimitive.Empty>
            {options.map((option, index) => {
              const active = selected.includes(option.value)
              return (
                <CommandPrimitive.Item
                  key={option.value}
                  disabled={disabled}
                  value={`${option.label} ${option.value}`}
                  onSelect={() => pick(option.value)}
                  className="flex h-8 cursor-default items-center gap-2.5 rounded-[5px] px-2 text-[13px] text-ink outline-none data-[selected=true]:bg-list-active data-[disabled=true]:opacity-50"
                >
                  {multiple && (
                    <span className={cn('flex size-3.5 items-center justify-center rounded-[4px] border border-ink-3/60', active && 'border-primary bg-primary text-on-primary')}>
                      {active && <Check className="size-2.5" strokeWidth={3} />}
                    </span>
                  )}
                  {option.icon}
                  <span className="min-w-0 flex-1 truncate">{option.label}</span>
                  {!multiple && active && <Check className="size-3.5 text-ink-2" />}
                  {!onCreate && index < 9 && <span className="w-3 text-right text-[11px] tabular-nums text-ink-3">{index + 1}</span>}
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
                className="flex h-8 cursor-default items-center gap-2.5 rounded-[5px] px-2 text-[13px] text-ink outline-none data-[selected=true]:bg-list-active data-[disabled=true]:opacity-50"
              >
                <Plus className="size-3.5 text-ink-3" />
                <span className="truncate">Create “{newValue}”</span>
              </CommandPrimitive.Item>
            )}
          </CommandPrimitive.List>
        </CommandPrimitive>
      </PopoverContent>
    </Popover>
  )
}
