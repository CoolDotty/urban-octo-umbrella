import { type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { User } from "@/types/auth";
import { authQueryKeys, useSessionQuery } from "@/api/authQueries";
import { AuthContext } from "@/context/useAuth";

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
