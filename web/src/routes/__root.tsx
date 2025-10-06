import { createRootRoute, Outlet, useNavigate, useLocation } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/router-devtools'
import { SidebarProvider, SidebarInset } from '@/components/ui/sidebar'
import { AppSidebar } from '@/components/app-sidebar'
import { useEffect, useState } from 'react'
import { getBootstrap } from '@/lib/api'
import { AuthProvider, useAuth } from '@/lib/auth'

export const Route = createRootRoute({
  component: RootComponent,
})

function RootComponent() {
  return (
    <AuthProvider>
      <RootLayout />
    </AuthProvider>
  )
}

function RootLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { isAuthenticated, loading: authLoading } = useAuth()
  const [isChecking, setIsChecking] = useState(true)

  useEffect(() => {
    async function checkBootstrap() {
      try {
        const response = await getBootstrap()
        if (response.data && !response.data.initialized) {
          // System not initialized, redirect to onboarding
          navigate({ to: '/onboarding' })
        } else if (!isAuthenticated && location.pathname !== '/login' && location.pathname !== '/onboarding') {
          // Not authenticated and not on login/onboarding page
          navigate({ to: '/login' })
        }
      } catch (error) {
        console.error('Failed to check bootstrap:', error)
      } finally {
        setIsChecking(false)
      }
    }

    if (!authLoading) {
      checkBootstrap()
    }
  }, [navigate, isAuthenticated, authLoading, location.pathname])

  if (isChecking || authLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    )
  }

  // Don't render sidebar for login/signup/onboarding pages
  if (location.pathname === '/login' || location.pathname === '/signup' || location.pathname === '/onboarding') {
    return (
      <>
        <Outlet />
        <TanStackRouterDevtools position="bottom-right" />
      </>
    )
  }

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset>
        <main className="flex flex-1 flex-col gap-4 p-4">
          <Outlet />
        </main>
        <TanStackRouterDevtools position="bottom-right" />
      </SidebarInset>
    </SidebarProvider>
  )
}
