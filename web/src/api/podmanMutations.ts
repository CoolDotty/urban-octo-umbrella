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

export type ContainerActionPayload = {
  containerId: string;
};

export type ContainerActionResponse = {
  status: "running" | "stopped" | "deleted";
};

const createWorkspace = async (payload: CreateWorkspacePayload) => {
  const response = await apiClient.post<CreateWorkspaceResponse>(
    "/podman/workspaces",
    payload,
  );
  return response.data;
};

const stopContainer = async ({ containerId }: ContainerActionPayload) => {
  const encodedId = encodeURIComponent(containerId);
  const response = await apiClient.post<ContainerActionResponse>(
    `/podman/containers/${encodedId}/stop`,
  );
  return response.data;
};

const startContainer = async ({ containerId }: ContainerActionPayload) => {
  const encodedId = encodeURIComponent(containerId);
  const response = await apiClient.post<ContainerActionResponse>(
    `/podman/containers/${encodedId}/start`,
  );
  return response.data;
};

const deleteContainer = async ({ containerId }: ContainerActionPayload) => {
  const encodedId = encodeURIComponent(containerId);
  const response = await apiClient.delete<ContainerActionResponse>(
    `/podman/containers/${encodedId}`,
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

const getContainerActionErrorMessage = (err: unknown, fallback: string) => {
  if (isAxiosError(err)) {
    const message = err.response?.data?.message;
    if (typeof message === "string" && message.trim() !== "") {
      return message;
    }
  }
  return fallback;
};

export const getStopContainerErrorMessage = (err: unknown) =>
  getContainerActionErrorMessage(err, "Failed to stop container.");

export const getStartContainerErrorMessage = (err: unknown) =>
  getContainerActionErrorMessage(err, "Failed to run container.");

export const getDeleteContainerErrorMessage = (err: unknown) =>
  getContainerActionErrorMessage(err, "Failed to delete container.");

export const useStartContainerMutation = (
  options?: UseMutationOptions<
    ContainerActionResponse,
    unknown,
    ContainerActionPayload
  >,
) => useMutation({ mutationFn: startContainer, ...options });

export const useStopContainerMutation = (
  options?: UseMutationOptions<
    ContainerActionResponse,
    unknown,
    ContainerActionPayload
  >,
) => useMutation({ mutationFn: stopContainer, ...options });

export const useDeleteContainerMutation = (
  options?: UseMutationOptions<
    ContainerActionResponse,
    unknown,
    ContainerActionPayload
  >,
) => useMutation({ mutationFn: deleteContainer, ...options });
