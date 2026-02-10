import { useState } from "react";
import { useAuth } from "@/context/AuthContext";
import { useAuthError } from "@/hooks/useAuthError";
import { useLogoutMutation } from "@/api/authMutations";
import styles from "./Dashboard.module.css";

export default function DashboardPage() {
  const [error, setError] = useState<string | null>(null);
  const { user, setUser } = useAuth();
  const { getLogoutErrorMessage } = useAuthError(setError);
  const logoutMutation = useLogoutMutation({
    onSuccess: () => {
      setUser(null);
    },
  });
  const submitting = logoutMutation.isPending;

  if (!user) return null;

  return (
    <section className={styles.card}>
      <h2>Dashboard</h2>
      <p className="muted">You are signed in and ready to go.</p>
      <div className={styles.details} style={{ marginTop: "1rem" }}>
        <div>
          <strong>{user.display_name || user.email}</strong>
          <div className="muted">{user.email}</div>
        </div>
        <span className={styles.pill}>{user.role}</span>
      </div>
      <div className={styles.actions} style={{ marginTop: "1.5rem" }}>
        <button
          className="button outline"
          type="button"
          onClick={async () => {
            setError(null);
            try {
              await logoutMutation.mutateAsync();
            } catch (err) {
              setError(getLogoutErrorMessage(err));
            }
          }}
          disabled={submitting}
        >
          Log out
        </button>
      </div>
      {error ? <p className="error">{error}</p> : null}
    </section>
  );
}
