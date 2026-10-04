import apiClient from './client'

export interface TicketReply {
  id: string
  ticket_id: string
  user_id: number
  user?: {
    id: number
    username: string
    email: string
  }
  content: string
  is_staff: boolean
  created_at: string
  updated_at: string
}

export interface Ticket {
  id: string
  ticket_no: string
  user_id: number
  user?: {
    id: number
    username: string
    email: string
  }
  subject: string
  content: string
  type: string
  priority: string
  status: string
  product_id?: string
  product?: {
    id: string
    name: string
  }
  instance_id?: string
  instance?: {
    id: string
    name: string
  }
  assignee_id?: number
  assignee?: {
    id: number
    username: string
  }
  replies?: TicketReply[]
  closed_at?: string
  created_at: string
  updated_at: string
}

export interface CreateTicketRequest {
  subject: string
  content: string
  type: string
  priority: string
  product_id?: string
  instance_id?: string
}

export interface TicketListResponse {
  data: Ticket[]
  pagination: {
    page: number
    page_size: number
    total: number
  }
}

export const ticketsApi = {
  list: (params?: { page?: number; page_size?: number; status?: string }) =>
    apiClient.get<TicketListResponse>('/user/tickets', { params }),
  get: (id: string) =>
    apiClient.get<Ticket>(`/user/tickets/${id}`),
  create: (req: CreateTicketRequest) =>
    apiClient.post<Ticket>('/user/tickets', req),
  reply: (id: string, content: string) =>
    apiClient.post<TicketReply>(`/user/tickets/${id}/reply`, { content }),
  close: (id: string) =>
    apiClient.post(`/user/tickets/${id}/close`),
}
