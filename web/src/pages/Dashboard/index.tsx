import { useState } from "react";
import { useAuth } from "@/context/useAuth";
import { useAuthError } from "@/hooks/useAuthError";
import { useLogoutMutation } from "@/api/authMutations";
import {
  getCreateWorkspaceErrorMessage,
  useCreateWorkspaceMutation,
} from "@/api/podmanMutations";
import {
  getPodmanContainersErrorMessage,
  usePodmanContainersQuery,
} from "@/api/podmanQueries";
import { usePodmanContainersStream } from "@/api/podmanStream";
import styles from "./Dashboard.module.css";

export default function DashboardPage() {
  const [error, setError] = useState<string | null>(null);
  const [workspaceError, setWorkspaceError] = useState<string | null>(null);
  const [workspaceSuccess, setWorkspaceSuccess] = useState<string | null>(null);
  const [repoUrl, setRepoUrl] = useState("");
  const [workspaceName, setWorkspaceName] = useState("");
  const [autoStart, setAutoStart] = useState(true);
  const { user, setUser } = useAuth();
  const { getLogoutErrorMessage } = useAuthError(setError);
  const { streamError } = usePodmanContainersStream(Boolean(user));
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
  const createWorkspaceMutation = useCreateWorkspaceMutation({
    onSuccess: (data) => {
      setWorkspaceError(null);
      setWorkspaceSuccess(
        `Workspace ${data.name} created with status ${data.status}.`,
      );
      void refetchContainers();
    },
  });
  const submitting = logoutMutation.isPending;
  const creatingWorkspace = createWorkspaceMutation.isPending;
  const containerErrorMessage = containersError
    ? getPodmanContainersErrorMessage(containersError)
    : null;

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
          <h3>Create Workspace</h3>
        </div>
        <form
          className={styles.workspaceForm}
          onSubmit={async (event) => {
            event.preventDefault();
            setWorkspaceError(null);
            setWorkspaceSuccess(null);

            const payload: {
              repoUrl: string;
              name?: string;
              ref?: string;
              autoStart?: boolean;
            } = {
              repoUrl: repoUrl.trim(),
            };

            const trimmedName = workspaceName.trim();
            if (trimmedName) {
              payload.name = trimmedName;
            }

            if (!autoStart) {
              payload.autoStart = false;
            }

            try {
              await createWorkspaceMutation.mutateAsync(payload);
            } catch (err) {
              setWorkspaceError(getCreateWorkspaceErrorMessage(err));
            }
          }}
        >
          <label>
            Repo URL
            <input
              type="url"
              value={repoUrl}
              onChange={(event) => setRepoUrl(event.target.value)}
              placeholder="https://github.com/org/repo.git"
              required
            />
          </label>
          <label>
            Name (optional)
            <input
              type="text"
              value={workspaceName}
              onChange={(event) => setWorkspaceName(event.target.value)}
              placeholder="my-workspace"
            />
          </label>
          <label className={styles.checkbox}>
            <input
              type="checkbox"
              checked={autoStart}
              onChange={(event) => setAutoStart(event.target.checked)}
            />
            Auto start container
          </label>
          <div className={styles.workspaceActions}>
            <button className="button" type="submit" disabled={creatingWorkspace}>
              {creatingWorkspace ? "Creating..." : "Create workspace"}
            </button>
          </div>
        </form>
        {workspaceError ? <p className="error">{workspaceError}</p> : null}
        {workspaceSuccess ? (
          <p className={styles.success}>{workspaceSuccess}</p>
        ) : null}
      </section>
      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <h3>Containers</h3>
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
        {streamError ? <p className="error">{streamError}</p> : null}
        {!containersLoading &&
        !containerErrorMessage &&
        containers.length === 0 ? (
          <p className="muted">No containers found.</p>
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
                      {container.status || "Unknown"}
                    </span>
                  </div>
                  <div className={styles.containerMeta}>
                    {container.createdAt ? (
                      <span>Created: {container.createdAt}</span>
                    ) : null}
                    {container.ports ? (
                      <span>Ports: {container.ports}</span>
                    ) : null}
                    {container.storageSize ? (
                      <span>Storage: {container.storageSize}</span>
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
