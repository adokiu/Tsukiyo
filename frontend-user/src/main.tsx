import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from '@/components/ThemeProvider'
import { router } from '@/router'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'
import '@/i18n'
import '@/styles/main.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})

// 初始化时获取公开设置
useAppStore.getState().fetchPublicSettings()

// 已登录时刷新用户信息 (获取最新余额等)
const { token, setUser } = useAuthStore.getState()
if (token) {
  useAuthStore.setState({ isAuthenticated: true })
  authApi.me().then((res) => setUser(res.data)).catch(() => {})
}

createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={queryClient}>
    <ThemeProvider>
      <RouterProvider router={router} />
    </ThemeProvider>
  </QueryClientProvider>
)
