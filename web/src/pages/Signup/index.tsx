import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import LoadingCard from "@/components/LoadingCard";
import { useAuth } from "@/context/AuthContext";
import { useAuthError } from "@/hooks/useAuthError";
import { useSignupMutation } from "@/api/authMutations";
import { useSignupConfigQuery } from "@/api/authQueries";
import styles from "./Signup.module.css";

export default function SignupPage() {
  const [error, setError] = useState<string | null>(null);
  const [signupEmail, setSignupEmail] = useState("");
  const [signupPassword, setSignupPassword] = useState("");
  const [signupPasswordConfirm, setSignupPasswordConfirm] = useState("");
  const [inviteToken, setInviteToken] = useState("");
  const { setUser } = useAuth();
  const { getSignupErrorMessage } = useAuthError(setError);
  const navigate = useNavigate();
  const {
    data: signupConfig,
    isLoading: isConfigLoading,
    error: configError,
  } = useSignupConfigQuery();
  const signupMutation = useSignupMutation({
    onSuccess: (data) => {
      setUser(data);
      setSignupPassword("");
      setSignupPasswordConfirm("");
      setInviteToken("");
      navigate("/dashboard", { replace: true });
    },
  });
  const submitting = signupMutation.isPending;
  const requiresInvite = useMemo(
    () => signupConfig?.requiresInvite ?? false,
    [signupConfig],
  );

  if (isConfigLoading) {
    return <LoadingCard />;
  }

  return (
    <section className={styles.card}>
      <h2>Signup</h2>
      <div className={styles.forms}>
        <form
          className={styles.form}
          onSubmit={async (event) => {
            event.preventDefault();
            setError(null);
            try {
              await signupMutation.mutateAsync({
                email: signupEmail,
                password: signupPassword,
                passwordConfirm: signupPasswordConfirm,
                inviteToken: inviteToken || undefined,
              });
            } catch (err) {
              setError(getSignupErrorMessage(err));
            }
          }}
        >
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
              onChange={(event) =>
                setSignupPasswordConfirm(event.target.value)
              }
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
              First user signup is open. Subsequent signups require an invite
              token.
            </p>
          )}
          <button className="button" type="submit" disabled={submitting}>
            Create account
          </button>
          <button
            className="button outline"
            type="button"
            onClick={() => navigate("/login")}
            disabled={submitting}
          >
            Back to login
          </button>
        </form>
      </div>
      {error ? <p className="error">{error}</p> : null}
      {!error && configError instanceof Error ? (
        <p className="error">{configError.message}</p>
      ) : null}
    </section>
  );
}
