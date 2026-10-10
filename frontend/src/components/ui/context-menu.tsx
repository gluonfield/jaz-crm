import * as React from "react"
import { cn } from "@/lib/utils"
import { CheckIcon, PlusIcon } from "lucide-react"
import { ContextMenu as ContextMenuPrimitive } from "radix-ui"
import { MenuShortcut, SubmenuArrow, menuContent, menuItem, menuSeparator } from "./menu"

function ContextMenu({
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.Root>) {
  return <ContextMenuPrimitive.Root data-slot="context-menu" {...props} />
}

function ContextMenuTrigger({
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.Trigger>) {
  return (
    <ContextMenuPrimitive.Trigger data-slot="context-menu-trigger" {...props} />
  )
}

function ContextMenuSub({
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.Sub>) {
  return <ContextMenuPrimitive.Sub data-slot="context-menu-sub" {...props} />
}

function ContextMenuSubTrigger({
  className,
  children,
  shortcut,
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.SubTrigger> & {
  shortcut?: string
}) {
  return (
    <ContextMenuPrimitive.SubTrigger
      data-slot="context-menu-sub-trigger"
      className={cn(menuItem, className)}
      {...props}
    >
      {children}
      <SubmenuArrow shortcut={shortcut} />
    </ContextMenuPrimitive.SubTrigger>
  )
}

function ContextMenuSubContent({
  className,
  collisionPadding = 8,
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.SubContent>) {
  return (
    <ContextMenuPrimitive.Portal>
      <ContextMenuPrimitive.SubContent
        data-slot="context-menu-sub-content"
        collisionPadding={collisionPadding}
        className={cn(menuContent, className)}
        {...props}
      />
    </ContextMenuPrimitive.Portal>
  )
}

function ContextMenuContent({
  className,
  collisionPadding = 8,
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.Content>) {
  return (
    <ContextMenuPrimitive.Portal>
      <ContextMenuPrimitive.Content
        data-slot="context-menu-content"
        collisionPadding={collisionPadding}
        className={cn(menuContent, className)}
        {...props}
      />
    </ContextMenuPrimitive.Portal>
  )
}

function ContextMenuItem({
  className,
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.Item>) {
  return (
    <ContextMenuPrimitive.Item
      data-slot="context-menu-item"
      className={cn(menuItem, className)}
      {...props}
    />
  )
}

function ContextMenuSeparator({
  className,
  ...props
}: React.ComponentProps<typeof ContextMenuPrimitive.Separator>) {
  return (
    <ContextMenuPrimitive.Separator
      data-slot="context-menu-separator"
      className={cn(menuSeparator, className)}
      {...props}
    />
  )
}

const ContextMenuShortcut = MenuShortcut

type MenuOption = { value: string; label: string; icon?: React.ReactNode }

// ContextMenuOptions is Linear's property submenu: a filter over the options,
// the chosen ones checked, and number keys picking one of the first nine
// before anything is typed. Typing anywhere in it, or on its row, filters.
function ContextMenuOptions({
  icon,
  label,
  shortcut,
  placeholder = `Change ${label.toLowerCase()}…`,
  options,
  selected,
  onSelect,
  onCreate,
  multiple = false,
  disabled = false,
}: {
  icon: React.ReactNode
  label: string
  shortcut?: string
  placeholder?: string
  options: MenuOption[]
  selected: string[]
  onSelect: (value: string) => void
  onCreate?: (label: string) => void
  multiple?: boolean
  disabled?: boolean
}) {
  const [query, setQuery] = React.useState("")
  const input = React.useRef<HTMLInputElement>(null)
  const text = query.trim()
  const shown = options.filter((o) => o.label.toLowerCase().includes(text.toLowerCase()))
  const exact = shown.findIndex((o) => o.label.toLowerCase() === text.toLowerCase())
  const items = () => [...(input.current?.parentElement?.querySelectorAll<HTMLElement>("[role^=menuitem]") ?? [])]
  const create = () => {
    onCreate?.(text)
    setQuery("")
  }
  const typeIn = (e: React.KeyboardEvent) => {
    if (!input.current || e.key.length !== 1 || e.key === " " || e.metaKey || e.ctrlKey || e.altKey) {
      return
    }
    e.stopPropagation()
    if (!query && !onCreate && Number(e.key) >= 1) {
      e.preventDefault()
      items()[Number(e.key) - 1]?.click()
    } else if (e.target !== input.current) {
      input.current.focus()
    }
  }
  return (
    <ContextMenuSub onOpenChange={() => setQuery("")}>
      <ContextMenuSubTrigger shortcut={shortcut} onKeyDownCapture={typeIn}>
        {icon}
        {label}
      </ContextMenuSubTrigger>
      <ContextMenuSubContent className="w-56" onKeyDownCapture={typeIn}>
        <input
          ref={input}
          value={query}
          aria-label={placeholder}
          placeholder={placeholder}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key !== "Escape") {
              e.stopPropagation()
            }
            if (e.key === "Enter" && text) {
              if (exact < 0 && onCreate) {
                create()
              } else {
                items()[Math.max(exact, 0)]?.click()
                setQuery("")
              }
            }
            if (e.key === "ArrowDown") {
              items()[0]?.focus()
            }
          }}
          className="h-8 w-full bg-transparent px-2 text-[13px] text-ink outline-none placeholder:text-ink-3 pointer-coarse:text-[16px]"
        />
        {(shown.length > 0 || (text && onCreate)) && <ContextMenuSeparator />}
        {shown.map((option, index) =>
          multiple ? (
            <ContextMenuPrimitive.CheckboxItem
              key={option.value}
              checked={selected.includes(option.value)}
              disabled={disabled}
              onSelect={(e) => e.preventDefault()}
              onCheckedChange={() => onSelect(option.value)}
              className={cn(menuItem, "group")}
            >
              <span className="flex size-4 shrink-0 items-center justify-center rounded-[4px] border border-ink-3/60 opacity-0 group-data-[highlighted]:opacity-100 group-data-[state=checked]:border-primary group-data-[state=checked]:bg-primary group-data-[state=checked]:opacity-100">
                <ContextMenuPrimitive.ItemIndicator>
                  <CheckIcon className="size-3 text-on-primary" strokeWidth={3} />
                </ContextMenuPrimitive.ItemIndicator>
              </span>
              {option.icon}
              <span className="truncate">{option.label}</span>
            </ContextMenuPrimitive.CheckboxItem>
          ) : (
            <ContextMenuItem key={option.value} disabled={disabled} onSelect={() => onSelect(option.value)}>
              {option.icon}
              <span className="min-w-0 flex-1 truncate">{option.label}</span>
              {selected.includes(option.value) && <CheckIcon className="text-ink" />}
              {!text && !onCreate && index < 9 && <span className="w-3 text-right text-[12px] tabular-nums text-ink-3">{index + 1}</span>}
            </ContextMenuItem>
          ),
        )}
        {text && exact < 0 && onCreate && (
          <ContextMenuItem
            disabled={disabled}
            onSelect={(e) => {
              e.preventDefault()
              create()
            }}
          >
            <PlusIcon /> Create “{text}”
          </ContextMenuItem>
        )}
      </ContextMenuSubContent>
    </ContextMenuSub>
  )
}

export {
  ContextMenu,
  ContextMenuTrigger,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuOptions,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubTrigger,
  ContextMenuSubContent,
}
