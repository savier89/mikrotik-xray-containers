import React, { useState, useEffect, useRef, useCallback } from 'react'
import './App.css'

// ==================== API Client ====================
const API = {
  baseUrl: '',
  token: '',

  async request(endpoint, options = {}) {
    const url = `${this.baseUrl}${endpoint}`
    const headers = { 'Content-Type': 'application/json', ...options.headers }
    if (this.token) headers['Authorization'] = `Bearer ${this.token}`
    try {
      const response = await fetch(url, { ...options, headers })
      if (!response || !response.json) {
        throw new Error('Network response unavailable')
      }
      const data = await response.json()
      if (!response.ok) {
        throw new Error(data.error || `HTTP ${response.status}`)
      }
      return data
    } catch (e) {
      if (e.message === 'Network response unavailable') throw e
      throw new Error(e.message || 'API request failed')
    }
  },

  async health() { return this.request('/api/health') },
  async status() { return this.request('/api/status') },
  async stats() { return this.request('/api/stats') },
  async connections() { return this.request('/api/connections') },
  async getSubscriptions() { return this.request('/api/subscriptions') },
  async addSubscription(name, url) {
    return this.request('/api/subscriptions', {
      method: 'POST',
      body: JSON.stringify({ name, url }),
    })
  },
  async deleteSubscription(id) {
    return this.request(`/api/subscriptions/${id}`, { method: 'DELETE' })
  },
  async getServers() { return this.request('/api/servers') },
  async selectServer(index) {
    return this.request('/api/servers/select', {
      method: 'POST',
      body: JSON.stringify({ index }),
    })
  },
  async testServers(timeout = 5) {
    return this.request('/api/servers/test', {
      method: 'POST',
      body: JSON.stringify({ timeout }),
    })
  },
  async testServerConfig(index) {
    return this.request('/api/servers/test-config', {
      method: 'POST',
      body: JSON.stringify({ index }),
    })
  },
  async connect(action) {
    return this.request('/api/connect', {
      method: 'POST',
      body: JSON.stringify({ action }),
    })
  },
  async getConfig() { return this.request('/api/config') },
  async reloadConfig() {
    return this.request('/api/config/reload', { method: 'POST' })
  },
  async getLogs() { return this.request('/api/logs') },
}

// ==================== Settings ====================
const loadSettings = () => {
  try {
    const raw = localStorage.getItem('singbox_settings')
    if (raw) {
      const { apiUrl, apiToken, refreshRate } = JSON.parse(raw)
      API.baseUrl = apiUrl || ''
      API.token = apiToken || ''
      return { apiUrl, apiToken, refreshRate: refreshRate || 5000 }
    }
  } catch (e) { /* ignore */ }
  return { apiUrl: '', apiToken: '', refreshRate: 5000 }
}

const saveSettings = (settings) => {
  API.baseUrl = settings.apiUrl || ''
  API.token = settings.apiToken || ''
  localStorage.setItem('singbox_settings', JSON.stringify(settings))
}

