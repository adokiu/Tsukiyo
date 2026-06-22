import { useEffect, useState } from 'react'
import { Users, Trash2 } from 'lucide-react'
import apiClient from '@/api/client'
import { DataTable, type Column } from '@/components/DataTable/DataTable'
import { Button } from '@/components/Button/Button'
import { useToastStore } from '@/stores/toast'

interface User {
  id: number
  username: string
  email: string
  status: string
  created_at: string
}

export default function UsersPage() {
  const toast = useToastStore()
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)

  const fetchUsers = () => {
    setLoading(true)
    apiClient.get('/users').then((res) => setUsers(res.data.data || [])).finally(() => setLoading(false))
  }

  useEffect(() => { fetchUsers() }, [])

  const handleDelete = async (id: number) => {
    if (!confirm('确认删除该用户？')) return
    await apiClient.delete(`/users/${id}`)
    toast.success('用户删除成功')
    fetchUsers()
  }

  const columns: Column<User>[] = [
    { key: 'username', title: '用户名' },
    { key: 'email', title: '邮箱' },
    { key: 'status', title: '状态' },
    { key: 'created_at', title: '创建时间' },
    {
      key: 'action',
      title: '操作',
      render: (row: User) => (
        <button className="text-red-500 hover:text-red-700" onClick={() => handleDelete(row.id)}>
          <Trash2 size={16} />
        </button>
      ),
    },
  ]

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Users size={22} className="text-primary" />
          <h1 className="text-xl font-semibold text-primary">用户管理</h1>
        </div>
        <Button onClick={() => toast.info('新建用户功能待完善')}>新建用户</Button>
      </div>

      <DataTable columns={columns} data={users} rowKey={(r) => r.id} loading={loading} />
    </div>
  )
}
