// Small building blocks for the docs pages: context (base URL + user key), Code, Callout, headings.
import { createContext, useContext, type ReactNode } from 'react'
import { LuInfo, LuTriangleAlert } from 'react-icons/lu'
import { CopyButton } from '@/components/common'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { cn } from '@/lib/utils'

export const KEY_PLACEHOLDER = 'YOUR_API_KEY'

export const DocsCtx = createContext<{ base: string; apiKey: string }>({ base: '', apiKey: '' })
export const useDocs = () => useContext(DocsCtx)

/** Code block with language label + copy button. Substitutes YOUR_API_KEY with the key typed at the top of the page. */
export function Code({ lang, code, className }: { lang: string; code: string; className?: string }) {
  const { apiKey } = useDocs()
  const text = apiKey ? code.replaceAll(KEY_PLACEHOLDER, apiKey) : code
  return (
    <div className={cn('my-4 overflow-hidden rounded-lg border bg-muted/50', className)}>
      <div className="flex items-center justify-between border-b bg-muted/60 py-1 pr-1 pl-3">
        <span className="font-mono text-[11px] uppercase tracking-wide text-muted-foreground">{lang}</span>
        <CopyButton value={text} />
      </div>
      <pre className="overflow-x-auto p-3 font-mono text-[13px] leading-relaxed">
        <code>{text}</code>
      </pre>
    </div>
  )
}

export function Callout({ kind = 'info', title, children }: { kind?: 'info' | 'warning'; title?: string; children: ReactNode }) {
  const warn = kind === 'warning'
  return (
    <div
      className={cn(
        'my-4 flex gap-3 rounded-lg border p-3 text-sm',
        warn ? 'border-amber-500/40 bg-amber-500/10' : 'border-sky-500/40 bg-sky-500/10',
      )}
    >
      {warn ? <LuTriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-600 dark:text-amber-400" /> : <LuInfo className="mt-0.5 size-4 shrink-0 text-sky-600 dark:text-sky-400" />}
      <div className="min-w-0 space-y-1 [&_code]:rounded [&_code]:bg-background/60 [&_code]:px-1 [&_code]:font-mono [&_code]:text-[12px]">
        {title && <div className="font-medium">{title}</div>}
        <div>{children}</div>
      </div>
    </div>
  )
}

export const H1 = ({ children, lead }: { children: ReactNode; lead?: ReactNode }) => (
  <header className="mb-8">
    <h1 className="text-3xl font-semibold tracking-tight">{children}</h1>
    {lead && <p className="mt-2 text-muted-foreground">{lead}</p>}
  </header>
)
export const H2 = ({ id, children }: { id: string; children: ReactNode }) => (
  <h2 id={id} className="mt-10 mb-3 scroll-mt-20 border-b pb-1.5 text-xl font-semibold tracking-tight">
    {children}
  </h2>
)
export const H3 = ({ id, children }: { id: string; children: ReactNode }) => (
  <h3 id={id} className="mt-6 mb-2 scroll-mt-20 text-base font-semibold">
    {children}
  </h3>
)
export const P = ({ children }: { children: ReactNode }) => <p className="my-3 leading-7">{children}</p>
export const Ul = ({ children }: { children: ReactNode }) => <ul className="my-3 list-disc space-y-1.5 pl-6 leading-7">{children}</ul>
export const Ol = ({ children }: { children: ReactNode }) => <ol className="my-3 list-decimal space-y-1.5 pl-6 leading-7">{children}</ol>
export const C = ({ children }: { children: ReactNode }) => <code className="rounded bg-muted px-1 py-0.5 font-mono text-[12.5px]">{children}</code>
export const A = ({ href, children }: { href: string; children: ReactNode }) => (
  <a href={href} className="font-medium text-primary underline underline-offset-4">
    {children}
  </a>
)

/** Simple table: header row + body rows of ReactNodes. */
export function T({ head, rows }: { head: ReactNode[]; rows: ReactNode[][] }) {
  return (
    <div className="my-4 rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            {head.map((h, i) => (
              <TableHead key={i}>{h}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((r, i) => (
            <TableRow key={i}>
              {r.map((c, j) => (
                <TableCell key={j} className="whitespace-normal align-top">
                  {c}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
