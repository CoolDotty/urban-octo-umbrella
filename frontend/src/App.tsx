import { useEffect, useState } from 'react'
import { BrowserRouter, Link, Route, Routes, useNavigate } from 'react-router-dom'
import './App.css'

type Container = {
  id: string
  name?: string
  image?: string
  status?: string
  createdAt?: string
}

type User = { username?: string; displayName?: string }

function useAuth() {
  const [user, setUser] = useState<User | null>(null)
  const [authChecked, setAuthChecked] = useState(false)

  const loadUser = async () => {
    try {
      const response = await fetch('/api/me')
      const data = await response.json()
      if (!response.ok || !data.ok) {
        setUser(null)
        return
      }
      setUser(data.user || null)
    } catch {
      setUser(null)
    } finally {
      setAuthChecked(true)
    }
  }

  useEffect(() => {
    loadUser()
  }, [])

  return { user, authChecked, reload: loadUser }
}

function Dashboard({ user }: { user: User }) {
  const [containers, setContainers] = useState<Container[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)

  const loadContainers = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await fetch('/api/containers')
      const data = await response.json()
      if (!response.ok || !data.ok) {
        throw new Error(data?.error || 'Failed to fetch containers')
      }
      setContainers(data.containers || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch containers')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadContainers()
  }, [])

  const handleCreate = async () => {
    setBusyId('create')
    setError(null)
    try {
      const response = await fetch('/api/containers', { method: 'POST' })
      const data = await response.json()
      if (!response.ok || !data.ok) {
        throw new Error(data?.error || 'Failed to create container')
      }
      await loadContainers()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create container')
    } finally {
      setBusyId(null)
    }
  }

  const handleDelete = async (id: string) => {
    setBusyId(id)
    setError(null)
    try {
      const response = await fetch(`/api/containers/${id}`, { method: 'DELETE' })
      const data = await response.json()
      if (!response.ok || !data.ok) {
        throw new Error(data?.error || 'Failed to delete container')
      }
      await loadContainers()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete container')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className="app">
      <header className="app__header">
        <div>
          <p className="app__eyebrow">Podman Control</p>
          <h1>Running Containers</h1>
          <p className="app__subtitle">
            Start a fresh container or clean up the ones already running.
          </p>
        </div>
        <div className="header__actions">
          <div className="user-pill">{user.displayName || user.username || 'Signed in'}</div>
          <button className="primary" onClick={handleCreate} disabled={busyId === 'create'}>
            {busyId === 'create' ? 'Starting...' : 'Run New Container'}
          </button>
        </div>
      </header>

      {error ? <div className="error">{error}</div> : null}

      <section className="panel">
        <div className="panel__header">
          <h2>Containers</h2>
          <button className="ghost" onClick={loadContainers} disabled={loading}>
            {loading ? 'Refreshing...' : 'Refresh'}
          </button>
        </div>

        {loading ? (
          <p className="muted">Loading containers...</p>
        ) : containers.length === 0 ? (
          <p className="muted">No running containers found.</p>
        ) : (
          <div className="list">
            {containers.map((container) => (
              <div key={container.id} className="card">
                <div>
                  <p className="card__title">{container.name || container.id}</p>
                  <p className="card__meta">
                    {container.image || 'Unknown image'} · {container.status || 'Unknown status'}
                  </p>
                  {container.createdAt ? (
                    <p className="card__meta">Created: {container.createdAt}</p>
                  ) : null}
                </div>
                <button
                  className="danger"
                  onClick={() => handleDelete(container.id)}
                  disabled={busyId === container.id}
                >
                  {busyId === container.id ? 'Deleting...' : 'Delete'}
                </button>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

function HomePage({ user }: { user: User | null }) {
  return (
    <div className="app app--center">
      <section className="panel panel--login">
        <p className="app__eyebrow">Podman Control</p>
        <h1>Welcome</h1>
        <p className="app__subtitle">
          Manage your app-owned Podman containers from a single dashboard.
        </p>
        {user ? (
          <Link className="primary link-button" to="/dashboard">
            Go to Dashboard
          </Link>
        ) : (
          <Link className="primary link-button" to="/login">
            Login with GitHub
          </Link>
        )}
      </section>
    </div>
  )
}

function LoginPage({ user }: { user: User | null }) {
  const navigate = useNavigate()
  const loginUrl = import.meta.env.DEV
    ? 'http://localhost:3000/auth/github'
    : '/auth/github'

  useEffect(() => {
    if (user) {
      navigate('/dashboard', { replace: true })
    }
  }, [user, navigate])

  return (
    <div className="app app--center">
      <section className="panel panel--login">
        <p className="app__eyebrow">Access Required</p>
        <h1>Sign in with GitHub</h1>
        <p className="app__subtitle">
          You need to authenticate before managing containers.
        </p>
        <a className="primary link-button" href={loginUrl}>
          Continue with GitHub
        </a>
      </section>
    </div>
  )
}

function DashboardRoute({ user }: { user: User | null }) {
  const navigate = useNavigate()

  useEffect(() => {
    if (!user) {
      navigate('/login', { replace: true })
    }
  }, [user, navigate])

  if (!user) {
    return (
      <div className="app">
        <p className="muted">Checking session...</p>
      </div>
    )
  }

  return <Dashboard user={user} />
}

function ErrorPage() {
  return (
    <div className="app app--center">
      <section className="panel panel--login">
        <p className="app__eyebrow">Something Went Wrong</p>
        <h1>We hit an error</h1>
        <p className="app__subtitle">
          Please try again. If the issue persists, check the backend logs.
        </p>
        <Link className="primary link-button" to="/">
          Back to Home
        </Link>
      </section>
    </div>
  )
}

function NotFoundPage() {
  return (
    <div className="app app--center">
      <section className="panel panel--login">
        <p className="app__eyebrow">Not Found</p>
        <h1>That page does not exist</h1>
        <p className="app__subtitle">
          The route you requested is not available in this app.
        </p>
        <Link className="ghost link-button" to="/">
          Back to Home
        </Link>
      </section>
    </div>
  )
}

function App() {
  const { user, authChecked } = useAuth()

  if (!authChecked) {
    return (
      <div className="app">
        <p className="muted">Checking session...</p>
      </div>
    )
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<HomePage user={user} />} />
        <Route path="/login" element={<LoginPage user={user} />} />
        <Route path="/dashboard" element={<DashboardRoute user={user} />} />
        <Route path="/error" element={<ErrorPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
