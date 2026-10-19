// Generic TanStack Table wrapper with sorting, global filter and optional row selection.
import {
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  getGroupedRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type ExpandedState,
  type Row,
  type RowSelectionState,
  type SortingState,
} from '@tanstack/react-table'
import { useState, type ReactNode } from 'react'
import { LuArrowDown, LuArrowUp, LuArrowUpDown, LuChevronLeft, LuChevronRight } from 'react-icons/lu'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useI18n } from '@/lib/i18n'
import { cn } from '@/lib/utils'

type Props<T> = {
  data: T[]
  columns: ColumnDef<T, unknown>[]
  /** Show a search box filtering across all columns. */
  searchable?: boolean
  searchPlaceholder?: string
  /** Extra controls rendered next to the search box. */
  toolbar?: ReactNode
  emptyState?: ReactNode
  getRowId?: (row: T, index: number) => string
  rowSelection?: RowSelectionState
  onRowSelectionChange?: (s: RowSelectionState) => void
  onRowClick?: (row: T) => void
  className?: string
  /** Rows per page (default 25). Pass 0 to disable pagination. */
  pageSize?: number
  /** Collapse rows sharing a key under one header row; undefined = not grouped. */
  groupBy?: (row: T) => string | undefined
  /** Content of a group's header row (spans the whole table). */
  renderGroup?: (group: GroupRow<T>) => ReactNode
}

export type GroupRow<T> = {
  key: string
  rows: T[]
  expanded: boolean
  toggleExpanded: () => void
  selected: boolean | 'some'
  toggleSelected: (on: boolean) => void
}

const GROUP_COL = '__group'
const SOLO = '\u0000solo:'
// Module-level so their identity is stable: a fresh array/object per render
// makes TanStack recompute the grouped model, whose auto-reset sets state,
// which re-renders — an infinite loop.
const GROUPING = [GROUP_COL]
const HIDE_GROUP_COL = { [GROUP_COL]: false }

