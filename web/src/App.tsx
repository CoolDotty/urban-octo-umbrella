import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";

type User = {
  id: string;
  email: string;
  role: string;
  display_name: string;
};

type SignupConfig = {
  requiresInvite: boolean;
  userCount: number;
};

async function fetchJson<T>(input: RequestInfo, init?: RequestInit) {
  const response = await fetch(input, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
    ...init,
  });

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`;
    try {
      const data = (await response.json()) as { message?: string };
      if (data?.message) {
        message = data.message;
      }
    } catch {
      // ignore json parse errors
    }
    throw new Error(message);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export default function App() {
  const [path, setPath] = useState(() => window.location.pathname);
  const [user, setUser] = useState<User | null>(null);
  const [signupConfig, setSignupConfig] = useState<SignupConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [loginEmail, setLoginEmail] = useState("");
  const [loginPassword, setLoginPassword] = useState("");
  const [signupEmail, setSignupEmail] = useState("");
  const [signupPassword, setSignupPassword] = useState("");
  const [signupPasswordConfirm, setSignupPasswordConfirm] = useState("");
  const [inviteToken, setInviteToken] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const requiresInvite = useMemo(
    () => signupConfig?.requiresInvite ?? false,
    [signupConfig],
  );

  const loadSession = async () => {
    setLoading(true);
    setError(null);

    const [meResult, configResult] = await Promise.all([
      fetch("/auth/me", { credentials: "include" }),
      fetch("/auth/signup-config", { credentials: "include" }),
    ]);

    if (meResult.ok) {
      setUser((await meResult.json()) as User);
    } else {
      setUser(null);
    }

    if (configResult.ok) {
      setSignupConfig((await configResult.json()) as SignupConfig);
    }

    setLoading(false);
  };

  useEffect(() => {
    void loadSession();
  }, []);

  useEffect(() => {
    const handlePop = () => setPath(window.location.pathname);
    window.addEventListener("popstate", handlePop);
    return () => window.removeEventListener("popstate", handlePop);
  }, []);

  const navigate = (nextPath: string) => {
    if (nextPath === path) {
      return;
    }
    window.history.pushState({}, "", nextPath);
    setPath(nextPath);
  };

  const handleLogin = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      const data = await fetchJson<User>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ email: loginEmail, password: loginPassword }),
      });
      setUser(data);
      setLoginPassword("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setSubmitting(false);
    }
  };

  const handleSignup = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      const data = await fetchJson<User>("/auth/signup", {
        method: "POST",
        body: JSON.stringify({
          email: signupEmail,
          password: signupPassword,
          passwordConfirm: signupPasswordConfirm,
          inviteToken: inviteToken || undefined,
        }),
      });
      setUser(data);
      setSignupPassword("");
      setSignupPasswordConfirm("");
      setInviteToken("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Signup failed");
    } finally {
      setSubmitting(false);
    }
  };

  const handleLogout = async () => {
    setSubmitting(true);
    setError(null);

    try {
      await fetchJson<void>("/auth/logout", { method: "POST" });
      setUser(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Logout failed");
    } finally {
      setSubmitting(false);
    }
  };

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

      <section className="card auth-card">
        <h2>Authentication</h2>
        {loading ? (
          <p className="muted">Loading session...</p>
        ) : user ? (
          <div className="auth-state">
            <p className="muted">Signed in</p>
            <div className="auth-details">
              <div>
                <strong>{user.display_name || user.email}</strong>
                <div className="muted">{user.email}</div>
              </div>
              <span className="pill">{user.role}</span>
            </div>
            <button
              className="button outline"
              type="button"
              onClick={handleLogout}
              disabled={submitting}
            >
              Log out
            </button>
          </div>
        ) : path === "/signup" ? (
          <div className="auth-forms">
            <form className="auth-form" onSubmit={handleSignup}>
              <h3>Sign up</h3>
              <label>
                Email
                <input
                  type="email"
                  value={signupEmail}
                  onChange={(event) => setSignupEmail(event.target.value)}
                  autoComplete="email"
                  required
                />
              </label>
              <label>
                Password
                <input
                  type="password"
                  value={signupPassword}
                  onChange={(event) => setSignupPassword(event.target.value)}
                  autoComplete="new-password"
                  required
                />
              </label>
              <label>
                Confirm password
                <input
                  type="password"
                  value={signupPasswordConfirm}
                  onChange={(event) => setSignupPasswordConfirm(event.target.value)}
                  autoComplete="new-password"
                  required
                />
              </label>
              {requiresInvite ? (
                <label>
                  Invite token
                  <input
                    type="text"
                    value={inviteToken}
                    onChange={(event) => setInviteToken(event.target.value)}
                    required
                  />
                </label>
              ) : (
                <p className="muted">
                  First user signup is open. Subsequent signups require an
                  invite token.
                </p>
              )}
              <button className="button" type="submit" disabled={submitting}>
                Create account
              </button>
              <button
                className="button outline"
                type="button"
                onClick={() => navigate("/")}
                disabled={submitting}
              >
                Back to login
              </button>
            </form>
          </div>
        ) : (
          <div className="auth-forms">
            <form className="auth-form" onSubmit={handleLogin}>
              <h3>Log in</h3>
              <label>
                Email
                <input
                  type="email"
                  value={loginEmail}
                  onChange={(event) => setLoginEmail(event.target.value)}
                  autoComplete="email"
                  required
                />
              </label>
              <label>
                Password
                <input
                  type="password"
                  value={loginPassword}
                  onChange={(event) => setLoginPassword(event.target.value)}
                  autoComplete="current-password"
                  required
                />
              </label>
              <button className="button" type="submit" disabled={submitting}>
                Log in
              </button>
              <button
                className="button outline"
                type="button"
                onClick={() => navigate("/signup")}
                disabled={submitting}
              >
                Create account
              </button>
            </form>
          </div>
        )}

        {error ? <p className="error">{error}</p> : null}
      </section>
    </div>
  );
}
