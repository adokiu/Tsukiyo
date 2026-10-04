import { type ReactNode, useCallback, useMemo } from 'react'
import { Loader2, ArrowUp, ArrowDown, ArrowUpDown } from 'lucide-react'
import { Pagination } from './Pagination'
import './DataTable.css'

export interface RowContext {
  level: number
  expanded: boolean
  canExpand: boolean
  onToggleExpand?: (key: string | number) => void
}

export interface Column<T> {
  key: string
  title: string
  width?: number | string
  sortable?: boolean
  render?: (row: T, index: number, ctx?: RowContext) => ReactNode
}

export interface SortState {
  field: string
  order: 'asc' | 'desc'
}

export interface PaginationState {
  page: number
  size: number
  total: number
}

interface DataTableProps<T> {
  columns: Column<T>[]
  data: T[]
  rowKey: (row: T) => string | number
  loading?: boolean
  pagination?: PaginationState
  onPageChange?: (page: number) => void
  onSizeChange?: (size: number) => void
  emptyText?: string
  selectable?: boolean
  selectedKeys?: Set<string | number>
  onSelectionChange?: (keys: Set<string | number>) => void
  sort?: SortState
  onSortChange?: (sort: SortState) => void
  header?: ReactNode
  footer?: ReactNode
  expandedRowKeys?: (string | number)[]
  renderExpanded?: (row: T) => ReactNode
  getChildren?: (row: T) => T[]
  hasChildren?: (row: T) => boolean
  expandedKeys?: Set<string | number>
  onToggleExpand?: (key: string | number) => void
  getLevel?: (row: T) => number
}

export function DataTable<T>({
  columns,
  data,
  rowKey,
  loading = false,
  pagination,
  onPageChange,
  onSizeChange,
  emptyText = '暂无数据',
  selectable = false,
  selectedKeys,
  onSelectionChange,
  sort,
  onSortChange,
  header,
  footer,
  expandedRowKeys,
  renderExpanded,
  getChildren,
  hasChildren,
  expandedKeys,
  onToggleExpand,
  getLevel,
}: DataTableProps<T>) {
  const safeData = Array.isArray(data) ? data : []
  const allKeys = useMemo(() => safeData.map((r) => rowKey(r)), [safeData, rowKey])
  const allSelected = useMemo(
    () => allKeys.length > 0 && selectedKeys != null && allKeys.every((k) => selectedKeys.has(k)),
    [allKeys, selectedKeys],
  )

  const toggleAll = useCallback(() => {
    if (!onSelectionChange) return
    if (allSelected) {
      const next = new Set(selectedKeys)
      allKeys.forEach((k) => next.delete(k))
      onSelectionChange(next)
    } else {
      const next = new Set(selectedKeys)
      allKeys.forEach((k) => next.add(k))
      onSelectionChange(next)
    }
  }, [allSelected, allKeys, selectedKeys, onSelectionChange])

  const toggleRow = useCallback(
    (key: string | number) => {
      if (!onSelectionChange || !selectedKeys) return
      const next = new Set(selectedKeys)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      onSelectionChange(next)
    },
    [selectedKeys, onSelectionChange],
  )

  const colSpan = columns.length + (selectable ? 1 : 0)

  const effectiveExpandedKeys = useMemo(() => {
    if (expandedKeys) return expandedKeys
    if (expandedRowKeys && expandedRowKeys.length > 0) return new Set(expandedRowKeys)
    return null
  }, [expandedKeys, expandedRowKeys])

  const renderRow = (row: T, idx: number, level: number): ReactNode[] => {
    const key = rowKey(row)
    const checked = selectedKeys?.has(key) ?? false
    const isExpanded = effectiveExpandedKeys?.has(key) ?? false
    const canExpand = hasChildren ? hasChildren(row) : false
    const rows: ReactNode[] = [
      <tr key={key} className={checked ? 'data-table__row--selected' : ''}>
        {selectable && (
          <td>
            <input
              type="checkbox"
              className="data-table__checkbox"
              checked={checked}
              onChange={() => toggleRow(key)}
            />
          </td>
        )}
        {columns.map((col) => (
          <td key={col.key} className={col.key === 'action' ? 'data-table__td--sticky-right' : ''}>
            {col.render
              ? col.render(row, idx, { level, expanded: isExpanded, canExpand, onToggleExpand })
              : String((row as Record<string, unknown>)[col.key] ?? '')}
          </td>
        ))}
      </tr>,
    ]
    if (isExpanded && renderExpanded) {
      rows.push(
        <tr key={`${key}-expanded`} className="data-table__row--expanded">
          <td colSpan={colSpan} style={{ padding: 0, border: 'none' }}>
            {renderExpanded(row)}
          </td>
        </tr>,
      )
    }
    if (isExpanded && getChildren && canExpand) {
      const children = getChildren(row)
      children.forEach((child, childIdx) => {
        rows.push(...renderRow(child, childIdx, level + 1))
      })
    }
    return rows
  }

  return (
    <div className="data-table-wrapper">
      {header && <div className="data-table-header">{header}</div>}
      <div className="data-table-body">
      <div className="data-table-scroll">
        <table className="data-table">
          <thead>
            <tr>
              {selectable && (
                <th style={{ width: 40 }}>
                  <input
                    type="checkbox"
                    className="data-table__checkbox"
                    checked={allSelected}
                    onChange={toggleAll}
                  />
                </th>
              )}
              {columns.map((col) => (
                <th
                  key={col.key}
                  style={col.width ? { width: col.width, minWidth: col.width } : undefined}
                  className={`${col.sortable ? 'data-table__th--sortable' : ''} ${col.key === 'action' ? 'data-table__th--sticky-right' : ''}`}
                  onClick={col.sortable && onSortChange ? () => {
                    if (sort?.field === col.key) {
                      onSortChange({ field: col.key, order: sort.order === 'asc' ? 'desc' : 'asc' })
                    } else {
                      onSortChange({ field: col.key, order: 'asc' })
                    }
                  } : undefined}
                >
                  <span className="data-table__th-content">
                    {col.title}
                    {col.sortable && (
                      <span className="data-table__sort-icon">
                        {sort?.field === col.key
                          ? sort.order === 'asc'
                            ? <ArrowUp size={14} />
                            : <ArrowDown size={14} />
                          : <ArrowUpDown size={14} />}
                      </span>
                    )}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={colSpan}>
                  <div className="data-table__loading">
                    <Loader2 size={20} className="data-table__spinner" />
                    <span>加载中...</span>
                  </div>
                </td>
              </tr>
            ) : safeData.length === 0 ? (
              <tr>
                <td colSpan={colSpan}>
                  <div className="data-table__empty">{emptyText}</div>
                </td>
              </tr>
            ) : (
              safeData.flatMap((row, idx) => renderRow(row, idx, getLevel ? getLevel(row) : 0))
            )}
          </tbody>
        </table>
      </div>

      </div>
      {footer && <div className="data-table-footer">{footer}</div>}
      {pagination && pagination.total > 0 && (
        <Pagination
          page={pagination.page}
          size={pagination.size}
          total={pagination.total}
          onPageChange={onPageChange}
          onSizeChange={onSizeChange}
        />
      )}
    </div>
  )
}
