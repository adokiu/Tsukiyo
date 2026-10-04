import axios from 'axios'
import { useAuthStore } from '@/stores/auth'

export const apiClient = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

apiClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => {
    const body = response.data
    if (body && typeof body === 'object' && (body.code === 401 || body.code === 403)) {
      if (body.code === 401) {
        useAuthStore.getState().logout()
        window.location.href = '/login'
      }
      return Promise.reject({ response: { status: body.code, data: body } })
    }
    return response
  },
  (error) => {
    return Promise.reject(error)
  }
)

export default apiClient
