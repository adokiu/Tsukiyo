import { useState, useEffect, useCallback, useRef } from 'react'
import apiClient from '@/api/client'
import { useWebSocket } from './useWebSocket'

export interface ListQueryState {
  page: number
  perPage: number
  search: string
  filters: Record<string, string>
}

export interface ListQueryOptions {
  defaultPerPage?: number
  wsUrl?: string
  wsType?: string
  wsUpdate?: (data: any[], msg: any) => any[]
  wsRefreshTypes?: string[]
}

export interface ListQueryResult<T> {
  data: T[]
  total: number
  loading: boolean
  page: number
  perPage: number
  search: string
  filters: Record<string, string>
  setPage: (page: number) => void
  setPerPage: (size: number) => void
  setSearch: (value: string) => void
  setFilter: (key: string, value: string) => void
  refresh: () => void
}

export function useListQuery<T>(
  url: string,
  extraParams: Record<string, string | undefined> = {},
  options: ListQueryOptions = {}
): ListQueryResult<T> {
  const defaultPerPage = options.defaultPerPage || 20
  const [data, setData] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [perPage, setPerPageState] = useState(defaultPerPage)
  const [search, setSearchState] = useState('')
  const [filters, setFilters] = useState<Record<string, string>>({})
  const [refreshKey, setRefreshKey] = useState(0)
  const firstLoad = useRef(true)

  const refresh = useCallback(() => {
    setRefreshKey((k) => k + 1)
  }, [])

  const setPageWrapper = useCallback((p: number) => {
    setPage(p)
  }, [])

  const setPerPage = useCallback((size: number) => {
    setPerPageState(size)
    setPage(1)
  }, [])

  const setSearch = useCallback((value: string) => {
    setSearchState(value)
    setPage(1)
  }, [])

  const setFilter = useCallback((key: string, value: string) => {
    setFilters((prev) => {
      const next = { ...prev }
      if (value) {
        next[key] = value
      } else {
        delete next[key]
      }
      return next
    })
    setPage(1)
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    const fetchData = async () => {
      if (firstLoad.current) {
        setLoading(true)
      }
      try {
        const params: Record<string, string | number> = {
          page,
          per_page: perPage,
        }
        if (search) params.search = search
        for (const [k, v] of Object.entries(filters)) {
          if (v) params[`filter_${k}`] = v
        }
        for (const [k, v] of Object.entries(extraParams)) {
          if (v) params[k] = v
        }
        const res = await apiClient.get(url, { params, signal: controller.signal })
        setData(res.data.data || [])
        setTotal(res.data.total || 0)
      } catch (err: any) {
        if (err.name !== 'CanceledError') {
          console.error('查询失败:', err)
        }
      } finally {
        setLoading(false)
        firstLoad.current = false
      }
    }
    fetchData()
    return () => controller.abort()
  }, [url, page, perPage, search, filters, refreshKey, JSON.stringify(extraParams)])

  // WebSocket 局部更新 / 刷新通知
  const wsRefreshTypes = options.wsRefreshTypes || ['data_refresh']
  const wsUpdateRef = useRef(options.wsUpdate)
  useEffect(() => {
    wsUpdateRef.current = options.wsUpdate
  }, [options.wsUpdate])

  const refreshRef = useRef(refresh)
  useEffect(() => {
    refreshRef.current = refresh
  }, [refresh])

  const handleWsMessage = useCallback((msg: any) => {
    if (!msg.type) return

    // 局部更新：调用 wsUpdate 对当前 data 做就地修改
    if (options.wsType && msg.type === options.wsType && wsUpdateRef.current) {
      setData((prev) => wsUpdateRef.current!(prev, msg))
      return
    }

    // 刷新通知：收到指定类型消息时触发 refresh
    if (wsRefreshTypes.includes(msg.type)) {
      refreshRef.current()
      return
    }
  }, [options.wsType, wsRefreshTypes.join(','), refresh])

  useWebSocket({
    url: options.wsUrl || '',
    onMessage: handleWsMessage,
    enabled: !!(options.wsUrl),
  })

  return {
    data,
    total,
    loading,
    page,
    perPage,
    search,
    filters,
    setPage: setPageWrapper,
    setPerPage,
    setSearch,
    setFilter,
    refresh,
  }
}
