import { useState } from "react";
import type { FormEvent } from "react";
import { Navigate, Route, Routes, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import DashboardPage from "./pages/Dashboard";
import LoginPage from "./pages/Login";
import SignupPage from "./pages/Signup";
import { useAuthError } from "./hooks/useAuthError";
import {
  authQueryKeys,
  useSessionQuery,
  useSignupConfigQuery,
} from "./queries/authQueries";
import {
  useLoginMutation,
  useLogoutMutation,
  useSignupMutation,
} from "./queries/authMutations";

export default function App() {
  const [error, setError] = useState<string | null>(null);
  const [loginEmail, setLoginEmail] = useState("");
  const [loginPassword, setLoginPassword] = useState("");
  const [signupEmail, setSignupEmail] = useState("");
  const [signupPassword, setSignupPassword] = useState("");
  const [signupPasswordConfirm, setSignupPasswordConfirm] = useState("");
  const [inviteToken, setInviteToken] = useState("");

  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { getLoginErrorMessage, getSignupErrorMessage, getLogoutErrorMessage } =
    useAuthError(setError);

  const { data: user, isLoading: isUserLoading } = useSessionQuery();
  const {
    data: signupConfig,
    isLoading: isConfigLoading,
    error: configError,
  } = useSignupConfigQuery();

  const loginMutation = useLoginMutation({
    onSuccess: (data) => {
      queryClient.setQueryData(authQueryKeys.me, data);
      setLoginPassword("");
      navigate("/dashboard", { replace: true });
    },
  });

  const signupMutation = useSignupMutation({
    onSuccess: (data) => {
      queryClient.setQueryData(authQueryKeys.me, data);
      setSignupPassword("");
      setSignupPasswordConfirm("");
      setInviteToken("");
      navigate("/dashboard", { replace: true });
    },
  });

  const logoutMutation = useLogoutMutation({
    onSuccess: () => {
      queryClient.setQueryData(authQueryKeys.me, null);
      navigate("/login", { replace: true });
    },
  });

  const handleLogin = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);

    try {
      await loginMutation.mutateAsync({ email: loginEmail, password: loginPassword });
    } catch (err) {
      setError(getLoginErrorMessage(err));
    }
  };

  const handleSignup = async (event: FormEvent<HTMLFormElement>) => {
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
  };

  const handleLogout = async () => {
    setError(null);

    try {
      await logoutMutation.mutateAsync();
    } catch (err) {
      setError(getLogoutErrorMessage(err));
    }
  };

  const loading = isUserLoading || isConfigLoading;
  const submitting =
    loginMutation.isPending || signupMutation.isPending || logoutMutation.isPending;
  const displayError =
    error ?? (configError instanceof Error ? configError.message : null);

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

      <Routes>
        <Route
          path="/"
          element={<Navigate to={user ? "/dashboard" : "/login"} replace />}
        />
        <Route
          path="/dashboard"
          element={
            <DashboardPage
              user={user}
              loading={loading}
              submitting={submitting}
              error={displayError}
              onLogout={handleLogout}
            />
          }
        />
        <Route
          path="/login"
          element={
            <LoginPage
              user={user}
              loading={loading}
              error={displayError}
              submitting={submitting}
              loginEmail={loginEmail}
              loginPassword={loginPassword}
              onLogin={handleLogin}
              setLoginEmail={setLoginEmail}
              setLoginPassword={setLoginPassword}
            />
          }
        />
        <Route
          path="/signup"
          element={
            <SignupPage
              user={user}
              loading={loading}
              signupConfig={signupConfig}
              error={displayError}
              submitting={submitting}
              signupEmail={signupEmail}
              signupPassword={signupPassword}
              signupPasswordConfirm={signupPasswordConfirm}
              inviteToken={inviteToken}
              onSignup={handleSignup}
              setSignupEmail={setSignupEmail}
              setSignupPassword={setSignupPassword}
              setSignupPasswordConfirm={setSignupPasswordConfirm}
              setInviteToken={setInviteToken}
            />
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </div>
  );
}
