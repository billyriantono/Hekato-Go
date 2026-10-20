// Searchable model picker: type to filter, arrow keys + Enter to pick.
// Also accepts a free-form ID that is not in the list (Enter on the query).
import { useState } from 'react'
import { LuChevronsUpDown, LuX } from 'react-icons/lu'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useI18n } from '@/lib/i18n'
import { cn } from '@/lib/utils'

export function ModelCombobox({
  value,
  onChange,
  options,
  placeholder,
  emptyLabel,
  allowCustom = true,
  className,
}: {
  value: string
  onChange: (v: string) => void
  options: string[]
  /** Shown when value is '' (e.g. "automatic"). */
  placeholder?: string
  /** Optional first entry that selects '' (e.g. "(automatic)"). */
  emptyLabel?: string
  allowCustom?: boolean
  className?: string
}) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const q = query.trim().toLowerCase()
  const filtered = q ? options.filter((o) => o.toLowerCase().includes(q)) : options
  const exact = options.some((o) => o.toLowerCase() === q)
  const pick = (v: string) => {
    onChange(v)
    setOpen(false)
    setQuery('')
  }
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button variant="outline" role="combobox" aria-expanded={open} className={cn('justify-between font-mono text-xs font-normal', className)}>
            <span className="truncate">{value || placeholder || t('common.search')}</span>
            <LuChevronsUpDown className="ml-2 size-3.5 shrink-0 opacity-50" />
          </Button>
        }
      />
      <PopoverContent className="w-(--anchor-width) min-w-72 p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={t('models.searchPlaceholder')}
            value={query}
            onValueChange={setQuery}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && allowCustom && q && !exact && filtered.length === 0) {
                e.preventDefault()
                pick(query.trim())
              }
            }}
          />
          <CommandList className="max-h-72">
            <CommandEmpty>
              {allowCustom && q ? (
                <button type="button" className="w-full px-2 py-1 text-left font-mono text-xs hover:bg-muted" onClick={() => pick(query.trim())}>
                  {t('models.useCustom', query.trim())}
                </button>
              ) : (
                t('common.noData')
              )}
            </CommandEmpty>
            <CommandGroup>
              {emptyLabel && !q && (
                <CommandItem value="__auto__" onSelect={() => pick('')} data-checked={value === ''}>
                  <span className="text-muted-foreground">{emptyLabel}</span>
                </CommandItem>
              )}
              {filtered.slice(0, 300).map((o) => (
                <CommandItem key={o} value={o} onSelect={() => pick(o)} data-checked={o === value} className="font-mono text-xs">
                  {o}
                </CommandItem>
              ))}
              {filtered.length > 300 && <div className="px-2 py-1 text-[11px] text-muted-foreground">{t('models.moreResults', filtered.length - 300)}</div>}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

// A list of model IDs as removable chips plus a picker to add one more.
export function ModelListPicker({
  value,
  onChange,
  options,
  placeholder,
  variant = 'secondary',
}: {
  value: string[]
  onChange: (v: string[]) => void
  options: string[]
  placeholder?: string
  variant?: 'secondary' | 'outline' | 'destructive'
}) {
  const { t } = useI18n()
  const has = (m: string) => value.some((x) => x.toLowerCase() === m.toLowerCase())
  return (
    <div className="space-y-1.5">
      {value.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {value.map((m) => (
            <Badge key={m} variant={variant} className="max-w-full gap-1 font-mono text-xs">
              <span className="truncate">{m}</span>
              <button type="button" aria-label={t('common.remove')} className="opacity-60 hover:opacity-100" onClick={() => onChange(value.filter((x) => x !== m))}>
                <LuX className="size-3" />
              </button>
            </Badge>
          ))}
        </div>
      )}
      <ModelCombobox value="" onChange={(m) => m && !has(m) && onChange([...value, m])} options={options.filter((m) => !has(m))} placeholder={placeholder} className="w-full" />
    </div>
  )
}
