import { isAxiosError } from "axios";
import { useMutation, type UseMutationOptions } from "@tanstack/react-query";
import apiClient from "./client";

export type CreateWorkspacePayload = {
  repoUrl: string;
  name?: string;
  ref?: string;
  autoStart?: boolean;
};

export type CreateWorkspaceResponse = {
  name: string;
  status: string;
  repoUrl: string;
  ref?: string;
};

const createWorkspace = async (payload: CreateWorkspacePayload) => {
  const response = await apiClient.post<CreateWorkspaceResponse>(
    "/podman/workspaces",
    payload,
  );
  return response.data;
};

export const getCreateWorkspaceErrorMessage = (err: unknown) => {
  if (isAxiosError(err)) {
    const message = err.response?.data?.message;
    if (typeof message === "string" && message.trim() !== "") {
      return message;
    }
  }
  return "Failed to create workspace.";
};

export const useCreateWorkspaceMutation = (
  options?: UseMutationOptions<
    CreateWorkspaceResponse,
    unknown,
    CreateWorkspacePayload
  >,
) => useMutation({ mutationFn: createWorkspace, ...options });
