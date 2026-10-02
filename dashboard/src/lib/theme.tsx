import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'

type Theme = 'light' | 'dark'
type Origin = { clientX: number; clientY: number }
const KEY = 'kiro_theme'
const Ctx = createContext<{ theme: Theme; toggle: (from?: Origin) => void } | null>(null)

function initial(): Theme {
  const saved = localStorage.getItem(KEY)
  if (saved === 'light' || saved === 'dark') return saved
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const CURTAIN = 64 // px; drawn small and scaled up so the GPU texture stays tiny
const nextFrame = () => new Promise<void>((r) => requestAnimationFrame(() => requestAnimationFrame(() => r())))

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(initial)
  const busy = useRef(false)
  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
    localStorage.setItem(KEY, theme)
  }, [theme])

  /*
   * Swapping the theme restyles every element: one unavoidably heavy frame.
   * With an origin, a curtain in the new background colour grows from that
   * point (transform only, so it stays on the compositor), the swap and the
   * restyle that follows both happen while the screen is covered, then the
   * curtain fades away. The heavy frames are never seen. (View Transitions
   * were tried first: snapshotting the whole page at retina size stalled
   * 100–300 ms per switch.)
   */
  const toggle = useCallback(async (from?: Origin) => {
    if (busy.current) return
    const root = document.documentElement
    const next: Theme = root.classList.contains('dark') ? 'light' : 'dark'
    const swap = async () => {
      root.classList.add('theme-switching') // freeze colour transitions during the swap
      root.classList.toggle('dark', next === 'dark')
      setTheme(next)
      await nextFrame()
      root.classList.remove('theme-switching') // a second full restyle…
      await nextFrame() // …which also lands while the screen is still covered
    }
    if (!from || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return void swap()

    busy.current = true
    const { clientX: x, clientY: y } = from
    const radius = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y))
    const curtain = document.createElement('div')
    curtain.className = 'theme-curtain'
    curtain.dataset.to = next
    curtain.style.left = `${x - CURTAIN / 2}px`
    curtain.style.top = `${y - CURTAIN / 2}px`
    document.body.appendChild(curtain)
    try {
      await curtain.animate([{ transform: 'scale(0)' }, { transform: `scale(${(radius * 2) / CURTAIN + 0.5})` }], {
        duration: 460,
        easing: 'cubic-bezier(0.65, 0, 0.35, 1)',
        fill: 'forwards',
      }).finished
      await swap()
      await curtain.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 300, easing: 'cubic-bezier(0.16, 1, 0.3, 1)', fill: 'forwards' }).finished
    } finally {
      curtain.remove()
      busy.current = false
    }
  }, [])

  const value = useMemo(() => ({ theme, toggle }), [theme, toggle])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useTheme() {
  const v = useContext(Ctx)
  if (!v) throw new Error('useTheme outside ThemeProvider')
  return v
}
