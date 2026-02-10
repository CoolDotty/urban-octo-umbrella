import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import LoadingCard from "./LoadingCard";

export default function ProtectedRoute() {
  const { user, isSessionLoading } = useAuth();

  if (isSessionLoading) {
    return <LoadingCard />;
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}
