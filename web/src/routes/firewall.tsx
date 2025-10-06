import { createFileRoute } from '@tanstack/react-router'
import { useState, useEffect } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Field, FieldLabel, FieldDescription } from '@/components/ui/field'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { getConfigByName, postCommit, putConfigByNameBySectionByOption, getAuthCsrf } from '@/lib/api'
import { Shield, Plus, Trash2, Save, AlertCircle, Info } from 'lucide-react'

export const Route = createFileRoute('/firewall')({
  component: Firewall,
})

interface FirewallRule {
  name: string
  src: string
  dest: string
  proto: string
  src_port: string
  dest_port: string
  target: string
}

interface FirewallZone {
  name: string
  input: string
  output: string
  forward: string
  network: string[]
}

function Firewall() {
  const [zones, setZones] = useState<FirewallZone[]>([])
  const [rules, setRules] = useState<FirewallRule[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string>('')
  const [csrfToken, setCsrfToken] = useState<string>('')
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)

  useEffect(() => {
    loadFirewallConfig()
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

  async function loadFirewallConfig() {
    try {
      setLoading(true)
      const response = await getConfigByName({ path: { name: 'firewall' } })
      if (response.data) {
        // Parse firewall configuration
        const config = response.data as Record<string, unknown>
        const parsedZones: FirewallZone[] = []
        const parsedRules: FirewallRule[] = []

        // Parse zones and rules from config
        Object.entries(config).forEach(([key, value]) => {
          const section = value as Record<string, unknown>
          if (section['.type'] === 'zone') {
            parsedZones.push({
              name: (section.name as string) || key,
              input: (section.input as string) || 'ACCEPT',
              output: (section.output as string) || 'ACCEPT',
              forward: (section.forward as string) || 'REJECT',
              network: Array.isArray(section.network) ? section.network as string[] : section.network ? [section.network as string] : [],
            })
          } else if (section['.type'] === 'rule') {
            parsedRules.push({
              name: (section.name as string) || key,
              src: (section.src as string) || '*',
              dest: (section.dest as string) || '*',
              proto: (section.proto as string) || 'tcp',
              src_port: (section.src_port as string) || '',
              dest_port: (section.dest_port as string) || '',
              target: (section.target as string) || 'ACCEPT',
            })
          }
        })

        setZones(parsedZones)
        setRules(parsedRules)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load firewall config')
    } finally {
      setLoading(false)
    }
  }

  async function saveRuleToAPI(rule: FirewallRule, ruleName: string) {
    if (!csrfToken) {
      throw new Error('CSRF token not loaded')
    }

    // Save each field of the rule to the API
    const fields: Array<keyof FirewallRule> = ['name', 'src', 'dest', 'proto', 'src_port', 'dest_port', 'target']

    for (const field of fields) {
      const value = rule[field]
      if (value) {
        await putConfigByNameBySectionByOption({
          path: {
            name: 'firewall',
            section: ruleName,
            option: field
          },
          body: { value: String(value) },
          headers: {
            'X-CSRF-Token': csrfToken,
          },
        })
      }
    }
  }

  async function handleCommit() {
    try {
      setSaving(true)
      setError('')

      // Save all modified rules to the API first
      for (let i = 0; i < rules.length; i++) {
        const rule = rules[i]
        const ruleName = rule.name || `rule_${i}`
        await saveRuleToAPI(rule, ruleName)
      }

      // Now commit the staged changes
      await postCommit({
        headers: {
          'X-CSRF-Token': csrfToken,
        },
      })

      setHasUnsavedChanges(false)
      alert('Firewall configuration committed successfully')

      // Reload the config to show committed state
      await loadFirewallConfig()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to commit changes')
    } finally {
      setSaving(false)
    }
  }

  function addRule() {
    setRules([...rules, {
      name: `rule_${rules.length + 1}`,
      src: 'wan',
      dest: 'lan',
      proto: 'tcp',
      src_port: '',
      dest_port: '22',
      target: 'ACCEPT',
    }])
    setHasUnsavedChanges(true)
  }

  function removeRule(index: number) {
    setRules(rules.filter((_, i) => i !== index))
    setHasUnsavedChanges(true)
  }

  function updateRule(index: number, field: keyof FirewallRule, value: string) {
    const updated = [...rules]
    updated[index] = { ...updated[index], [field]: value }
    setRules(updated)
    setHasUnsavedChanges(true)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="text-muted-foreground">Loading firewall configuration...</div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-bold flex items-center gap-2">
            <Shield className="w-8 h-8" />
            Firewall Configuration
          </h2>
          <p className="text-muted-foreground">
            Manage firewall rules, zones, and port forwarding
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
          <CardTitle>Zones</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {zones.map((zone, index) => (
              <div key={index} className="border rounded-lg p-4">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <Field>
                    <FieldLabel>Zone Name</FieldLabel>
                    <Input value={zone.name} readOnly />
                  </Field>
                  <Field>
                    <FieldLabel>Input</FieldLabel>
                    <Input value={zone.input} readOnly />
                  </Field>
                  <Field>
                    <FieldLabel>Output</FieldLabel>
                    <Input value={zone.output} readOnly />
                  </Field>
                  <Field>
                    <FieldLabel>Forward</FieldLabel>
                    <Input value={zone.forward} readOnly />
                  </Field>
                </div>
                <Field className="mt-2">
                  <FieldLabel>Networks</FieldLabel>
                  <Input value={zone.network.join(', ')} readOnly />
                  <FieldDescription>
                    Networks associated with this zone
                  </FieldDescription>
                </Field>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Firewall Rules</CardTitle>
            <Button onClick={addRule} size="sm">
              <Plus className="w-4 h-4 mr-2" />
              Add Rule
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {rules.map((rule, index) => (
              <div key={index} className="border rounded-lg p-4">
                <div className="flex items-start justify-between mb-4">
                  <h4 className="font-semibold">Rule {index + 1}</h4>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => removeRule(index)}
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <Field>
                    <FieldLabel>Name</FieldLabel>
                    <Input
                      value={rule.name}
                      onChange={(e) => updateRule(index, 'name', e.target.value)}
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Source Zone</FieldLabel>
                    <Input
                      value={rule.src}
                      onChange={(e) => updateRule(index, 'src', e.target.value)}
                      placeholder="wan"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Destination Zone</FieldLabel>
                    <Input
                      value={rule.dest}
                      onChange={(e) => updateRule(index, 'dest', e.target.value)}
                      placeholder="lan"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Protocol</FieldLabel>
                    <Input
                      value={rule.proto}
                      onChange={(e) => updateRule(index, 'proto', e.target.value)}
                      placeholder="tcp"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Source Port</FieldLabel>
                    <Input
                      value={rule.src_port}
                      onChange={(e) => updateRule(index, 'src_port', e.target.value)}
                      placeholder="any"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Destination Port</FieldLabel>
                    <Input
                      value={rule.dest_port}
                      onChange={(e) => updateRule(index, 'dest_port', e.target.value)}
                      placeholder="80"
                    />
                  </Field>
                  <Field>
                    <FieldLabel>Target</FieldLabel>
                    <Input
                      value={rule.target}
                      onChange={(e) => updateRule(index, 'target', e.target.value)}
                      placeholder="ACCEPT"
                    />
                  </Field>
                </div>
              </div>
            ))}
            {rules.length === 0 && (
              <div className="text-center text-muted-foreground py-8">
                No firewall rules configured. Click "Add Rule" to create one.
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
