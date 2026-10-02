// Tiny motion kit — rAF + CSS only, so animation never costs a dependency.
import { useEffect, useRef, useState, type PointerEvent } from 'react'

const reduced = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches
const easeOutExpo = (t: number) => (t >= 1 ? 1 : 1 - Math.pow(2, -10 * t))

/** Eases from the previous value to `target`; first mount counts up from 0. */
export function useTween(target: number, duration = 900) {
  const [value, setValue] = useState(reduced() ? target : 0)
  const from = useRef(value)
  useEffect(() => {
    if (!Number.isFinite(target)) return setValue(target)
    if (reduced()) return setValue(target)
    const start = performance.now()
    const a = from.current
    let raf = 0
    const step = (now: number) => {
      const v = a + (target - a) * easeOutExpo((now - start) / duration)
      from.current = v
      setValue(v)
      if (now - start < duration) raf = requestAnimationFrame(step)
    }
    raf = requestAnimationFrame(step)
    return () => cancelAnimationFrame(raf)
  }, [target, duration])
  return value
}

/** A number that rolls to its new value. `format` receives the in-flight value. */
export function Tween({ value, format = (n) => Math.round(n).toLocaleString() }: { value: number; format?: (n: number) => string }) {
  return <>{format(useTween(value))}</>
}

/** onPointerMove handler that feeds .spotlight its cursor position. */
export function spotlight(e: PointerEvent<HTMLElement>) {
  const r = e.currentTarget.getBoundingClientRect()
  e.currentTarget.style.setProperty('--mx', `${e.clientX - r.left}px`)
  e.currentTarget.style.setProperty('--my', `${e.clientY - r.top}px`)
}
