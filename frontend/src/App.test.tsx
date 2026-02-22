import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import App from './App'

const mockFetch = vi.fn()

describe('App', () => {
  let hrefSetter: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.stubGlobal('fetch', mockFetch)
    hrefSetter = vi.fn()
    Object.defineProperty(window, 'location', {
      value: {
        ...window.location,
        get href() {
          return ''
        },
        set href(v: string) {
          hrefSetter(v)
        },
      },
      writable: true,
    })
  })

  it('shows loading state initially', () => {
    mockFetch.mockImplementation(() => new Promise(() => {}))
    render(<App />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('shows app placeholder when GET /api/v1/users/me returns 200', async () => {
    mockFetch.mockResolvedValueOnce({ ok: true })
    render(<App />)
    await waitFor(() => {
      expect(screen.getByText('You are authenticated.')).toBeInTheDocument()
    })
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/v1/users/me'),
      expect.objectContaining({ credentials: 'include' })
    )
  })

  it('redirects to auth login URL when GET /api/v1/users/me returns 401', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 })
    render(<App />)
    await waitFor(() => {
      expect(hrefSetter).toHaveBeenCalledWith(expect.stringContaining('/auth/login'))
    })
  })

  it('shows error when fetch fails', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'))
    render(<App />)
    await waitFor(() => {
      expect(screen.getByText(/Unable to verify authentication/)).toBeInTheDocument()
    })
  })
})
