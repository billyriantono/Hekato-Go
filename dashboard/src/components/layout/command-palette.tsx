// ⌘K — jump anywhere, flip theme/language, sign out. Lazy-loaded on first open.
import { Command } from 'cmdk'
import { useNavigate } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { LuBookOpen, LuCornerDownLeft, LuLanguages, LuLogOut, LuSearch, LuSunMoon } from 'react-icons/lu'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { useAuth } from '@/lib/auth'
import { useI18n } from '@/lib/i18n'
import { useTheme } from '@/lib/theme'
import { NAV } from './nav'

function Item({ icon, label, hint, onSelect }: { icon: ReactNode; label: string; hint?: ReactNode; onSelect: () => void }) {
  return (
    <Command.Item
      value={label}
      onSelect={onSelect}
      className="group/item relative flex h-10 cursor-pointer items-center gap-3 rounded-lg px-2.5 text-[13px] text-muted-foreground transition-colors duration-150 outline-none select-none before:absolute before:inset-y-2.5 before:left-0 before:w-0.5 before:rounded-full before:bg-signal before:opacity-0 before:transition-opacity data-[selected=true]:bg-accent data-[selected=true]:text-foreground data-[selected=true]:before:opacity-100"
    >
      <span className="grid size-6 place-items-center rounded-md border bg-card text-muted-foreground transition-colors group-data-[selected=true]/item:border-signal/40 group-data-[selected=true]/item:text-signal [&_svg]:size-3.5">
        {icon}
      </span>
      <span className="flex-1 truncate">{label}</span>
      {hint && <span className="flex gap-1">{hint}</span>}
    </Command.Item>
  )
}

export default function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const { t, lang, setLang } = useI18n()
  const { toggle } = useTheme()
  const { logout } = useAuth()
  const navigate = useNavigate()
  const run = (fn: () => void) => () => {
    onOpenChange(false)
    fn()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent showCloseButton={false} className="top-[18%] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-[560px] data-open:slide-in-from-top-2">
        <DialogTitle className="sr-only">{t('palette.title')}</DialogTitle>
        <Command loop className="flex flex-col">
          <div className="flex items-center gap-3 border-b px-4">
            <LuSearch className="size-4 shrink-0 text-muted-foreground" />
            <Command.Input
              autoFocus
              placeholder={t('palette.placeholder')}
              className="h-13 flex-1 bg-transparent text-[15px] outline-none placeholder:text-muted-foreground/60"
            />
            <kbd className="key">esc</kbd>
          </div>
          <Command.List className="max-h-[min(380px,60vh)] scroll-py-2 overflow-y-auto p-2 [&_[cmdk-group-heading]]:eyebrow [&_[cmdk-group-heading]]:px-2.5 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:pb-1.5 [&_[cmdk-group-heading]]:text-[9.5px]">
            <Command.Empty className="py-10 text-center text-[13px] text-muted-foreground">{t('palette.empty')}</Command.Empty>
            <Command.Group heading={t('palette.navigate')}>
              {NAV.map((n) => (
                <Item
                  key={n.to}
                  icon={<n.icon />}
                  label={t(n.labelKey)}
                  hint={<><kbd className="key">g</kbd><kbd className="key">{n.key}</kbd></>}
                  onSelect={run(() => navigate({ to: n.to }))}
                />
              ))}
            </Command.Group>
            <Command.Group heading={t('palette.actions')}>
              <Item icon={<LuSunMoon />} label={t('header.theme')} onSelect={run(() => toggle())} />
              <Item icon={<LuLanguages />} label={lang === 'en' ? '切换到中文' : 'Switch to English'} onSelect={run(() => setLang(lang === 'en' ? 'zh' : 'en'))} />
              <Item icon={<LuBookOpen />} label={t('header.docs')} onSelect={run(() => window.open('/docs', '_blank'))} />
              <Item icon={<LuLogOut />} label={t('common.logout')} onSelect={run(logout)} />
            </Command.Group>
          </Command.List>
          <div className="flex items-center gap-4 border-t bg-muted/40 px-4 py-2 font-mono text-[10.5px] text-muted-foreground">
            <span className="flex items-center gap-1.5"><kbd className="key">↑</kbd><kbd className="key">↓</kbd>{t('palette.move')}</span>
            <span className="flex items-center gap-1.5"><kbd className="key"><LuCornerDownLeft className="size-3" /></kbd>{t('palette.open')}</span>
            <span className="ml-auto flex items-center gap-1.5"><kbd className="key">⌘</kbd><kbd className="key">K</kbd></span>
          </div>
        </Command>
      </DialogContent>
    </Dialog>
  )
}
