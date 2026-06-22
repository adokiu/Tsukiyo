import { useEffect, useState } from 'react'
import apiClient from '@/api/client'
import { Button } from '@/components/Button/Button'
import { Select } from '@/components/Select/Select'
import { SlidePanel } from '@/components/SlidePanel/SlidePanel'
import { useToastStore } from '@/stores/toast'
import type { EIPPool } from './types'

interface EIPPoolEditPanelProps {
  open: boolean
  pool: EIPPool | null
  onClose: () => void
  onSuccess: () => void
}

export function EIPPoolEditPanel({ open, pool, onClose, onSuccess }: EIPPoolEditPanelProps) {
  const toast = useToastStore()
  const [loading, setLoading] = useState(false)
  const [interfaceName, setInterfaceName] = useState('')
  const [gateway, setGateway] = useState('')
  const [alias, setAlias] = useState('')
  const [netmaskPrefix, setNetmaskPrefix] = useState(0)
  const [poolType, setPoolType] = useState('eip')
  const [status, setStatus] = useState('active')

  useEffect(() => {
    if (pool) {
      setInterfaceName(pool.interface || '')
      setGateway(pool.gateway || '')
      setAlias(pool.alias || '')
      setNetmaskPrefix(pool.netmask_prefix || 0)
      setPoolType(pool.pool_type || 'eip')
      setStatus(pool.status || 'active')
    }
  }, [pool])

  const handleSubmit = async () => {
    if (!pool) return
    setLoading(true)
    try {
      await apiClient.put(`/network/eip-pools/${pool.id}`, {
        interface: interfaceName,
        gateway,
        alias,
        netmask_prefix: netmaskPrefix,
        pool_type: poolType,
        status,
      })
      toast.success('EIP 池已更新')
      onSuccess()
      onClose()
    } catch (err: any) {
      toast.error(err.response?.data?.error || '操作失败')
    } finally {
      setLoading(false)
    }
  }

  if (!pool) return null

  return (
    <SlidePanel
      open={open}
      onClose={onClose}
      title={`编辑 EIP 池 - ${pool.cidr}`}
      width={500}
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>取消</Button>
          <Button loading={loading} onClick={handleSubmit}>保存</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">CIDR</label>
          <input value={pool.cidr} disabled className="w-full px-3 py-2 border border-surface-strong rounded text-sm bg-surface-secondary text-tertiary" />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">IP 版本</label>
          <input value={pool.ip_version} disabled className="w-full px-3 py-2 border border-surface-strong rounded text-sm bg-surface-secondary text-tertiary" />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">网卡</label>
          <input value={interfaceName} onChange={(e) => setInterfaceName(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 eth0" />
        </div>
        {pool.ip_version === 'ipv4' && (
          <div>
            <label className="block text-sm font-medium text-secondary mb-1">掩码前缀</label>
            <input type="number" value={netmaskPrefix} onChange={(e) => setNetmaskPrefix(parseInt(e.target.value) || 0)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 24" />
          </div>
        )}
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">网关</label>
          <input value={gateway} onChange={(e) => setGateway(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 172.18.10.254" />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">别名 (公网 CIDR)</label>
          <input value={alias} onChange={(e) => setAlias(e.target.value)} className="w-full px-3 py-2 border border-surface-strong rounded text-sm" placeholder="如 125.25.1.10/30，IP 数量需与池一致" />
          <p className="text-xs text-muted mt-1">DMZ 场景下对应的公网 IP 段，前缀长度必须与池 CIDR 相同</p>
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">类型</label>
          <Select
            value={poolType}
            options={[
              { label: '宿主机', value: 'host' },
              { label: '弹性', value: 'eip' },
            ]}
            onChange={(v) => setPoolType(v as string)}
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-secondary mb-1">状态</label>
          <Select
            value={status}
            options={[
              { label: '启用', value: 'active' },
              { label: '禁用', value: 'disabled' },
            ]}
            onChange={(v) => setStatus(v as string)}
          />
        </div>
      </div>
    </SlidePanel>
  )
}
