import { Button as ButtonPrimitive } from "@base-ui/react/button"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "cn"

const buttonVariants = cva(
  "group/button relative isolate inline-flex shrink-0 items-center justify-center overflow-hidden rounded-lg border border-transparent bg-clip-padding text-[13px] font-medium tracking-[-0.005em] whitespace-nowrap outline-none select-none transition-[background-color,border-color,color,box-shadow,transform,opacity] duration-200 ease-(--ease-out-expo) focus-visible:ring-3 focus-visible:ring-ring/40 active:not-aria-[haspopup]:scale-[0.97] active:duration-75 disabled:pointer-events-none disabled:opacity-45 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-[inset_0_1px_0_rgb(255_255_255/0.12),0_1px_2px_hsl(var(--shadow-color)/0.2),0_6px_16px_-8px_hsl(var(--shadow-color)/0.45)] hover:bg-primary/92 before:pointer-events-none before:absolute before:inset-0 before:-z-10 before:-translate-x-full before:bg-[linear-gradient(105deg,transparent_30%,color-mix(in_oklch,var(--primary-foreground)_16%,transparent)_50%,transparent_70%)] before:transition-transform before:duration-700 before:ease-(--ease-out-expo) hover:before:translate-x-full",
        signal:
          "bg-signal text-signal-foreground shadow-[inset_0_1px_0_rgb(255_255_255/0.3),0_0_0_1px_color-mix(in_oklch,var(--signal),black_12%),0_8px_24px_-8px_var(--signal-glow)] hover:brightness-[1.04] hover:shadow-[inset_0_1px_0_rgb(255_255_255/0.3),0_0_0_1px_color-mix(in_oklch,var(--signal),black_12%),0_10px_30px_-6px_var(--signal-glow)] before:pointer-events-none before:absolute before:inset-0 before:-z-10 before:-translate-x-full before:bg-[linear-gradient(105deg,transparent_30%,rgb(255_255_255/0.35)_50%,transparent_70%)] before:transition-transform before:duration-700 before:ease-(--ease-out-expo) hover:before:translate-x-full",
        outline:
          "border-border bg-card/70 shadow-[inset_0_1px_0_var(--hairline-hi),0_1px_2px_hsl(var(--shadow-color)/0.06)] hover:border-foreground/15 hover:bg-accent hover:text-foreground aria-expanded:bg-accent aria-expanded:text-foreground",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-[color-mix(in_oklch,var(--secondary),var(--foreground)_5%)] aria-expanded:bg-secondary aria-expanded:text-secondary-foreground",
        ghost:
          "text-muted-foreground hover:bg-accent hover:text-foreground aria-expanded:bg-accent aria-expanded:text-foreground",
        destructive:
          "bg-destructive/10 text-destructive hover:bg-destructive/20 focus-visible:border-destructive/40 focus-visible:ring-destructive/20 dark:bg-destructive/20 dark:hover:bg-destructive/30 dark:focus-visible:ring-destructive/40",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default:
          "h-8 gap-1.5 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        xs: "h-6 gap-1 rounded-[min(var(--radius-md),10px)] px-2 text-xs in-data-[slot=button-group]:rounded-lg has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-7 gap-1 rounded-[min(var(--radius-md),12px)] px-2.5 text-[0.8rem] in-data-[slot=button-group]:rounded-lg has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-9 gap-1.5 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        icon: "size-8",
        "icon-xs":
          "size-6 rounded-[min(var(--radius-md),10px)] in-data-[slot=button-group]:rounded-lg [&_svg:not([class*='size-'])]:size-3",
        "icon-sm":
          "size-7 rounded-[min(var(--radius-md),12px)] in-data-[slot=button-group]:rounded-lg",
        "icon-lg": "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
