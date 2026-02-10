import { isAxiosError } from "axios";
import { useQuery, type UseQueryOptions } from "@tanstack/react-query";
import type { SignupConfig, User } from "@/types/auth";
import apiClient from "./client";

export const authQueryKeys = {
  me: ["me"] as const,
  signupConfig: ["signup-config"] as const,
};

const fetchMe = async () => {
  try {
    const response = await apiClient.get<User>("/auth/me");
    return response.data;
  } catch (err) {
    if (isAxiosError(err) && err.response?.status === 401) {
      return null;
    }
    throw err;
  }
};

const fetchSignupConfig = async () => {
  const response = await apiClient.get<SignupConfig>("/auth/signup-config");
  return response.data;
};

export const useSessionQuery = (
  options?: UseQueryOptions<
    User | null,
    unknown,
    User | null,
    typeof authQueryKeys.me
  >,
) =>
  useQuery({
    queryKey: authQueryKeys.me,
    queryFn: fetchMe,
    retry: false,
    ...options,
  });

export const useSignupConfigQuery = (
  options?: UseQueryOptions<
    SignupConfig,
    unknown,
    SignupConfig,
    typeof authQueryKeys.signupConfig
  >,
) =>
  useQuery({
    queryKey: authQueryKeys.signupConfig,
    queryFn: fetchSignupConfig,
    retry: false,
    ...options,
  });
