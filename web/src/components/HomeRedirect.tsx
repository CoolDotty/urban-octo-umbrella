import { Navigate } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { useSignupConfigQuery } from "@/api/authQueries";
import LoadingCard from "./LoadingCard";

export default function HomeRedirect() {
  const { user, isSessionLoading } = useAuth();
  const {
    data: signupConfig,
    isLoading: isConfigLoading,
    error: configError,
  } = useSignupConfigQuery({
    enabled: !user && !isSessionLoading,
  });

  if (isSessionLoading) {
    return <LoadingCard />;
  }

  if (user) {
    return <Navigate to="/dashboard" replace />;
  }

  if (isConfigLoading) {
    return <LoadingCard />;
  }

  if (configError) {
    return <Navigate to="/login" replace />;
  }

  const hasUsers = (signupConfig?.userCount ?? 0) > 0;

  return hasUsers ? (
    <Navigate to="/login" replace />
  ) : (
    <Navigate to="/signup" replace />
  );
}
