import { useEffect, useState } from 'react'
import './App.css'

const API_URL = import.meta.env.VITE_API_URL ?? 'https://localhost:8443'
const AUTH_LOGIN_URL = import.meta.env.VITE_AUTH_LOGIN_URL ?? 'https://localhost:8443/auth/login'

function App() {
  const [status, setStatus] = useState<'loading' | 'authenticated' | 'error'>('loading')

  useEffect(() => {
    const checkAuth = async () => {
      try {
        const res = await fetch(`${API_URL}/api/v1/users/me`, {
          credentials: 'include',
        })
        if (res.ok) {
          setStatus('authenticated')
        } else if (res.status === 401) {
          window.location.href = AUTH_LOGIN_URL
        } else {
          setStatus('error')
        }
      } catch {
        setStatus('error')
      }
    }
    checkAuth()
  }, [])

  if (status === 'loading') {
    return (
      <div className="loading">
        <p>Loading...</p>
      </div>
    )
  }

  if (status === 'error') {
    return (
      <div className="error">
        <p>Unable to verify authentication. Please try again.</p>
      </div>
    )
  }

  return (
    <div className="app">
      <h1>LLM Control Plane</h1>
      <p>You are authenticated.</p>
    </div>
  )
}

export default App
