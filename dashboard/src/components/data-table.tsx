// Generic TanStack Table wrapper with sorting, global filter and optional row selection.
import {
  flexRender,
  getCoreRowModel,
  getFilteredRowModel,
  getSortedRowModel,
  useReactTable,
  type ColumnDef,
  type RowSelectionState,
  type SortingState,
} from '@tanstack/react-table'
import { useState, type ReactNode } from 'react'
import { LuArrowDown, LuArrowUp, LuArrowUpDown } from 'react-icons/lu'
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
}

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
}: Props<T>) {
  const { t } = useI18n()
  const [sorting, setSorting] = useState<SortingState>([])
  const [globalFilter, setGlobalFilter] = useState('')

  const table = useReactTable({
    data,
    columns,
    state: { sorting, globalFilter, rowSelection: rowSelection ?? {} },
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
  })

  const rows = table.getRowModel().rows

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
              rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() ? 'selected' : undefined}
                  className={cn(onRowClick && 'cursor-pointer')}
                  onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
