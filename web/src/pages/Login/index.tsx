import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/context/useAuth";
import { useAuthError } from "@/hooks/useAuthError";
import { useLoginMutation } from "@/api/authMutations";
import styles from "./Login.module.css";

export default function LoginPage() {
  const [error, setError] = useState<string | null>(null);
  const [loginEmail, setLoginEmail] = useState("");
  const [loginPassword, setLoginPassword] = useState("");
  const { setUser } = useAuth();
  const { getLoginErrorMessage } = useAuthError(setError);
  const navigate = useNavigate();
  const loginMutation = useLoginMutation({
    onSuccess: (data) => {
      setUser(data);
      setLoginPassword("");
      navigate("/dashboard", { replace: true });
    },
  });
  const submitting = loginMutation.isPending;

  return (
    <section className={styles.card}>
      <h2>Login</h2>
      <div className={styles.forms}>
        <form
          className={styles.form}
          onSubmit={async (event) => {
            event.preventDefault();
            setError(null);
            try {
              await loginMutation.mutateAsync({
                email: loginEmail,
                password: loginPassword,
              });
            } catch (err) {
              setError(getLoginErrorMessage(err));
            }
          }}
        >
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
      {error ? <p className="error">{error}</p> : null}
    </section>
  );
}
