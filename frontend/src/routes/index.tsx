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
import TasksPage from '@/pages/admin/TasksPage'
import NetworkPage from '@/pages/admin/NetworkPage'
import StoragePage from '@/pages/admin/StoragePage'
import SecurityPage from '@/pages/admin/SecurityPage'
import SecurityPlaceholderPage from '@/pages/admin/SecurityPlaceholderPage'
import SettingsPage from '@/pages/admin/SettingsPage'

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
      .catch(() => setInitialized(true))
  }, [])

  if (initialized === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="mx-auto mb-4 h-8 w-8 animate-spin rounded-full border-2 border-[#087ed1] border-t-transparent" />
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
          <Route index element={<Navigate to="/admin/systemOverview" replace />} />
          <Route path="systemOverview" element={<DashboardPage />} />
          <Route path="hostManagement/nodes" element={<NodesPage />} />
          <Route path="hostManagement/images" element={<ImagesPage />} />
          <Route path="hostManagement/network" element={<NetworkPage />} />
          <Route path="hostManagement/storage" element={<StoragePage />} />
          <Route path="hostManagement/tasks" element={<TasksPage />} />
          <Route path="instanceManagement/vm" element={<InstancesPage instanceType="vm" />} />
          <Route path="instanceManagement/container" element={<InstancesPage instanceType="container" />} />
          <Route path="instanceManagement/instances/:id" element={<InstanceDetailPage />} />
          <Route path="securityManagement/security" element={<SecurityPage />} />
          <Route path="securityManagement/firewall" element={<SecurityPlaceholderPage titleKey="nav.firewallManagement" />} />
          <Route path="securityManagement/acl" element={<SecurityPlaceholderPage titleKey="nav.aclRules" />} />
          <Route path="securityManagement/url-filter" element={<SecurityPlaceholderPage titleKey="nav.urlFilter" />} />
          <Route path="systemManagement/settings" element={<SettingsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/admin" replace />} />
      </Routes>
    </InitGuard>
  )
}
