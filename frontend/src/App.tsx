import { useEffect, useState } from 'react'
import './App.css'

type Container = {
  id: string
  name?: string
  image?: string
  status?: string
  createdAt?: string
}

function App() {
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
        <button
          className="primary"
          onClick={handleCreate}
          disabled={busyId === 'create'}
        >
          {busyId === 'create' ? 'Starting…' : 'Run New Container'}
        </button>
      </header>

      {error ? <div className="error">{error}</div> : null}

      <section className="panel">
        <div className="panel__header">
          <h2>Containers</h2>
          <button className="ghost" onClick={loadContainers} disabled={loading}>
            {loading ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>

        {loading ? (
          <p className="muted">Loading containers…</p>
        ) : containers.length === 0 ? (
          <p className="muted">No running containers found.</p>
        ) : (
          <div className="list">
            {containers.map((container) => (
              <div key={container.id} className="card">
                <div>
                  <p className="card__title">{container.name || container.id}</p>
                  <p className="card__meta">
                    {container.image || 'Unknown image'} ·{' '}
                    {container.status || 'Unknown status'}
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
                  {busyId === container.id ? 'Deleting…' : 'Delete'}
                </button>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}

export default App
