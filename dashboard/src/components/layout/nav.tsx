// One source of truth for navigation: sidebar, ⌘K palette, page eyebrows and
// the `g <key>` jump shortcuts all read from here.
import type { IconType } from 'react-icons'
import { LuCoins, LuKeyRound, LuLayoutDashboard, LuScrollText, LuSettings, LuUsers } from 'react-icons/lu'

export type NavItem = { to: string; labelKey: string; icon: IconType; key: string; section: 'manage' | 'system' }

export const NAV: NavItem[] = [
  { to: '/', labelKey: 'nav.overview', icon: LuLayoutDashboard, key: 'o', section: 'manage' },
  { to: '/accounts', labelKey: 'nav.accounts', icon: LuUsers, key: 'a', section: 'manage' },
  { to: '/api-keys', labelKey: 'nav.apiKeys', icon: LuKeyRound, key: 'k', section: 'manage' },
  { to: '/model-prices', labelKey: 'nav.modelPrices', icon: LuCoins, key: 'p', section: 'system' },
  { to: '/logs', labelKey: 'nav.logs', icon: LuScrollText, key: 'l', section: 'system' },
  { to: '/settings', labelKey: 'nav.settings', icon: LuSettings, key: 's', section: 'system' },
]

const strip = (p: string) => p.replace(/^\/admin/, '').replace(/\/+$/, '') || '/'

export function navIndex(pathname: string) {
  const p = strip(pathname)
  return NAV.findIndex((n) => (n.to === '/' ? p === '/' : p.startsWith(n.to)))
}
