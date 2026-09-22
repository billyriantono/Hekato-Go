import * as React from "react"
import { cn } from "cn"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        // min-h-16 keeps vertical size sensible; w-full + max-w-full keeps the
        // box inside its parent. whitespace-pre-wrap wraps display lines without
        // altering the submitted value; break-all handles long unbreakable JWT
        // tokens; field-sizing-content (the previous default) was the cause of
        // the horizontal overflow that truncated pasted JSON past the dialog.
        "flex field-sizing-fixed min-h-16 w-full max-w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-base whitespace-pre-wrap break-all transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
