// Brand assets from the designer, compressed for the slow-network budget:
// icon.webp 8 KB (128px), logo-light.webp 14 KB, logo-dark.webp 18 KB (2x of
// 172px). favicon.ico (a tighter funnel crop that survives 16px) and
// apple-touch-icon.png in public/ come from the same icon.
import icon from '@/assets/icon.webp'
import logoDark from '@/assets/logo-dark.webp'
import logoLight from '@/assets/logo-light.webp'
import { cn } from '@/lib/utils'

export { icon as iconUrl }

/** The funnel icon: many requests in, one gateway, one way out. */
export function Mark({ className }: { className?: string }) {
  return <img src={icon} alt="" aria-hidden="true" width={64} height={64} decoding="async" className={cn('shrink-0 object-contain', className)} />
}

/** Horizontal logo; each theme gets the variant drawn for its background. */
export function Wordmark({ className }: { className?: string }) {
  return (
    <div className={cn('flex items-center', className)}>
      <img src={logoLight} alt="Hekato Gateway" width={172} height={43} decoding="async" className="h-[43px] w-[172px] dark:hidden" />
      <img src={logoDark} alt="Hekato Gateway" width={172} height={45} decoding="async" className="hidden h-[45px] w-[172px] dark:block" />
    </div>
  )
}

/** In-app twin of the HTML boot splash, so auth resolution never flashes. */
export function Splash() {
  return (
    <div className="fixed inset-0 grid place-items-center" role="status" aria-label="Loading">
      <div className="flex flex-col items-center gap-5">
        <Mark className="size-[72px]" />
        <div className="relative h-px w-[120px] overflow-hidden bg-border">
          <div className="absolute inset-y-0 w-2/5 bg-gradient-to-r from-signal to-pop [animation:boot-scan_1.3s_cubic-bezier(.65,0,.35,1)_infinite]" />
        </div>
      </div>
      <style>{'@keyframes boot-scan{from{transform:translateX(-100%)}to{transform:translateX(250%)}}'}</style>
    </div>
  )
}
