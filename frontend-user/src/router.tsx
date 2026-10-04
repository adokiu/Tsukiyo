import { createBrowserRouter, Navigate, type RouteObject } from 'react-router-dom'
import { AppLayout } from '@/components/AppLayout'
import { ShopLayout } from '@/components/ShopLayout'
import { LandingView } from '@/views/LandingView'
import { HomeView } from '@/views/HomeView'
import { InstancesView } from '@/views/InstancesView'
import { InstanceDetailView } from '@/views/InstanceDetailView'
import { ProfileView } from '@/views/ProfileView'
import { WalletView } from '@/views/WalletView'
import { ConsoleView } from '@/views/ConsoleView'
import { ProductsView } from '@/views/ProductsView'
import { CartView } from '@/views/CartView'
import { CheckoutView } from '@/views/CheckoutView'
import { PaymentView } from '@/views/PaymentView'
import { BillsView } from '@/views/BillsView'
import { InvoiceView } from '@/views/InvoiceView'
import { TicketsView } from '@/views/TicketsView'
import { TicketDetailView } from '@/views/TicketDetailView'
import { useAuthStore } from '@/stores/auth'
import type { JSX } from 'react'

function ProtectedRoute({ children }: { children: JSX.Element }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  return children
}

const routes: RouteObject[] = [
  {
    path: '/login',
    element: <LandingView mode="login" />,
  },
  {
    path: '/register',
    element: <LandingView mode="register" />,
  },
  {
    path: '/console',
    element: <ConsoleView />,
  },
  {
    path: '/products',
    element: (
      <ShopLayout>
        <ProductsView />
      </ShopLayout>
    ),
  },
  {
    path: '/cart',
    element: (
      <ProtectedRoute>
        <ShopLayout>
          <CartView />
        </ShopLayout>
      </ProtectedRoute>
    ),
  },
  {
    path: '/checkout',
    element: (
      <ProtectedRoute>
        <ShopLayout>
          <CheckoutView />
        </ShopLayout>
      </ProtectedRoute>
    ),
  },
  {
    path: '/',
    element: <LandingView />,
  },
  {
    element: <AppLayout />,
    children: [
      {
        path: '/dashboard',
        element: <ProtectedRoute><HomeView /></ProtectedRoute>,
      },
      {
        path: '/instances',
        element: <ProtectedRoute><InstancesView /></ProtectedRoute>,
      },
      {
        path: '/instances/:id',
        element: <ProtectedRoute><InstanceDetailView /></ProtectedRoute>,
      },
      {
        path: '/profile',
        element: <ProtectedRoute><ProfileView /></ProtectedRoute>,
      },
      {
        path: '/wallet',
        element: <ProtectedRoute><WalletView /></ProtectedRoute>,
      },
      {
        path: '/payment/:id',
        element: <ProtectedRoute><PaymentView /></ProtectedRoute>,
      },
      {
        path: '/bills',
        element: <ProtectedRoute><BillsView /></ProtectedRoute>,
      },
      {
        path: '/invoice/:id',
        element: <ProtectedRoute><InvoiceView /></ProtectedRoute>,
      },
      {
        path: '/tickets',
        element: <ProtectedRoute><TicketsView /></ProtectedRoute>,
      },
      {
        path: '/tickets/:id',
        element: <ProtectedRoute><TicketDetailView /></ProtectedRoute>,
      },
    ],
  },
  {
    path: '*',
    element: <Navigate to="/" replace />,
  },
]

export const router = createBrowserRouter(routes)
