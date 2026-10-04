import { create } from 'zustand'
import { cartApi, type CartItem } from '@/api/cart'

interface CartState {
  items: CartItem[]
  loading: boolean
  fetchCart: () => Promise<void>
  addToCart: (productId: string, config: Record<string, unknown>, quantity?: number) => Promise<void>
  updateQuantity: (itemId: string, quantity: number) => Promise<void>
  removeItem: (itemId: string) => Promise<void>
  clearCart: () => Promise<void>
  getTotalCents: () => number
  getCount: () => number
}

export const useCartStore = create<CartState>((set, get) => ({
  items: [],
  loading: false,

  fetchCart: async () => {
    set({ loading: true })
    try {
      const res = await cartApi.list()
      set({ items: res.data?.data || [], loading: false })
    } catch {
      set({ loading: false })
    }
  },

  addToCart: async (productId, config, quantity = 1) => {
    await cartApi.add({ product_id: productId, config, quantity })
    await get().fetchCart()
  },

  updateQuantity: async (itemId, quantity) => {
    await cartApi.update(itemId, { quantity })
    await get().fetchCart()
  },

  removeItem: async (itemId) => {
    await cartApi.remove(itemId)
    await get().fetchCart()
  },

  clearCart: async () => {
    await cartApi.clear()
    set({ items: [] })
  },

  getTotalCents: () => {
    return get().items.reduce((sum, item) => sum + item.unit_price_cents * item.quantity, 0)
  },

  getCount: () => {
    return get().items.reduce((sum, item) => sum + item.quantity, 0)
  },
}))
