import { createContext, useContext, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { User } from "@/types/auth";
import { authQueryKeys, useSessionQuery } from "@/api/authQueries";

type AuthContextValue = {
  user: User | null;
  isSessionLoading: boolean;
  setUser: (user: User | null) => void;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const queryClient = useQueryClient();
  const { data, isLoading } = useSessionQuery();

  const setUser = (user: User | null) => {
    queryClient.setQueryData(authQueryKeys.me, user);
  };

  return (
    <AuthContext.Provider
      value={{ user: data ?? null, isSessionLoading: isLoading, setUser }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return value;
};