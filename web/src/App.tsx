export default function App() {
  return (
    <div className="app">
      <header className="hero">
        <p className="eyebrow">PocketBase + Go + React</p>
        <h1>Urban Octo Umbrella</h1>
        <p className="lede">
          Single-binary deployment with PocketBase embedded in Go and a React
          frontend served from the same executable.
        </p>
      </header>
      <section className="card">
        <h2>Next Steps</h2>
        <ul>
          <li>Define your PocketBase collections.</li>
          <li>Add custom Go hooks and API routes.</li>
          <li>Build the web app and embed it in the binary.</li>
        </ul>
      </section>
    </div>
  );
}
