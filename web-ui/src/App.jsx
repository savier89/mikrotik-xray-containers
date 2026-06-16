import { useState, useEffect } from 'react'
import './App.css'

// API Client
const api = {
  baseUrl: '',
  token: '',
  
  async request(endpoint, options = {}) {
    const url = `${this.baseUrl}${endpoint}`
    const headers = {
      'Content-Type': 'application/json',
      ...options.headers,
    }
    
    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }
    
    try {
      const response = await fetch(url, { ...options, headers })
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      return await response.json()
    } catch (error) {
      throw error
    }
  },
  
  async status() {
    return this.request('/api/status')
  },
  
  async getSubscriptions() {
    return this.request('/api/subscriptions')
  },
  
  async getServers() {
    return this.request('/api/servers')
  },
  
  async getConfig() {
    return this.request('/api/config')
  },
}

// Load settings from localStorage
const loadSettings = () => {
  const settings = localStorage.getItem('singbox_settings')
  if (settings) {
    const { apiUrl, apiToken, refreshRate } = JSON.parse(settings)
    api.baseUrl = apiUrl
    api.token = apiToken
    return { apiUrl, apiToken, refreshRate: refreshRate || 3000 }
  }
  return { apiUrl: '', apiToken: '', refreshRate: 3000 }
}

// Save settings to localStorage
const saveSettings = (settings) => {
  api.baseUrl = settings.apiUrl
  api.token = settings.apiToken
  localStorage.setItem('singbox_settings', JSON.stringify(settings))
}

// Navigation component
const Navigation = ({ activePage, setActivePage }) => {
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
          <span className="brand-icon">⚡</span>
          <div className="brand-text">
            <span className="brand-name">SING-BOX</span>
            <span className="brand-sub">MANAGER</span>
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
          </button>
        ))}
      </div>
      
      <div className="sidebar-footer">
        <div className="connection-status">
          <span className="status-dot"></span>
          <span className="status-text">CONNECTED</span>
        </div>
      </div>
    </nav>
  )
}

// Dashboard page
const Dashboard = ({ data }) => {
  return (
    <div className="page">
      <div className="page-header">
        <h1>Dashboard</h1>
      </div>
      
      <div className="dashboard-grid">
        <div className="card status-card">
          <div className="card-header">
            <h3>System Status</h3>
            <span className="badge online">ONLINE</span>
          </div>
          <div className="card-body">
            <div className="status-row">
              <div className="status-item">
                <span className="status-label">Uptime</span>
                <span className="status-value">2h 15m</span>
              </div>
              <div className="status-item">
                <span className="status-label">PID</span>
                <span className="status-value">1234</span>
              </div>
              <div className="status-item">
                <span className="status-label">Subscriptions</span>
                <span className="status-value">{data.subscriptions?.length || 0}</span>
              </div>
              <div className="status-item">
                <span className="status-label">Servers</span>
                <span className="status-value">{data.servers?.length || 0}</span>
              </div>
            </div>
          </div>
        </div>
        
        <div className="card traffic-card">
          <div className="card-header">
            <h3>Traffic</h3>
            <span className="badge live">LIVE</span>
          </div>
          <div className="card-body">
            <div className="traffic-row">
              <div className="traffic-item upload">
                <div className="traffic-icon">↑</div>
                <div className="traffic-info">
                  <span className="traffic-label">Upload</span>
                  <span className="traffic-value">1.2 MB</span>
                </div>
              </div>
              <div className="traffic-item download">
                <div className="traffic-icon">↓</div>
                <div className="traffic-info">
                  <span className="traffic-label">Download</span>
                  <span className="traffic-value">5.8 MB</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

// Subscriptions page
const Subscriptions = ({ data }) => {
  return (
    <div className="page">
      <div className="page-header">
        <h1>Subscriptions</h1>
        <button className="btn-primary">+ Add Subscription</button>
      </div>
      
      <div className="subscriptions-list">
        {data.subscriptions?.map(sub => (
          <div key={sub.id} className="sub-item">
            <div className="sub-info">
              <span className="sub-name">{sub.name || 'Subscription'}</span>
              <span className="sub-meta">{sub.server_count || 0} servers • {sub.id}</span>
            </div>
            <div className="sub-actions">
              <button className="btn-small active">ACTIVE</button>
              <button className="btn-small delete">DEL</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// Servers page
const Servers = ({ data }) => {
  return (
    <div className="page">
      <div className="page-header">
        <h1>Servers</h1>
        <button className="btn-secondary">Test All</button>
      </div>
      
      <div className="servers-list">
        {data.servers?.map((server, idx) => (
          <div key={idx} className="server-item">
            <div className="server-info">
              <span className="server-name">Server {idx}</span>
              <span className="server-meta">vless • xhttp</span>
              <span className="server-latency">45ms</span>
            </div>
            <div className="server-actions">
              <button className="btn-small active">SELECTED</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// Config page
const Config = ({ data }) => {
  return (
    <div className="page">
      <div className="page-header">
        <h1>Configuration</h1>
        <div className="page-actions">
          <button className="btn-secondary">Raw JSON</button>
          <button className="btn-primary">Save Changes</button>
        </div>
      </div>
      
      <div className="config-container">
        <div className="config-tabs">
          <button className="tab active">Preview</button>
          <button className="tab">Raw JSON</button>
          <button className="tab">Subscription</button>
        </div>
        
        <div className="tab-content active">
          <pre className="code-block">
            {JSON.stringify(data.config || {}, null, 2)}
          </pre>
        </div>
      </div>
    </div>
  )
}

// Logs page
const Logs = () => {
  return (
    <div className="page">
      <div className="page-header">
        <h1>Logs</h1>
        <div className="page-actions">
          <select className="select">
            <option value="all">All</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
          </select>
          <button className="btn-secondary">Clear</button>
        </div>
      </div>
      
      <div className="logs-container">
        <div className="logs-header">
          <span>Real-time logs</span>
          <span className="log-count">0 lines</span>
        </div>
        <div className="logs-content">
          <div className="log-line">
            <span className="log-time">2026-06-16T17:00:00</span>
            System started
          </div>
        </div>
      </div>
    </div>
  )
}

// Main App
export default function App() {
  const [activePage, setActivePage] = useState('dashboard')
  const [data, setData] = useState({
    subscriptions: [],
    servers: [],
    config: null,
  })
  
  useEffect(() => {
    const settings = loadSettings()
    console.log('Settings loaded:', settings)
  }, [])
  
  const renderPage = () => {
    switch (activePage) {
      case 'dashboard':
        return <Dashboard data={data} />
      case 'subscriptions':
        return <Subscriptions data={data} />
      case 'servers':
        return <Servers data={data} />
      case 'config':
        return <Config data={data} />
      case 'logs':
        return <Logs />
      default:
        return <Dashboard data={data} />
    }
  }
  
  return (
    <div className="app">
      <Navigation activePage={activePage} setActivePage={setActivePage} />
      <main className="main">
        {renderPage()}
      </main>
    </div>
  )
}