// ==================== Animated Background ====================
const CyberBackground = () => {
  const canvasRef = useRef(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    let ctx
    try {
      ctx = canvas.getContext('2d')
    } catch (e) {
      return
    }
    if (!ctx) return
    let animId

    const resize = () => {
      canvas.width = window.innerWidth
      canvas.height = window.innerHeight
    }
    resize()
    window.addEventListener('resize', resize)

    const chars = '0123456789ABCDEF<>{}[]|/\\_-=+!@#$%^&*~'
    const fontSize = 14
    const columns = Math.floor(canvas.width / fontSize)
    const drops = Array(columns).fill(1)
    const speeds = Array(columns).fill(0).map(() => 0.2 + Math.random() * 0.5)

    const nodes = []
    for (let i = 0; i < 20; i++) {
      nodes.push({
        x: Math.random() * canvas.width,
        y: Math.random() * canvas.height,
        vx: (Math.random() - 0.5) * 0.2,
        vy: (Math.random() - 0.5) * 0.2,
        radius: 1 + Math.random() * 1.5,
        pulse: Math.random() * Math.PI * 2,
      })
    }

    const draw = () => {
      ctx.fillStyle = 'rgba(5, 5, 15, 0.06)'
      ctx.fillRect(0, 0, canvas.width, canvas.height)

      ctx.font = `${fontSize}px monospace`
      for (let i = 0; i < drops.length; i++) {
        const char = chars[Math.floor(Math.random() * chars.length)]
        const x = i * fontSize
        const y = drops[i] * fontSize

        ctx.fillStyle = 'rgba(0, 240, 255, 0.6)'
        ctx.fillText(char, x, y)

        ctx.fillStyle = 'rgba(0, 240, 255, 0.1)'
        ctx.fillText(chars[Math.floor(Math.random() * chars.length)], x, y - fontSize)

        drops[i] += speeds[i]
        if (drops[i] * fontSize > canvas.height && Math.random() > 0.975) {
          drops[i] = 0
          speeds[i] = 0.2 + Math.random() * 0.5
        }
      }

      ctx.strokeStyle = 'rgba(139, 92, 246, 0.04)'
      ctx.lineWidth = 0.5
      for (let i = 0; i < nodes.length; i++) {
        const a = nodes[i]
        for (let j = i + 1; j < nodes.length; j++) {
          const b = nodes[j]
          const dist = Math.hypot(a.x - b.x, a.y - b.y)
          if (dist < 180) {
            ctx.beginPath()
            ctx.moveTo(a.x, a.y)
            ctx.lineTo(b.x, b.y)
            ctx.stroke()
          }
        }
      }

      const time = Date.now() / 1000
      for (const node of nodes) {
        node.x += node.vx
        node.y += node.vy
        if (node.x < 0 || node.x > canvas.width) node.vx *= -1
        if (node.y < 0 || node.y > canvas.height) node.vy *= -1

        const pulseAlpha = 0.2 + 0.2 * Math.sin(time * 1.5 + node.pulse)
        ctx.beginPath()
        ctx.arc(node.x, node.y, node.radius, 0, Math.PI * 2)
        ctx.fillStyle = `rgba(0, 240, 255, ${pulseAlpha})`
        ctx.fill()
      }

      animId = requestAnimationFrame(draw)
    }

    draw()
    return () => {
      cancelAnimationFrame(animId)
      window.removeEventListener('resize', resize)
    }
  }, [])

  return <canvas ref={canvasRef} className="cyber-bg" />
}

// ==================== Scanline Overlay ====================
const Scanlines = () => <div className="scanlines" />

// ==================== Hex Badge ====================
const HexBadge = ({ label, value, color = 'cyan' }) => (
  <div className="hex-badge">
    <div className="hex-corner hex-tl" />
    <div className="hex-corner hex-tr" />
    <div className="hex-corner hex-bl" />
    <div className="hex-corner hex-br" />
    <div className="hex-content">
      <div className="hex-label">{label}</div>
      <div className={`hex-value ${color}`}>{value}</div>
    </div>
  </div>
)

// ==================== Glitch Text ====================
const GlitchText = ({ text, className = '' }) => (
  <span className={`glitch-text ${className}`} data-text={text}>
    {text}
  </span>
)

// ==================== Status Indicator ====================
const StatusDot = ({ online }) => (
  <span className={`status-dot ${online ? 'online' : 'offline'}`} />
)

// ==================== Navigation ====================
const Navigation = ({ activePage, setActivePage, backendOnline }) => {
  const pages = [
      { id: 'dashboard', icon: '◉', label: 'Dashboard' },
      { id: 'subscriptions', icon: '◈', label: 'Subscriptions' },
      { id: 'servers', icon: '◎', label: 'Servers' },
      { id: 'config', icon: '⚙', label: 'Configuration' },
      { id: 'logs', icon: '▤', label: 'Logs' },
  ]

  return (
    <nav className="sidebar">
      <div className="sidebar-header">
        <div className="brand">
          <div className="brand-icon-wrap">
            <span className="brand-icon">⚡</span>
          </div>
          <div className="brand-text">
            <span className="brand-name">
              <GlitchText text="SING-BOX" />
            </span>
            <span className="brand-sub">M A N A G E R</span>
          </div>
        </div>
      </div>

      <div className="sidebar-nav">
        {pages.map(page => (
          <button
            key={page.id}
            className={`nav-item ${activePage === page.id ? 'active' : ''}`}
            onClick={() => setActivePage(page.id)}
          >
            <span className="nav-icon">{page.icon}</span>
            <span className="nav-label">{page.label}</span>
            <span className="nav-glow" />
          </button>
        ))}
      </div>

      <div className="sidebar-footer">
        <div className="connection-status">
          <StatusDot online={backendOnline} />
          <span className="status-text">
            {backendOnline ? 'CONNECTED' : 'DISCONNECTED'}
          </span>
        </div>
      </div>
    </nav>
  )
}

