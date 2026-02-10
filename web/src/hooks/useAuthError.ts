import { useEffect } from "react";
import { useLocation } from "react-router-dom";
import axios from "axios";

export const useAuthError = (
  setError: (value: string | null) => void,
  fallbackMessages: {
    login: string;
    signup: string;
    logout: string;
  } = {
    login: "Login failed",
    signup: "Signup failed",
    logout: "Logout failed",
  },
) => {
  const location = useLocation();

  useEffect(() => {
    setError(null);
  }, [location.pathname, setError]);

  const getErrorMessage = (err: unknown, fallback: string) => {
    if (axios.isAxiosError(err)) {
      const data = err.response?.data as { message?: string } | undefined;
      if (data?.message) {
        return data.message;
      }
    }
    return err instanceof Error ? err.message : fallback;
  };

  return {
    getLoginErrorMessage: (err: unknown) => getErrorMessage(err, fallbackMessages.login),
    getSignupErrorMessage: (err: unknown) => getErrorMessage(err, fallbackMessages.signup),
    getLogoutErrorMessage: (err: unknown) => getErrorMessage(err, fallbackMessages.logout),
  };
};