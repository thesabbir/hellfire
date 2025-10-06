import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field, FieldLabel, FieldDescription } from '@/components/ui/field'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { getConfigByName, postCommit, putConfigByNameBySectionByOption, getAuthCsrf } from '@/lib/api'
import { Wifi, Plus, Trash2, Save, AlertCircle, Info } from 'lucide-react'

export const Route = createFileRoute('/dhcp')({
  component: DHCP,
})

interface DHCPPool {
  interface: string
  start: string
  limit: string
  leasetime: string
  dhcpv6: string
  ra: string
}

interface DHCPLease {
  name: string
  mac: string
  ip: string
  hostname: string
}

function DHCP() {
  const [pools, setPools] = useState<DHCPPool[]>([])
  const [leases, setLeases] = useState<DHCPLease[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string>('')
  const [csrfToken, setCsrfToken] = useState<string>('')
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)

  useEffect(() => {
    loadDHCPConfig()
    loadCSRFToken()
  }, [])

  async function loadCSRFToken() {
    try {
      const response = await getAuthCsrf()
      if (response.data && typeof response.data === 'object' && 'csrf_token' in response.data) {
        setCsrfToken(response.data.csrf_token as string)
      }
    } catch (err) {
      console.error('Failed to load CSRF token:', err)
      setError('Failed to load CSRF token. Please refresh the page.')
    }
  }

  async function loadDHCPConfig() {
    try {
      setLoading(true)
      const response = await getConfigByName({ path: { name: 'dhcp' } })
      if (response.data) {
        const config = response.data as Record<string, unknown>
        const parsedPools: DHCPPool[] = []
        const parsedLeases: DHCPLease[] = []

        Object.entries(config).forEach(([key, value]) => {
          const section = value as Record<string, unknown>
          if (section['.type'] === 'dhcp') {
            parsedPools.push({
              interface: (section.interface as string) || key,
              start: (section.start as string) || '100',
              limit: (section.limit as string) || '150',
              leasetime: (section.leasetime as string) || '12h',
              dhcpv6: (section.dhcpv6 as string) || 'server',
              ra: (section.ra as string) || 'server',
            })
          } else if (section['.type'] === 'host') {
            parsedLeases.push({
              name: (section.name as string) || key,
              mac: (section.mac as string) || '',
              ip: (section.ip as string) || '',
              hostname: (section.hostname as string) || '',
            })
          }
        })

        setPools(parsedPools)
        setLeases(parsedLeases)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load DHCP config')
    } finally {
      setLoading(false)
    }
  }

  async function savePoolToAPI(pool: DHCPPool, poolName: string) {
    if (!csrfToken) throw new Error('CSRF token not loaded')

    const fields: Array<keyof DHCPPool> = ['interface', 'start', 'limit', 'leasetime', 'dhcpv6', 'ra']
    for (const field of fields) {
      const value = pool[field]
      if (value) {
        await putConfigByNameBySectionByOption({
          path: { name: 'dhcp', section: poolName, option: field },
          body: { value: String(value) },
          headers: { 'X-CSRF-Token': csrfToken },
        })
      }
    }
  }

  async function saveLeaseToAPI(lease: DHCPLease, leaseName: string) {
    if (!csrfToken) throw new Error('CSRF token not loaded')

    const fields: Array<keyof DHCPLease> = ['name', 'mac', 'ip', 'hostname']
    for (const field of fields) {
      const value = lease[field]
      if (value) {
        await putConfigByNameBySectionByOption({
          path: { name: 'dhcp', section: leaseName, option: field },
          body: { value: String(value) },
          headers: { 'X-CSRF-Token': csrfToken },
        })
      }
    }
  }

  async function handleCommit() {
    try {
      setSaving(true)
      setError('')

      // Save all pools to the API
      for (let i = 0; i < pools.length; i++) {
        const pool = pools[i]
        const poolName = pool.interface || `pool_${i}`
        await savePoolToAPI(pool, poolName)
      }

      // Save all leases to the API
      for (let i = 0; i < leases.length; i++) {
        const lease = leases[i]
        const leaseName = lease.name || `host_${i}`
        await saveLeaseToAPI(lease, leaseName)
      }

      // Commit the staged changes
      await postCommit({
        headers: { 'X-CSRF-Token': csrfToken },
      })

      setHasUnsavedChanges(false)
      alert('DHCP configuration committed successfully')
      await loadDHCPConfig()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to commit changes')
    } finally {
      setSaving(false)
    }
  }

  function addLease() {
    setLeases([...leases, {
      name: `host_${leases.length + 1}`,
      mac: '',
      ip: '',
      hostname: '',
    }])
    setHasUnsavedChanges(true)
  }

  function removeLease(index: number) {
    setLeases(leases.filter((_, i) => i !== index))
    setHasUnsavedChanges(true)
  }

  function updateLease(index: number, field: keyof DHCPLease, value: string) {
    const updated = [...leases]
    updated[index] = { ...updated[index], [field]: value }
    setLeases(updated)
    setHasUnsavedChanges(true)
  }

  function updatePool(index: number, field: keyof DHCPPool, value: string) {
    const updated = [...pools]
    updated[index] = { ...updated[index], [field]: value }
    setPools(updated)
    setHasUnsavedChanges(true)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-muted-foreground">Loading DHCP configuration...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold flex items-center gap-2">
            <Wifi className="w-8 h-8" />
            DHCP Configuration
          </h2>
          <p className="text-muted-foreground">
            Manage DHCP server settings and static leases
          </p>
        </div>
        <Button onClick={handleCommit} disabled={saving}>
          <Save className="w-4 h-4 mr-2" />
          {saving ? 'Committing...' : 'Commit Changes'}
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {hasUnsavedChanges && (
        <Alert>
          <Info className="h-4 w-4" />
          <AlertDescription>
            You have unsaved changes. Click "Commit Changes" to save them to the system.
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle>DHCP Pools</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {pools.map((pool, index) => (
              <div key={index} className="border rounded-lg p-4">
                <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                  <Field>
                    <FieldLabel>Interface</FieldLabel>
                    <Input
                      value={pool.interface}
                      onChange={(e) => updatePool(index, 'interface', e.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Start Address</FieldLabel>
                    <Input
                      value={pool.start}
                      onChange={(e) => updatePool(index, 'start', e.target.value)}
                      placeholder="100"
                    />
                    <FieldDescription>
                      Starting IP offset
                    </FieldDescription>
                  </Field>
                  <Field>
                    <FieldLabel>Limit</FieldLabel>
                    <Input
                      value={pool.limit}
                      onChange={(e) => updatePool(index, 'limit', e.target.value)}
                      placeholder="150"
                    />
                    <FieldDescription>
                      Maximum number of leases
                    </FieldDescription>
                  </Field>
                  <Field>
                    <FieldLabel>Lease Time</FieldLabel>
                    <Input
                      value={pool.leasetime}
                      onChange={(e) => updatePool(index, 'leasetime', e.target.value)}
                      placeholder="12h"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>DHCPv6 Mode</FieldLabel>
                    <Input
                      value={pool.dhcpv6}
                      onChange={(e) => updatePool(index, 'dhcpv6', e.target.value)}
                      placeholder="server"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Router Advertisement</FieldLabel>
                    <Input
                      value={pool.ra}
                      onChange={(e) => updatePool(index, 'ra', e.target.value)}
                      placeholder="server"
                    />
                  </Field>
                </div>
              </div>
            ))}
            {pools.length === 0 && (
              <div className="text-center text-muted-foreground py-8">
                No DHCP pools configured.
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Static Leases</CardTitle>
            <Button onClick={addLease} size="sm">
              <Plus className="w-4 h-4 mr-2" />
              Add Static Lease
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {leases.map((lease, index) => (
              <div key={index} className="border rounded-lg p-4">
                <div className="flex items-start justify-between mb-4">
                  <h4 className="font-semibold">Static Lease {index + 1}</h4>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => removeLease(index)}
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <Field>
                    <FieldLabel>Name</FieldLabel>
                    <Input
                      value={lease.name}
                      onChange={(e) => updateLease(index, 'name', e.target.value)}
                      placeholder="device1"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>MAC Address</FieldLabel>
                    <Input
                      value={lease.mac}
                      onChange={(e) => updateLease(index, 'mac', e.target.value)}
                      placeholder="00:11:22:33:44:55"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>IP Address</FieldLabel>
                    <Input
                      value={lease.ip}
                      onChange={(e) => updateLease(index, 'ip', e.target.value)}
                      placeholder="192.168.1.100"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Hostname</FieldLabel>
                    <Input
                      value={lease.hostname}
                      onChange={(e) => updateLease(index, 'hostname', e.target.value)}
                      placeholder="mydevice"
                    />
                  </Field>
                </div>
              </div>
            ))}
            {leases.length === 0 && (
              <div className="text-center text-muted-foreground py-8">
                No static leases configured. Click "Add Static Lease" to create one.
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