// ==================== Dashboard ====================
const Dashboard = ({ data, loading, error }) => {
  const status = data?.status || {}
  const stats = data?.stats || {}
  const subs = data?.subscriptions || { subscriptions: [] }
  const conns = data?.connections || {}

  const uptimeSec = status?.uptime || 0
  const uptimeStr = uptimeSec > 0
    ? `${Math.floor(uptimeSec / 3600)}h ${Math.floor((uptimeSec % 3600) / 60)}m`
    : 'N/A'

  const formatBytes = (bytes) => {
    if (!bytes || bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return (bytes / Math.pow(k, i)).toFixed(1) + ' ' + sizes[i]
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1><GlitchText text="DASHBOARD" /></h1>
          <p className="page-subtitle">System overview & real-time metrics</p>
        </div>
        <div className="header-timestamp">
          {new Date().toISOString().replace('T', ' ').substring(0, 19)}
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="dashboard-grid">
        <HexBadge
          label="STATUS"
          value={status?.running ? 'ONLINE' : 'OFFLINE'}
          color={status?.running ? 'green' : 'red'}
        />
        <HexBadge label="UPTIME" value={uptimeStr} color="cyan" />
        <HexBadge label="SUBSCRIPTIONS" value={subs?.subscriptions?.length || 0} color="purple" />
        <HexBadge
          label="ACTIVE SUB"
          value={subs?.active || 'None'}
          color="cyan"
        />

        <div className="card traffic-card">
          <div className="card-header">
            <h3>TRAFFIC MONITOR</h3>
            <span className="badge live">LIVE</span>
          </div>
          <div className="card-body">
            <div className="traffic-row">
              <div className="traffic-item upload">
                <div className="traffic-icon">↑</div>
                <div className="traffic-info">
                  <span className="traffic-label">UPLOAD</span>
                  <span className="traffic-value">{formatBytes(stats?.upload)}</span>
                </div>
              </div>
              <div className="traffic-item download">
                <div className="traffic-icon">↓</div>
                <div className="traffic-info">
                  <span className="traffic-label">DOWNLOAD</span>
                  <span className="traffic-value">{formatBytes(stats?.download)}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="card connections-card">
          <div className="card-header">
            <h3>CONNECTIONS</h3>
            <span className="badge info">
              {conns?.metas?.length || 0} active
            </span>
          </div>
          <div className="card-body">
            <div className="conn-row">
              <span className="conn-label">Total Upload:</span>
              <span className="conn-value">{formatBytes(conns?.uploadTotal || 0)}</span>
            </div>
            <div className="conn-row">
              <span className="conn-label">Total Download:</span>
              <span className="conn-value">{formatBytes(conns?.downloadTotal || 0)}</span>
            </div>
          </div>
        </div>

        <div className="card security-card">
          <div className="card-header">
            <h3>SECURITY</h3>
            <span className="badge secure">PROTECTED</span>
          </div>
          <div className="card-body">
            <div className="security-row">
              <div className="security-item">
                <span className="sec-icon">🔒</span>
                <span className="sec-label">ENCRYPTION</span>
                <span className="sec-value">TLS 1.3</span>
              </div>
              <div className="security-item">
                <span className="sec-icon">🛡</span>
                <span className="sec-label">PROTOCOL</span>
                <span className="sec-value">VLESS</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

// ==================== Subscriptions ====================
const Subscriptions = ({ data, loading, error, onAdd, onDelete, onSetActive }) => {
  const [showForm, setShowForm] = useState(false)
  const [formName, setFormName] = useState('')
  const [formUrl, setFormUrl] = useState('')
  const [formError, setFormError] = useState('')
  const [formLoading, setFormLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!formUrl) {
      setFormError('URL is required')
      return
    }
    setFormError('')
    setFormLoading(true)
    try {
      await onAdd(formName || 'Subscription', formUrl)
      setFormName('')
      setFormUrl('')
      setShowForm(false)
    } catch (err) {
      setFormError(err.message || 'Failed to add subscription')
    } finally {
      setFormLoading(false)
    }
  }

  const subs = data?.subscriptions || { subscriptions: [] }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1><GlitchText text="SUBSCRIPTIONS" /></h1>
          <p className="page-subtitle">Manage proxy subscription sources</p>
        </div>
        <button
          className="btn-primary"
          onClick={() => setShowForm(!showForm)}
        >
          <span className="btn-icon">{showForm ? '✕' : '+'}</span>{' '}
          {showForm ? 'Cancel' : 'Add Subscription'}
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {showForm && (
        <div className="card form-card">
          <div className="card-header"><h3>New Subscription</h3></div>
          <div className="card-body">
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label>Name</label>
                <input
                  type="text"
                  className="input"
                  value={formName}
                  onChange={e => setFormName(e.target.value)}
                  placeholder="My Subscription"
                />
              </div>
              <div className="form-group">
                <label>URL</label>
                <input
                  type="text"
                  className="input"
                  value={formUrl}
                  onChange={e => setFormUrl(e.target.value)}
                  placeholder="https://..."
                />
              </div>
              {formError && <div className="form-error">{formError}</div>}
              <button type="submit" className="btn-primary" disabled={formLoading}>
                {formLoading ? 'Fetching...' : 'Add'}
              </button>
            </form>
          </div>
        </div>
      )}

      <div className="subscriptions-list">
        {subs?.subscriptions?.length === 0 ? (
          <div className="empty-state">
            <span className="empty-icon">◈</span>
            <span className="empty-text">No subscriptions yet</span>
          </div>
        ) : (
          subs?.subscriptions?.map(sub => (
            <div
              key={sub.id}
              className={`sub-item ${subs?.active === sub.id ? 'active' : ''}`}
            >
              <div className="sub-item-inner">
                <div className="sub-info">
                  <span className="sub-name">{sub.name || 'Subscription'}</span>
                  <span className="sub-meta">
                    {sub.server_count || 0} servers • {sub.created || ''}
                  </span>
                  <span className="sub-url">{sub.url || ''}</span>
                </div>
                <div className="sub-actions">
                  {subs?.active !== sub.id && (
                    <button
                      className="btn-small active"
                      onClick={() => onSetActive(sub.id)}
                    >
                      SET ACTIVE
                    </button>
                  )}
                  {subs?.active === sub.id && (
                    <span className="badge active">ACTIVE</span>
                  )}
                  <button
                    className="btn-small delete"
                    onClick={() => onDelete(sub.id)}
                  >
                    DEL
                  </button>
                </div>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

// ==================== Servers ====================
const Servers = ({ data, loading, error, onSelect, onTest, onConnect, onTestConfig }) => {
  const [testLoading, setTestLoading] = useState(false)
  const [testResults, setTestResults] = useState([])
  const [testConfigLoading, setTestConfigLoading] = useState(false)
  const [testConfigResult, setTestConfigResult] = useState(null)
  const [connectLoading, setConnectLoading] = useState(false)
  const [connected, setConnected] = useState(false)

  const handleTest = async () => {
    setTestLoading(true)
    setTestResults([])
    try {
      const result = await onTest(5)
      setTestResults(result.results || [])
    } catch (err) {
      // error handled by parent
    } finally {
      setTestLoading(false)
    }
  }

  const handleTestConfig = async (idx) => {
    setTestConfigLoading(true)
    setTestConfigResult(null)
    try {
      const result = await onTestConfig(idx)
      setTestConfigResult(result)
    } catch (err) {
      setTestConfigResult({ valid: false, message: err.message })
    } finally {
      setTestConfigLoading(false)
    }
  }

  const handleConnect = async () => {
    setConnectLoading(true)
    try {
      const result = await onConnect(connected ? 'disconnect' : 'connect')
      setConnected(result?.status === 'connected')
    } catch (err) {
      // error handled by parent
    } finally {
      setConnectLoading(false)
    }
  }

  const handleSelect = (idx) => {
    onSelect(idx)
  }

  const servers = data?.servers || {}
  const serverList = servers?.servers || []

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1><GlitchText text="SERVERS" /></h1>
          <p className="page-subtitle">
            {servers?.subscription ? `Servers from: ${servers.subscription}` : 'Select a subscription first'}
          </p>
        </div>
        <div className="page-actions">
          <button
            className={`btn-${connected ? 'danger' : 'primary'}`}
            onClick={handleConnect}
            disabled={connectLoading || serverList.length === 0}
          >
            {connectLoading ? '...' : connected ? 'Disconnect' : 'Connect'}
          </button>
          <button
            className="btn-secondary"
            onClick={handleTest}
            disabled={testLoading || serverList.length === 0}
          >
            {testLoading ? 'Testing...' : 'Test All'}
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {testResults.length > 0 && (
        <div className="card test-results">
          <div className="card-header">
            <h3>TEST RESULTS</h3>
            <span className="badge info">
              {testResults.filter(r => r.latency != null).length} reachable
            </span>
          </div>
          <div className="card-body">
            {testResults.map((r, i) => (
              <div
                key={i}
                className={`test-row ${r.latency != null ? 'reachable' : 'unreachable'}`}
              >
                <span className="test-index">#{r.index}</span>
                <span className="test-latency">
                  {r.latency != null ? `${r.latency}ms` : 'timeout'}
                </span>
                <span className="test-status">
                  {r.latency != null ? '✓' : '✗'}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="servers-list">
        {serverList.length === 0 ? (
          <div className="empty-state">
            <span className="empty-icon">◎</span>
            <span className="empty-text">
              {error ? 'Error loading servers' : 'No servers — set an active subscription'}
            </span>
          </div>
        ) : (
          serverList.map((server, idx) => (
            <div
              key={idx}
              className={`server-item ${servers?.selected === idx ? 'selected' : ''}`}
            >
              <div className="server-item-inner">
                <div className="server-info">
                  <span className="server-name">{server.name || `Server #${idx}`}</span>
                  <span className="server-meta">
                    {server.url ? server.url.split('://')[0] : 'proxy'}
                  </span>
                  {testResults[idx]?.latency != null && (
                    <span className="server-latency">
                      {testResults[idx].latency}ms
                    </span>
                  )}
                </div>
                <div className="server-actions">
                  {servers?.selected !== idx && (
                    <button
                      className="btn-small select"
                      onClick={() => onSelect(idx)}
                    >
                      SELECT
                    </button>
                  )}
                  {servers?.selected === idx && (
                    <span className="badge selected">SELECTED</span>
                  )}
                  <button
                    className="btn-small test-config"
                    onClick={() => handleTestConfig(idx)}
                    disabled={testConfigLoading}
                  >
                    {testConfigLoading ? '...' : 'Test Config'}
                  </button>
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {testConfigResult && (
        <div className="card test-config-result">
          <div className="card-header">
            <h3>CONFIG TEST RESULT</h3>
            <span className={`badge ${testConfigResult.valid ? 'success' : 'error'}`}>
              {testConfigResult.valid ? '✓ VALID' : '✗ INVALID'}
            </span>
          </div>
          <div className="card-body">
            <pre className="config-test-output">{testConfigResult.message}</pre>
            {testConfigResult.config && (
              <pre className="config-json">{testConfigResult.config}</pre>
            )}
          </div>
        </div>
      )}

      {data?.config && (
        <div className="card config-card">
          <div className="card-header">
            <h3>SERVER CONFIG</h3>
            <button className="btn-small" onClick={() => navigator.clipboard.writeText(JSON.stringify(data.config, null, 2))}>
              Copy JSON
            </button>
          </div>
          <div className="card-body">
            <pre className="config-json">{JSON.stringify(data.config, null, 2)}</pre>
          </div>
        </div>
      )}
    </div>
  )
}

// ==================== Config ====================
const Config = ({ data, loading, error, onReload }) => {
  const [activeTab, setActiveTab] = useState('preview')
  const [reloadLoading, setReloadLoading] = useState(false)
  const [reloadMsg, setReloadMsg] = useState('')

  const handleReload = async () => {
    setReloadLoading(true)
    setReloadMsg('')
    try {
      const result = await onReload()
      setReloadMsg(result.message || 'Reloaded')
    } catch (err) {
      setReloadMsg(err.message || 'Reload failed')
    } finally {
      setReloadLoading(false)
    }
  }

  const config = data?.config || {}
  const configJson = JSON.stringify(config, null, 2)

  // Get VPN status from connections
  const [vpnStatus, setVpnStatus] = useState({ connected: false, upload: 0, download: 0 })
  const [statusLoading, setStatusLoading] = useState(false)

  const checkVpnStatus = async () => {
    setStatusLoading(true)
    try {
      const connections = await API.connections()
      const activeConns = connections?.connections?.filter(c => c.upload > 0 || c.download > 0) || []
      setVpnStatus({
        connected: activeConns.length > 0,
        upload: connections?.uploadTotal || 0,
        download: connections?.downloadTotal || 0,
      })
    } catch (err) {
      console.error('Failed to get VPN status:', err)
    } finally {
      setStatusLoading(false)
    }
  }

  useEffect(() => {
    checkVpnStatus()
    const interval = setInterval(checkVpnStatus, 5000)
    return () => clearInterval(interval)
  }, [])

  const formatBytes = (bytes) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1><GlitchText text="CONFIGURATION" /></h1>
          <p className="page-subtitle">sing-box configuration & settings</p>
        </div>
        <div className="page-actions">
          <button
            className="btn-secondary"
            onClick={handleReload}
            disabled={reloadLoading}
          >
            {reloadLoading ? 'Reloading...' : 'Reload sing-box'}
          </button>
          <button
            className="btn-primary"
            onClick={() => navigator.clipboard?.writeText(configJson)}
          >
            Copy JSON
          </button>
        </div>
      </div>

      {/* VPN Status Indicator */}
      <div className="vpn-status" style={{
        display: 'flex',
        alignItems: 'center',
        gap: '16px',
        padding: '16px',
        background: vpnStatus.connected ? 'rgba(0, 255, 136, 0.1)' : 'rgba(255, 0, 0, 0.1)',
        border: `1px solid ${vpnStatus.connected ? 'rgba(0, 255, 136, 0.3)' : 'rgba(255, 0, 0, 0.3)'}`,
        borderRadius: '8px',
        marginBottom: '16px',
      }}>
        <div style={{
          width: '12px',
          height: '12px',
          borderRadius: '50%',
          background: vpnStatus.connected ? '#00ff88' : '#ff0044',
          boxShadow: vpnStatus.connected ? '0 0 10px #00ff88' : '0 0 10px #ff0044',
        }} />
        <span style={{ fontWeight: 'bold', color: vpnStatus.connected ? '#00ff88' : '#ff0044' }}>
          {vpnStatus.connected ? 'CONNECTED' : 'DISCONNECTED'}
        </span>
        <span style={{ color: 'var(--text-muted)' }}>
          ↑ {formatBytes(vpnStatus.upload)} | ↓ {formatBytes(vpnStatus.download)}
        </span>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {reloadMsg && <div className="alert alert-info">{reloadMsg}</div>}

      <div className="config-container">
        <div className="config-tabs">
          <button
            className={`tab ${activeTab === 'preview' ? 'active' : ''}`}
            onClick={() => setActiveTab('preview')}
          >
            Preview
          </button>
          <button
            className={`tab ${activeTab === 'raw' ? 'active' : ''}`}
            onClick={() => setActiveTab('raw')}
          >
            Raw JSON
          </button>
        </div>

        <div className="tab-content active">
          <pre className="code-block">
            {activeTab === 'preview' ? (
              <>
{JSON.stringify({
  log: config?.log || {},
  dns: config?.dns || {},
  inbounds: config?.inbounds?.map(i => ({ tag: i.tag, type: i.type })) || [],
  outbounds: config?.outbounds?.map(o => ({ tag: o.tag, type: o.type })) || [],
  route: config?.route ? {
    rules: config.route.rules?.length || 0,
    ruleSet: config.route.rule_set?.length || 0,
  } : {},
  experimental: config?.experimental || {},
}, null, 2)}
              </>
            ) : (
              configJson
            )}
          </pre>
        </div>
      </div>
    </div>
  )
}

// ==================== Logs ====================
const Logs = ({ data, loading, error }) => {
  const [filter, setFilter] = useState('all')
  const logs = data?.logs?.logs || data?.logs || ''
  const lines = typeof logs === 'string' ? logs.split('\n').filter(Boolean) : []

  const filtered = filter === 'all'
    ? lines
    : lines.filter(l => l.toLowerCase().includes(filter))

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1><GlitchText text="SYSTEM LOGS" /></h1>
          <p className="page-subtitle">Real-time event stream</p>
        </div>
        <div className="page-actions">
          <select
            className="select"
            value={filter}
            onChange={e => setFilter(e.target.value)}
          >
            <option value="all">All Levels</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
          <button
            className="btn-secondary"
            onClick={() => { window.location.reload() }}
          >
            Refresh
          </button>
        </div>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="logs-container">
        <div className="logs-header">
          <span>Log output</span>
          <span className="log-count">{filtered.length} lines</span>
        </div>
        <div className="logs-content">
          {filtered.length === 0 ? (
            <div className="empty-state">
              <span className="empty-icon">▤</span>
              <span className="empty-text">No logs available</span>
            </div>
          ) : (
            filtered.map((line, i) => {
              let level = 'info'
              if (/\[WARN\]|warn/i.test(line)) level = 'warn'
              else if (/\[ERROR\]|error/i.test(line)) level = 'error'
              return (
                <div key={i} className="log-line">
                  <span className={`log-level ${level}`}>[{level.toUpperCase()}]</span>
                  <span className="log-text">{line}</span>
                </div>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}

// ==================== Settings ====================
const Settings = ({ settings, onSave }) => {
  const [form, setForm] = useState(settings)
  const [saved, setSaved] = useState(false)

  const handleSave = () => {
    onSave(form)
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div className="page">
      <div className="page-header">
        <div>
          <h1><GlitchText text="SETTINGS" /></h1>
          <p className="page-subtitle">Connection & display settings</p>
        </div>
      </div>

      {saved && <div className="alert alert-success">Settings saved!</div>}

      <div className="card settings-card">
        <div className="card-header"><h3>Backend API</h3></div>
        <div className="card-body">
          <div className="form-group">
            <label>API Base URL</label>
            <input
              type="text"
              className="input"
              value={form.apiUrl || ''}
              onChange={e => setForm({ ...form, apiUrl: e.target.value })}
              placeholder="Leave empty for local API proxy"
            />
            <span className="form-hint">
              Leave empty to use the built-in API proxy (sing-box container on port 9090)
            </span>
          </div>
          <div className="form-group">
            <label>API Token</label>
            <input
              type="text"
              className="input"
              value={form.apiToken || ''}
              onChange={e => setForm({ ...form, apiToken: e.target.value })}
              placeholder="Bearer token (optional)"
            />
          </div>
          <div className="form-group">
            <label>Refresh Rate (ms)</label>
            <input
              type="number"
              className="input"
              value={form.refreshRate || 5000}
              onChange={e => setForm({ ...form, refreshRate: parseInt(e.target.value) || 5000 })}
            />
          </div>
          <button className="btn-primary" onClick={handleSave}>Save Settings</button>
        </div>
      </div>
    </div>
  )
}

// ==================== Main App ====================
export default function App() {
  const [activePage, setActivePage] = useState('dashboard')
  const [settings] = useState(loadSettings)
  const [backendOnline, setBackendOnline] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Data state
  const [statusData, setStatusData] = useState(null)
  const [statsData, setStatsData] = useState(null)
  const [subsData, setSubsData] = useState(null)
  const [serversData, setServersData] = useState(null)
  const [serverConfig, setServerConfig] = useState(null)
  const [configData, setConfigData] = useState(null)
  const [logsData, setLogsData] = useState(null)

  // API call wrapper
  const apiCall = useCallback(async (fn, setErrorKey = true) => {
    try {
      setLoading(true)
      setError('')
      const result = await fn()
      return result
    } catch (err) {
      if (setErrorKey) setError(err.message || 'API Error')
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  // Check backend health
  useEffect(() => {
    const checkHealth = async () => {
      try {
        await API.health()
        setBackendOnline(true)
      } catch {
        setBackendOnline(false)
      }
    }
    checkHealth()
  }, [])

  // Fetch dashboard data
  useEffect(() => {
    if (!backendOnline) return

    const fetchData = async () => {
      try {
        const [status, stats, subs, conns] = await Promise.allSettled([
          API.status(),
          API.stats(),
          API.getSubscriptions(),
          API.connections(),
        ])

        if (status.status === 'fulfilled') setStatusData(status.value)
        if (stats.status === 'fulfilled') setStatsData(stats.value)
        if (subs.status === 'fulfilled') setSubsData(subs.value)
        if (conns.status === 'fulfilled') setConnsData(conns.value)
      } catch {
        // ignore
      }
    }

    fetchData()
    const interval = setInterval(fetchData, settings.refreshRate)
    return () => clearInterval(interval)
  }, [backendOnline, settings.refreshRate])

  // Fetch config
  useEffect(() => {
    if (!backendOnline || activePage !== 'config') return
    apiCall(() => API.getConfig().then(setConfigData))
  }, [backendOnline, activePage, apiCall])

  // Fetch logs
  useEffect(() => {
    if (!backendOnline || activePage !== 'logs') return
    apiCall(() => API.getLogs().then(setLogsData))
  }, [backendOnline, activePage, apiCall])

  // Fetch servers
  useEffect(() => {
    if (!backendOnline || activePage !== 'servers') return
    apiCall(async () => {
      const data = await API.getServers()
      setServersData(data)
    })
  }, [backendOnline, activePage, apiCall])

  // Subscription actions
  const handleAddSubscription = async (name, url) => {
    const result = await apiCall(() => API.addSubscription(name, url))
    setSubsData(await API.getSubscriptions())
    return result
  }

  const handleDeleteSubscription = async (id) => {
    await apiCall(() => API.deleteSubscription(id))
    setSubsData(await API.getSubscriptions())
    setServersData(null)
    setServerConfig(null)
    setConfigData(await API.getConfig())
  }

  const handleSetActive = async (id) => {
    // Activate subscription via API endpoint
    await apiCall(async () => {
      await API.request(`/api/subscriptions/${id}/activate`, { method: 'POST' })
      setSubsData(await API.getSubscriptions())
      setServersData(await API.getServers())
    })
  }

  const handleSelectServer = async (index) => {
    const result = await apiCall(() => API.selectServer(index))
    if (result?.config) {
      setServerConfig(result.config)
    }
    setServersData(await API.getServers())
    setConfigData(await API.getConfig())
  }

  const handleTestServers = async (timeout) => {
    const result = await apiCall(() => API.testServers(timeout))
    setServersData(await API.getServers())
    return result
  }

  const handleTestServerConfig = async (index) => {
    return await apiCall(() => API.testServerConfig(index))
  }

  const handleConnect = async (action) => {
    return await apiCall(() => API.connect(action))
  }

  const handleReloadConfig = async () => {
    const result = await apiCall(() => API.reloadConfig())
    return result
  }

  const renderPage = () => {
    if (!backendOnline && activePage !== 'settings') {
      return (
        <div className="page">
          <div className="empty-state large">
            <span className="empty-icon">⚠</span>
            <span className="empty-text">Backend API unavailable</span>
            <span className="empty-hint">
              Check Settings to configure API connection
            </span>
          </div>
        </div>
      )
    }

    switch (activePage) {
      case 'dashboard':
        return (
          <Dashboard
            data={{ status: statusData, stats: statsData, subscriptions: subsData, connections: serversData }}
            loading={loading}
            error={error}
          />
        )
      case 'subscriptions':
        return (
          <Subscriptions
            data={{ subscriptions: subsData }}
            loading={loading}
            error={error}
            onAdd={handleAddSubscription}
            onDelete={handleDeleteSubscription}
            onSetActive={handleSetActive}
          />
        )
      case 'servers':
        return (
          <Servers
            data={{ servers: serversData, config: serverConfig }}
            loading={loading}
            error={error}
            onSelect={handleSelectServer}
            onTest={handleTestServers}
            onConnect={handleConnect}
            onTestConfig={handleTestServerConfig}
          />
        )
      case 'config':
        return (
          <Config
            data={{ config: configData }}
            loading={loading}
            error={error}
            onReload={handleReloadConfig}
          />
        )
      case 'logs':
        return (
          <Logs
            data={{ logs: logsData }}
            loading={loading}
            error={error}
          />
        )
      case 'settings':
        return (
          <Settings
            settings={settings}
            onSave={saveSettings}
          />
        )
      default:
        return <Dashboard data={{}} loading={loading} error={error} />
    }
  }

  return (
    <div className="app">
      <CyberBackground />
      <Scanlines />
      <Navigation
        activePage={activePage}
        setActivePage={setActivePage}
        backendOnline={backendOnline}
      />
      <main className="main">
        {renderPage()}
      </main>
    </div>
  )
}
