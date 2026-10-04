import apiClient from './client'

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  token: string
  expires_at: string
  user: {
    id: string
    username: string
    email: string
    status: string
    balance_cents: number
  }
}

export interface RegisterRequest {
  username: string
  email: string
  password: string
  code: string
}

export const authApi = {
  login: (data: LoginRequest) => apiClient.post<LoginResponse>('/auth/login', data),
  register: (data: RegisterRequest) => apiClient.post('/auth/register', data),
  sendRegisterCode: (email: string) => apiClient.post('/auth/register/send-code', { email }),
  logout: () => apiClient.post('/auth/logout'),
  me: () => apiClient.get('/auth/me'),
  changePassword: (data: { old_password: string; new_password: string }) =>
    apiClient.post('/auth/change-password', data),
}
