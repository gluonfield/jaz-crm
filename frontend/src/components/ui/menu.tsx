import type { ReactNode } from "react"

// Menus follow Linear's: roomy rows, icons as strong as their labels, quiet
// shortcut hints, a small filled arrow on submenus, and separators that run
// edge to edge. Context and dropdown menus share these classes.
export const menuContent =
  "z-50 max-h-(--radix-popper-available-height) min-w-44 origin-(--radix-popper-transform-origin) overflow-x-hidden overflow-y-auto rounded-[var(--radius-card)] border bg-popover p-1.5 text-popover-foreground shadow-[var(--shadow-raised)] data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95"

export const menuItem =
  "relative flex h-8 cursor-default items-center gap-2.5 rounded-[var(--radius-control)] px-2 text-[13px] text-ink outline-hidden select-none focus:bg-list-active data-[state=open]:bg-list-active data-[disabled]:pointer-events-none data-[disabled]:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4 [&_svg:not([class*='text-'])]:text-ink-2"

export const menuSeparator = "-mx-1.5 my-1.5 h-px bg-border"

export const menuLabel = "flex h-8 items-center truncate px-2 text-[12px] text-ink-3"

export function MenuShortcut({ children }: { children: ReactNode }) {
  return <span className="ml-auto pl-3 text-[12px] tracking-wide whitespace-pre text-ink-3">{children}</span>
}

export function SubmenuArrow({ shortcut }: { shortcut?: string }) {
  return (
    <span className="ml-auto flex items-center gap-3 pl-3 text-[12px] tracking-wide whitespace-pre text-ink-3">
      {shortcut}
      <svg viewBox="0 0 6 8" aria-hidden className="size-2 fill-current text-ink-3">
        <path d="M0 0l6 4-6 4z" />
      </svg>
    </span>
  )
}
