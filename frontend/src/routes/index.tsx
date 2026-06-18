import { useEffect, useState } from 'react'
import { Routes, Route, Navigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import apiClient from '@/api/client'
import InitPage from '@/pages/init/InitPage'
import LoginPage from '@/pages/auth/LoginPage'
import AppLayout from '@/layouts/AppLayout'
import DashboardPage from '@/pages/admin/DashboardPage'
import NodesPage from '@/pages/admin/NodesPage'
import InstancesPage from '@/pages/admin/InstancesPage'
import InstanceDetailPage from '@/pages/admin/InstanceDetailPage'
import ImagesPage from '@/pages/admin/ImagesPage'
import NetworkPage from '@/pages/admin/NetworkPage'
import SecurityPage from '@/pages/admin/SecurityPage'
import UsersPage from '@/pages/admin/UsersPage'
import AuditLogsPage from '@/pages/admin/AuditLogsPage'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />
}

function InitGuard({ children }: { children: React.ReactNode }) {
  const location = useLocation()
  const [initialized, setInitialized] = useState<boolean | null>(null)

  useEffect(() => {
    apiClient
      .get('/init/status')
      .then((res) => setInitialized(res.data.initialized))
      .catch(() => setInitialized(false))
  }, [])

  if (initialized === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-2 border-apple-blue border-t-transparent" />
        </div>
      </div>
    )
  }

  if (!initialized && location.pathname !== '/init') {
    return <Navigate to="/init" replace />
  }
  if (initialized && location.pathname === '/init') {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

export default function AppRoutes() {
  return (
    <InitGuard>
      <Routes>
        <Route path="/init" element={<InitPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route
          path="/admin"
          element={
            <ProtectedRoute>
              <AppLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Navigate to="/admin/dashboard" replace />} />
          <Route path="dashboard" element={<DashboardPage />} />
          <Route path="nodes" element={<NodesPage />} />
          <Route path="instances" element={<InstancesPage />} />
          <Route path="instances/:id" element={<InstanceDetailPage />} />
          <Route path="images" element={<ImagesPage />} />
          <Route path="network" element={<NetworkPage />} />
          <Route path="security" element={<SecurityPage />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="audit-logs" element={<AuditLogsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/admin" replace />} />
      </Routes>
    </InitGuard>
  )
}
