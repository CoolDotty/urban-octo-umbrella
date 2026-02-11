import { useState } from "react";
import { useAuth } from "@/context/useAuth";
import { useAuthError } from "@/hooks/useAuthError";
import { useLogoutMutation } from "@/api/authMutations";
import {
  getPodmanContainersErrorMessage,
  usePodmanContainersQuery,
} from "@/api/podmanQueries";
import styles from "./Dashboard.module.css";

export default function DashboardPage() {
  const [error, setError] = useState<string | null>(null);
  const { user, setUser } = useAuth();
  const { getLogoutErrorMessage } = useAuthError(setError);
  const {
    data: containers = [],
    isLoading: containersLoading,
    isFetching: containersFetching,
    error: containersError,
    refetch: refetchContainers,
  } = usePodmanContainersQuery({
    enabled: Boolean(user),
  });
  const logoutMutation = useLogoutMutation({
    onSuccess: () => {
      setUser(null);
    },
  });
  const submitting = logoutMutation.isPending;
  const containerErrorMessage = containersError
    ? getPodmanContainersErrorMessage(containersError)
    : null;

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
      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <h3>Running containers</h3>
          <button
            className="button outline"
            type="button"
            onClick={() => refetchContainers()}
            disabled={containersFetching}
          >
            {containersFetching ? "Refreshing..." : "Refresh"}
          </button>
        </div>
        {containersLoading ? (
          <p className="muted">Loading containers...</p>
        ) : null}
        {containerErrorMessage ? (
          <p className="error">{containerErrorMessage}</p>
        ) : null}
        {!containersLoading &&
        !containerErrorMessage &&
        containers.length === 0 ? (
          <p className="muted">No running containers found.</p>
        ) : null}
        {!containersLoading &&
        !containerErrorMessage &&
        containers.length > 0 ? (
          <div className={styles.containerList}>
            {containers.map((container, index) => {
              const key = container.id || container.name || `container-${index}`;
              const displayName =
                container.name ||
                (container.id ? container.id.slice(0, 12) : "Unnamed");

              return (
                <div className={styles.containerCard} key={key}>
                  <div className={styles.containerHeader}>
                    <div>
                      <strong>{displayName}</strong>
                      <div className="muted">{container.image}</div>
                    </div>
                    <span className={styles.statusPill}>
                      {container.status || "Running"}
                    </span>
                  </div>
                  <div className={styles.containerMeta}>
                    {container.createdAt ? (
                      <span>Created: {container.createdAt}</span>
                    ) : null}
                    {container.ports ? (
                      <span>Ports: {container.ports}</span>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        ) : null}
      </section>
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
