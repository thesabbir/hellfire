import { createFileRoute } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { getConfigByName } from '@/lib/api'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AlertCircle, Settings } from 'lucide-react'

export const Route = createFileRoute('/system')({
  component: System,
})

function System() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['config', 'system'],
    queryFn: async () => {
      const response = await getConfigByName({
        path: { name: 'system' },
      })
      return response.data
    },
  })

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-3xl font-bold flex items-center gap-2">
          <Settings className="w-8 h-8" />
          System Configuration
        </h2>
        <p className="text-muted-foreground">
          Manage system settings, hostname, and timezone
        </p>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            Failed to load system configuration: {error.message}
          </AlertDescription>
        </Alert>
      )}

      {isLoading ? (
        <Card>
          <CardHeader>
            <Skeleton className="h-8 w-[200px]" />
            <Skeleton className="h-4 w-[300px]" />
          </CardHeader>
          <CardContent>
            <Skeleton className="h-[200px] w-full" />
          </CardContent>
        </Card>
      ) : data ? (
        <div className="space-y-4">
          {Object.entries(data).map(([key, value]) => (
            <Card key={key}>
              <CardHeader>
                <CardTitle className="text-xl capitalize">{key}</CardTitle>
                <CardDescription>
                  {typeof value === 'object' && value !== null && 'type' in value
                    ? `Type: ${value.type}`
                    : ''}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  {typeof value === 'object' && value !== null ? (
                    Object.entries(value).map(([optionKey, optionValue]) => (
                      <div
                        key={optionKey}
                        className="flex items-start justify-between border-b pb-2 last:border-0"
                      >
                        <span className="font-medium text-sm text-muted-foreground">
                          {optionKey}
                        </span>
                        <span className="text-sm font-mono">
                          {Array.isArray(optionValue)
                            ? optionValue.join(', ')
                            : String(optionValue)}
                        </span>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm">{String(value)}</p>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : (
        <Card>
          <CardContent className="pt-6">
            <p className="text-muted-foreground text-center">
              No system configuration found
            </p>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
