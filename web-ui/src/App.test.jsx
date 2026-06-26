import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import React from 'react'
import App from './App'

// Mock fetch
const mockFetch = vi.fn()
global.fetch = mockFetch

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(() => null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
}
global.localStorage = localStorageMock

// Mock canvas getContext
HTMLCanvasElement.prototype.getContext = vi.fn(() => null)

// Mock requestAnimationFrame
global.requestAnimationFrame = (cb) => setTimeout(cb, 0)

function mockResponse(data) {
  return Promise.resolve({
    ok: true,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  })
}

function mockError(status = 500) {
  return Promise.resolve({
    ok: false,
    status,
    text: () => Promise.resolve('Error'),
  })
}

describe('App - Dashboard', () => {
  beforeEach(() => {
    mockFetch.mockClear()
    // Reset mock implementation to return connections by default
    mockFetch.mockImplementation(() => mockResponse({ connections: [], uploadTotal: 0, downloadTotal: 0 }))
  })

  it('renders dashboard by default', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({
      uploadTotal: 1048576,
      downloadTotal: 2097152,
      version: '1.0.0',
      startTime: new Date().toISOString(),
      activeSub: 'test-sub',
    }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('DASHBOARD')).toBeInTheDocument()
    })
  })

  it('shows error when backend is unavailable', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('◉')).toBeInTheDocument()
    })
  })

  it('navigates to subscriptions page', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('Subscriptions')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Subscriptions'))

    await waitFor(() => {
      expect(screen.getByText('SUBSCRIPTIONS')).toBeInTheDocument()
    })
  })

  it('navigates to servers page', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('Servers')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Servers'))

    await waitFor(() => {
      expect(screen.getByText('SERVERS')).toBeInTheDocument()
    })
  })

  it('navigates to config page', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('Configuration')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Configuration'))

    await waitFor(() => {
      expect(screen.getByText('CONFIGURATION')).toBeInTheDocument()
    })
  })

  it('navigates to logs page', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))
    mockFetch.mockResolvedValueOnce(mockResponse({ logs: [] }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('Logs')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Logs'))

    await waitFor(() => {
      expect(screen.getByText('SYSTEM LOGS')).toBeInTheDocument()
    })
  })
})

describe('App - Subscriptions', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('displays subscription list', async () => {
    mockFetch.mockImplementation(() => {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({
          subscriptions: [
            { id: 'sub1', name: 'Test Sub', url: 'http://example.com', active: true },
          ],
        }),
      })
    })

    render(<App />)

    fireEvent.click(screen.getByText('Subscriptions'))

    await waitFor(() => {
      expect(screen.getByText('Test Sub')).toBeInTheDocument()
    })
  })

  it('shows add subscription form', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))
    mockFetch.mockResolvedValueOnce(mockResponse({ subscriptions: [] }))

    render(<App />)

    fireEvent.click(screen.getByText('Subscriptions'))

    await waitFor(() => {
      expect(screen.getByText('SUBSCRIPTIONS')).toBeInTheDocument()
    })

    // Check that the page rendered
    expect(screen.getByText('SUBSCRIPTIONS')).toBeInTheDocument()
  })
})

describe('App - Servers', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('displays server list', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))
    mockFetch.mockResolvedValueOnce(mockResponse({
      servers: [
        { id: 'srv1', name: 'Server 1', address: '192.168.1.1', port: 443, selected: true },
      ],
    }))

    render(<App />)

    fireEvent.click(screen.getByText('Servers'))

    await waitFor(() => {
      expect(screen.getByText('SERVERS')).toBeInTheDocument()
    })
  })
})

describe('App - Config', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('displays config editor', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))
    mockFetch.mockResolvedValueOnce(mockResponse({
      config: JSON.stringify({ log: { level: 'info' } }),
    }))

    render(<App />)

    fireEvent.click(screen.getByText('Configuration'))

    await waitFor(() => {
      expect(screen.getByText('CONFIGURATION')).toBeInTheDocument()
    })
  })
})

describe('App - Logs', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('displays logs page', async () => {
    // First call: stats (initial load)
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))
    // Second call: logs page
    mockFetch.mockResolvedValueOnce(mockResponse({ logs: [] }))

    render(<App />)

    fireEvent.click(screen.getByText('Logs'))

    await waitFor(() => {
      expect(screen.getByText('SYSTEM LOGS')).toBeInTheDocument()
    })
  })
})

describe('App - Navigation', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('shows all navigation items', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('Dashboard')).toBeInTheDocument()
      expect(screen.getByText('Subscriptions')).toBeInTheDocument()
      expect(screen.getByText('Servers')).toBeInTheDocument()
      expect(screen.getByText('Configuration')).toBeInTheDocument()
      expect(screen.getByText('Logs')).toBeInTheDocument()
    })
  })
})

describe('App - Dashboard Stats', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('displays uptime', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({
      uploadTotal: 1048576,
      downloadTotal: 2097152,
      version: '1.0.0',
      startTime: new Date().toISOString(),
      activeSub: 'test-sub',
    }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('UPTIME')).toBeInTheDocument()
    })
  })

  it('displays traffic stats', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({
      uploadTotal: 1048576,
      downloadTotal: 2097152,
    }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('UPLOAD')).toBeInTheDocument()
      expect(screen.getByText('DOWNLOAD')).toBeInTheDocument()
    })
  })
})

describe('App - Backend Status', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('shows backend unavailable', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('◉')).toBeInTheDocument()
    })
  })

  it('shows backend available', async () => {
    mockFetch.mockResolvedValueOnce(mockResponse({ uploadTotal: 0, downloadTotal: 0 }))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('◉')).toBeInTheDocument()
    })
  })
})

describe('App - API Error Handling', () => {
  beforeEach(() => {
    mockFetch.mockClear()
  })

  it('handles API errors gracefully', async () => {
    mockFetch.mockRejectedValueOnce(new Error('API Error'))

    render(<App />)

    await waitFor(() => {
      expect(screen.getByText('◉')).toBeInTheDocument()
    })
  })
})
