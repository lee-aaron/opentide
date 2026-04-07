import { useConfig, useProviders } from '@/api/hooks'
import { Card, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Settings, Cpu } from 'lucide-react'

export function SettingsPage() {
  const { data: config, isLoading: configLoading } = useConfig()
  const { data: providers, isLoading: providersLoading } = useProviders()

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Settings className="h-5 w-5 text-sky-400" />
        <h1 className="text-lg font-bold text-slate-100">Settings</h1>
      </div>

      {/* Provider Status */}
      <div>
        <div className="mb-3 flex items-center gap-2">
          <Cpu className="h-4 w-4 text-sky-400" />
          <h2 className="text-sm font-medium text-slate-200">LLM Providers</h2>
        </div>
        {providersLoading ? (
          <p className="text-sm text-slate-500">Loading...</p>
        ) : !providers?.length ? (
          <Card>
            <p className="text-sm text-slate-500">No providers configured.</p>
          </Card>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            {providers.map((p) => (
              <Card key={p.name} className={p.is_default ? 'border-sky-500/30' : ''}>
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-sm font-medium capitalize text-slate-200">{p.name}</span>
                  <div className="flex gap-1">
                    {p.is_default && <Badge variant="info">DEFAULT</Badge>}
                    <Badge variant={p.configured ? 'success' : 'danger'}>
                      {p.configured ? 'CONFIGURED' : 'MISSING'}
                    </Badge>
                  </div>
                </div>
                <div className="text-xs text-slate-400">
                  Model: <code className="text-sky-400">{p.model || 'default'}</code>
                </div>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* Gateway Config */}
      {!configLoading && config && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <Card>
            <CardTitle>Gateway</CardTitle>
            <div className="mt-3 space-y-2 text-sm">
              <ConfigRow label="Host" value={config.gateway.host} />
              <ConfigRow label="Port" value={String(config.gateway.port)} />
              <ConfigRow label="Log Level" value={config.gateway.log_level} />
              <ConfigRow label="Demo Mode" value={config.gateway.demo_mode ? 'Yes' : 'No'} highlight={config.gateway.demo_mode} />
              <ConfigRow label="Dev Mode" value={config.gateway.dev_mode ? 'Yes' : 'No'} highlight={config.gateway.dev_mode} />
            </div>
          </Card>

          <Card>
            <CardTitle>Security</CardTitle>
            <div className="mt-3 space-y-2 text-sm">
              <ConfigRow label="Max Message Size" value={`${config.security.max_message_size} bytes`} />
              <ConfigRow label="Approval TTL" value={`${config.security.approval_ttl}s`} />
              <ConfigRow label="Admin Port" value={String(config.security.admin_port)} />
              <ConfigRow label="Admin Secret" value={config.security.admin_secret} />
            </div>
          </Card>

          <Card>
            <CardTitle>State</CardTitle>
            <div className="mt-3 space-y-2 text-sm">
              <ConfigRow label="Driver" value={config.state.driver} />
            </div>
          </Card>
        </div>
      )}
    </div>
  )
}

function ConfigRow({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-slate-400">{label}</span>
      <span className={highlight ? 'text-amber-400' : 'font-mono text-slate-200'}>{value}</span>
    </div>
  )
}
