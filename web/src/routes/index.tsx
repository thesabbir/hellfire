import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { Activity, Server, Wifi, Shield, RefreshCw } from 'lucide-react'
import { getConfigByName, getChanges } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card as UICard, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export const Route = createFileRoute('/')({
  component: Dashboard,
})

function Dashboard() {
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [stats, setStats] = useState({
    networkInterfaces: 0,
    dhcpLeases: 0,
    firewallRules: 0,
    pendingChanges: 0,
  })

  useEffect(() => {
    loadStats()
  }, [])

  async function loadStats() {
    try {
      setRefreshing(true)

      // Load network interfaces
      const networkResponse = await getConfigByName({ path: { name: 'network' } })
      const networkCount = networkResponse.data
        ? Object.values(networkResponse.data).filter((v) => (v as Record<string, unknown>)['.type'] === 'interface').length
        : 0

      // Load DHCP leases
      const dhcpResponse = await getConfigByName({ path: { name: 'dhcp' } })
      const dhcpCount = dhcpResponse.data
        ? Object.values(dhcpResponse.data).filter((v) => (v as Record<string, unknown>)['.type'] === 'host').length
        : 0

      // Load firewall rules
      const firewallResponse = await getConfigByName({ path: { name: 'firewall' } })
      const firewallCount = firewallResponse.data
        ? Object.values(firewallResponse.data).filter((v) => (v as Record<string, unknown>)['.type'] === 'rule').length
        : 0

      // Load pending changes
      const changesResponse = await getChanges()
      const changesCount = changesResponse.data
        ? Object.keys(changesResponse.data).length
        : 0

      setStats({
        networkInterfaces: networkCount,
        dhcpLeases: dhcpCount,
        firewallRules: firewallCount,
        pendingChanges: changesCount,
      })
    } catch (error) {
      console.error('Failed to load stats:', error)
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold">Dashboard</h2>
          <p className="text-muted-foreground">
            Router configuration and monitoring
          </p>
        </div>
        <Button onClick={loadStats} disabled={refreshing} variant="outline">
          <RefreshCw className={`w-4 h-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <Card
          title="System Status"
          value="Online"
          icon={<Server className="w-6 h-6" />}
          className="bg-green-50 dark:bg-green-950"
          loading={loading}
        />
        <Card
          title="Network Interfaces"
          value={loading ? '...' : stats.networkInterfaces.toString()}
          icon={<Activity className="w-6 h-6" />}
          className="bg-blue-50 dark:bg-blue-950"
          loading={loading}
        />
        <Card
          title="DHCP Leases"
          value={loading ? '...' : `${stats.dhcpLeases} Static`}
          icon={<Wifi className="w-6 h-6" />}
          className="bg-purple-50 dark:bg-purple-950"
          loading={loading}
        />
        <Card
          title="Firewall Rules"
          value={loading ? '...' : stats.firewallRules.toString()}
          icon={<Shield className="w-6 h-6" />}
          className="bg-orange-50 dark:bg-orange-950"
          loading={loading}
        />
      </div>

      {stats.pendingChanges > 0 && (
        <UICard className="border-yellow-500 bg-yellow-50 dark:bg-yellow-950/20">
          <CardHeader>
            <CardTitle className="text-yellow-700 dark:text-yellow-400">
              Pending Changes
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm">
              You have {stats.pendingChanges} pending change{stats.pendingChanges !== 1 ? 's' : ''} that need to be committed.
              Navigate to the respective configuration pages to review and commit your changes.
            </p>
          </CardContent>
        </UICard>
      )}

      <UICard>
        <CardHeader>
          <CardTitle>Quick Start</CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="space-y-2 text-muted-foreground">
            <li>• Navigate to Network to configure interfaces</li>
            <li>• Set up firewall rules in the Firewall section</li>
            <li>• Configure DHCP server settings in DHCP</li>
            <li>• Adjust system settings in System</li>
          </ul>
        </CardContent>
      </UICard>
    </div>
  )
}

interface CardProps {
  title: string
  value: string
  icon: React.ReactNode
  className?: string
  loading?: boolean
}

function Card({ title, value, icon, className, loading }: CardProps) {
  return (
    <div className={`rounded-lg border p-6 ${className}`}>
      <div className="flex items-center justify-between mb-4">
        <div className="text-sm font-medium text-muted-foreground">{title}</div>
        <div className="text-primary">{icon}</div>
      </div>
      <div className="text-2xl font-bold">
        {loading ? <div className="animate-pulse">...</div> : value}
      </div>
    </div>
  )
}