export function DataTable<T>({
  data,
  columns,
  searchable,
  searchPlaceholder,
  toolbar,
  emptyState,
  getRowId,
  rowSelection,
  onRowSelectionChange,
  onRowClick,
  className,
  pageSize = 25,
  groupBy,
  renderGroup,
}: Props<T>) {
  const { t } = useI18n()
  const [sorting, setSorting] = useState<SortingState>([])
  const [expanded, setExpanded] = useState<ExpandedState>({})
  // Grouping runs on a hidden column; rows without a group get a unique key so
  // they form a group of one, rendered as a plain row.
  const allColumns: ColumnDef<T, unknown>[] = groupBy
    ? [...columns, { id: GROUP_COL, accessorFn: (r, i) => groupBy(r) ?? SOLO + i }]
    : columns
  const [globalFilter, setGlobalFilter] = useState('')
  const paginate = pageSize > 0

  const table = useReactTable({
    data,
    columns: allColumns,
    state: {
      sorting,
      globalFilter,
      rowSelection: rowSelection ?? {},
      ...(groupBy && { grouping: GROUPING, expanded, columnVisibility: HIDE_GROUP_COL }),
    },
    onExpandedChange: setExpanded,
    // Keep pools open across data refetches (every action refetches the list).
    autoResetExpanded: false,
    initialState: { pagination: { pageSize: paginate ? pageSize : Number.MAX_SAFE_INTEGER } },
    autoResetPageIndex: true,
    onSortingChange: setSorting,
    onGlobalFilterChange: setGlobalFilter,
    onRowSelectionChange: (updater) => {
      if (!onRowSelectionChange) return
      const next = typeof updater === 'function' ? updater(rowSelection ?? {}) : updater
      onRowSelectionChange(next)
    },
    enableRowSelection: !!onRowSelectionChange,
    getRowId,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    ...(groupBy && { getGroupedRowModel: getGroupedRowModel(), getExpandedRowModel: getExpandedRowModel() }),
  })

  const leafRow = (row: Row<T>) => (
    <TableRow
      key={row.id}
      data-state={row.getIsSelected() ? 'selected' : undefined}
      className={cn(onRowClick && 'cursor-pointer', row.depth > 0 && 'bg-muted/20')}
      onClick={onRowClick ? () => onRowClick(row.original) : undefined}
    >
      {row.getVisibleCells().map((cell) => (
        <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
      ))}
    </TableRow>
  )
  const groupRow = (row: Row<T>) => {
    const key = String(row.groupingValue)
    if (key.startsWith(SOLO) || !renderGroup) return leafRow(row.subRows[0])
    return (
      <TableRow key={row.id} className="bg-muted/40 hover:bg-muted/50">
        <TableCell colSpan={columns.length}>
          {renderGroup({
            key,
            rows: row.subRows.map((r) => r.original),
            expanded: row.getIsExpanded(),
            toggleExpanded: row.getToggleExpandedHandler(),
            selected: row.getIsAllSubRowsSelected() || (row.getIsSomeSelected() && 'some'),
            toggleSelected: (on) => row.toggleSelected(on),
          })}
        </TableCell>
      </TableRow>
    )
  }

  const rows = table.getRowModel().rows
  const total = table.getFilteredRowModel().rows.length
  const { pageIndex, pageSize: size } = table.getState().pagination
  const pageCount = table.getPageCount()

  return (
    <div className={cn('space-y-3', className)}>
      {(searchable || toolbar) && (
        <div className="flex flex-wrap items-center gap-2">
          {searchable && (
            <Input
              value={globalFilter}
              onChange={(e) => setGlobalFilter(e.target.value)}
              placeholder={searchPlaceholder ?? t('common.search')}
              className="max-w-xs"
            />
          )}
          {toolbar}
        </div>
      )}
      <div className="overflow-hidden rounded-lg border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((hg) => (
              <TableRow key={hg.id}>
                {hg.headers.map((header) => {
                  const canSort = header.column.getCanSort()
                  const dir = header.column.getIsSorted()
                  return (
                    <TableHead
                      key={header.id}
                      className={cn(canSort && 'cursor-pointer select-none')}
                      onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                    >
                      <span className="inline-flex items-center gap-1">
                        {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                        {canSort &&
                          (dir === 'asc' ? (
                            <LuArrowUp className="size-3" />
                          ) : dir === 'desc' ? (
                            <LuArrowDown className="size-3" />
                          ) : (
                            <LuArrowUpDown className="size-3 opacity-40" />
                          ))}
                      </span>
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {rows.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="p-0">
                  {emptyState ?? <div className="py-10 text-center text-sm text-muted-foreground">{t('common.noData')}</div>}
                </TableCell>
              </TableRow>
            ) : (
              rows.map((row) => (row.getIsGrouped() ? groupRow(row) : leafRow(row)))
            )}
          </TableBody>
        </Table>
      </div>
      {paginate && total > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
          <span>{t('table.range', pageIndex * size + 1, Math.min(total, (pageIndex + 1) * size), total)}</span>
          <div className="flex items-center gap-2">
            <select
              className="h-7 rounded-md border border-input bg-background px-1 text-xs"
              value={size}
              onChange={(e) => table.setPageSize(Number(e.target.value))}
              aria-label={t('table.pageSize')}
            >
              {[10, 25, 50, 100].map((n) => (
                <option key={n} value={n}>
                  {t('table.perPage', n)}
                </option>
              ))}
            </select>
            <Button variant="outline" size="icon-sm" onClick={() => table.previousPage()} disabled={!table.getCanPreviousPage()} aria-label={t('table.prev')}>
              <LuChevronLeft className="size-3.5" />
            </Button>
            <span className="tabular-nums">
              {pageIndex + 1} / {Math.max(1, pageCount)}
            </span>
            <Button variant="outline" size="icon-sm" onClick={() => table.nextPage()} disabled={!table.getCanNextPage()} aria-label={t('table.next')}>
              <LuChevronRight className="size-3.5" />
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
