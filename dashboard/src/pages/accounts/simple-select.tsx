import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export type Option = { value: string; label: string }

/** Thin wrapper so Select usage stays one-liner everywhere. */
export function SimpleSelect({
  value,
  onChange,
  options,
  className,
  size,
  disabled,
}: {
  value: string
  onChange: (v: string) => void
  options: Option[]
  className?: string
  size?: 'sm' | 'default'
  disabled?: boolean
}) {
  const label = options.find((o) => o.value === value)?.label ?? value
  return (
    <Select value={value} onValueChange={(v) => onChange(String(v))} disabled={disabled}>
      <SelectTrigger className={className} size={size}>
        <SelectValue>{label}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            {o.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
