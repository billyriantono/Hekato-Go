"use client"

import { Tabs as TabsPrimitive } from "@base-ui/react/tabs"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"

function Tabs({
  className,
  orientation = "horizontal",
  ...props
}: TabsPrimitive.Root.Props) {
  return (
    <TabsPrimitive.Root
      data-slot="tabs"
      data-orientation={orientation}
      className={cn(
        "group/tabs flex gap-2 data-horizontal:flex-col",
        className
      )}
      {...props}
    />
  )
}

const tabsListVariants = cva(
  "group/tabs-list relative isolate inline-flex w-fit items-center justify-center rounded-lg p-[3px] text-muted-foreground group-data-horizontal/tabs:h-8.5 group-data-vertical/tabs:h-fit group-data-vertical/tabs:flex-col data-[variant=line]:rounded-none",
  {
    variants: {
      variant: {
        default: "bg-muted/80 shadow-[inset_0_1px_2px_hsl(var(--shadow-color)/0.08)] ring-1 ring-border",
        line: "gap-1 bg-transparent",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

// The active pill (or underline) is one element that glides between tabs.
function TabsList({
  className,
  variant = "default",
  children,
  ...props
}: TabsPrimitive.List.Props & VariantProps<typeof tabsListVariants>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      data-variant={variant}
      className={cn(tabsListVariants({ variant }), className)}
      {...props}
    >
      {children}
      <TabsPrimitive.Indicator
        data-slot="tabs-indicator"
        className={cn(
          "absolute -z-10 transition-[left,top,width,height] duration-500 ease-(--ease-spring)",
          variant === "line"
            ? "group-data-horizontal/tabs:bottom-[-5px] group-data-horizontal/tabs:left-(--active-tab-left) group-data-horizontal/tabs:h-0.5 group-data-horizontal/tabs:w-(--active-tab-width) group-data-vertical/tabs:top-(--active-tab-top) group-data-vertical/tabs:-right-1 group-data-vertical/tabs:h-(--active-tab-height) group-data-vertical/tabs:w-0.5 rounded-full bg-signal shadow-[0_0_10px_var(--signal-glow)]"
            : "top-(--active-tab-top) left-(--active-tab-left) h-(--active-tab-height) w-(--active-tab-width) rounded-md bg-card shadow-[inset_0_1px_0_var(--hairline-hi),0_1px_3px_hsl(var(--shadow-color)/0.18)] ring-1 ring-border"
        )}
      />
    </TabsPrimitive.List>
  )
}

function TabsTrigger({ className, ...props }: TabsPrimitive.Tab.Props) {
  return (
    <TabsPrimitive.Tab
      data-slot="tabs-trigger"
      className={cn(
        "relative inline-flex h-[calc(100%-1px)] flex-1 items-center justify-center gap-1.5 rounded-md border border-transparent px-2 py-0.5 text-[13px] font-medium whitespace-nowrap text-muted-foreground transition-colors duration-200 group-data-vertical/tabs:w-full group-data-vertical/tabs:justify-start hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/40 focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50 has-data-[icon=inline-end]:pr-1 has-data-[icon=inline-start]:pl-1 aria-disabled:pointer-events-none aria-disabled:opacity-50 data-active:text-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    />
  )
}

function TabsContent({ className, ...props }: TabsPrimitive.Panel.Props) {
  return (
    <TabsPrimitive.Panel
      data-slot="tabs-content"
      className={cn("flex-1 text-sm outline-none animate-[fade_0.4s_var(--ease-out-expo)_both]", className)}
      {...props}
    />
  )
}

export { Tabs, TabsList, TabsTrigger, TabsContent, tabsListVariants }
