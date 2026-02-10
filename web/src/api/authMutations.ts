import { useMutation, type UseMutationOptions } from "@tanstack/react-query";
import type { User } from "@/types/auth";
import apiClient from "./client";

export type LoginPayload = {
  email: string;
  password: string;
};

export type SignupPayload = {
  email: string;
  password: string;
  passwordConfirm: string;
  inviteToken?: string;
};

const login = async (payload: LoginPayload) => {
  const response = await apiClient.post<User>("/auth/login", payload);
  return response.data;
};

const signup = async (payload: SignupPayload) => {
  const response = await apiClient.post<User>("/auth/signup", payload);
  return response.data;
};

const logout = async () => {
  await apiClient.post("/auth/logout");
};

export const useLoginMutation = (
  options?: UseMutationOptions<User, unknown, LoginPayload>,
) => useMutation({ mutationFn: login, ...options });

export const useSignupMutation = (
  options?: UseMutationOptions<User, unknown, SignupPayload>,
) => useMutation({ mutationFn: signup, ...options });

export const useLogoutMutation = (
  options?: UseMutationOptions<void, unknown, void>,
) => useMutation({ mutationFn: logout, ...options });
