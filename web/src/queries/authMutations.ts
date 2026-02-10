import axios from "axios";
import { useMutation, type UseMutationOptions } from "@tanstack/react-query";
import type { User } from "../types/auth";

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
  const response = await axios.post<User>("/auth/login", payload, {
    withCredentials: true,
  });
  return response.data;
};

const signup = async (payload: SignupPayload) => {
  const response = await axios.post<User>("/auth/signup", payload, {
    withCredentials: true,
  });
  return response.data;
};

const logout = async () => {
  await axios.post("/auth/logout", null, { withCredentials: true });
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