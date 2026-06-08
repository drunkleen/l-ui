import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback } from 'react';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import {
  NodeBootstrapJobSchema,
  PortGroupListSchema,
  ProbeResultSchema,
  PushConfigResultSchema,
  UfwStatusSchema,
  type NodeBootstrapFormValues,
  type NodeBootstrapJob,
  type PortGroup,
  type ProbeResult,
  type PushConfigResult,
  type UfwStatus,
} from '@/schemas/node';

export type { ProbeResult };

export interface NodeUpdateResult {
  id: number;
  name?: string;
  ok: boolean;
  error?: string;
}

export function useNodeMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.nodes.root() });

  const createMut = useMutation({
    mutationFn: (payload: Partial<NodeRecord>) =>
      HttpUtil.post('/panel/api/nodes/add', payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const updateMut = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: Partial<NodeRecord> }) =>
      HttpUtil.post(`/panel/api/nodes/update/${id}`, payload),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const removeMut = useMutation({
    mutationFn: ({ id, cleanupRemote }: { id: number; cleanupRemote?: boolean }) =>
      HttpUtil.post(`/panel/api/nodes/del/${id}`, { cleanupRemote: !!cleanupRemote }),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const setEnableMut = useMutation({
    mutationFn: ({ id, enable }: { id: number; enable: boolean }) =>
      HttpUtil.post(`/panel/api/nodes/setEnable/${id}`, { enable }),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const probeMut = useMutation({
    mutationFn: async (id: number): Promise<Msg<ProbeResult>> => {
      const raw = await HttpUtil.post(`/panel/api/nodes/probe/${id}`);
      return parseMsg(raw, ProbeResultSchema, 'nodes/probe');
    },
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bootstrapMut = useMutation({
    mutationFn: async (payload: NodeBootstrapFormValues): Promise<Msg<NodeBootstrapJob>> => {
      const raw = await HttpUtil.post('/panel/api/nodes/bootstrap', payload);
      return parseMsg(raw, NodeBootstrapJobSchema, 'nodes/bootstrap');
    },
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const bootstrapStatus = useCallback(async (id: string): Promise<Msg<NodeBootstrapJob>> => {
    const raw = await HttpUtil.get(`/panel/api/nodes/bootstrap/${id}`);
    return parseMsg(raw, NodeBootstrapJobSchema, 'nodes/bootstrap/status');
  }, []);

  const updatePanelsMut = useMutation({
    mutationFn: (ids: number[]) =>
      HttpUtil.post<NodeUpdateResult[]>('/panel/api/nodes/updatePanel', { ids }, {
        headers: { 'Content-Type': 'application/json' },
      }),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const reinstallMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/nodes/reinstall/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const rotateMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/nodes/rotateCredentials/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const reconcileMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/nodes/reconcile/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const fetchFirewallRules = useCallback(async (id: number): Promise<Msg<UfwStatus>> => {
    const raw = await HttpUtil.get(`/panel/api/nodes/ufw/${id}`);
    return parseMsg(raw, UfwStatusSchema, 'nodes/ufw');
  }, []);

  const allowFirewallPort = useCallback(async (id: number, port: number, protocol: string) => {
    return HttpUtil.post(`/panel/api/nodes/ufw/${id}/allow`, { port, protocol });
  }, []);

  const denyFirewallPort = useCallback(async (id: number, port: number, protocol: string) => {
    return HttpUtil.post(`/panel/api/nodes/ufw/${id}/deny`, { port, protocol });
  }, []);

  const deleteFirewallRule = useCallback(async (id: number, ruleNumber: string) => {
    return HttpUtil.post(`/panel/api/nodes/ufw/${id}/delete`, { rule_number: ruleNumber });
  }, []);

  const enableFirewall = useCallback(async (id: number) => {
    return HttpUtil.post(`/panel/api/nodes/ufw/${id}/enable`);
  }, []);

  const disableFirewall = useCallback(async (id: number) => {
    return HttpUtil.post(`/panel/api/nodes/ufw/${id}/disable`);
  }, []);

  const restartAgentMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/nodes/restart/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const restartXrayMut = useMutation({
    mutationFn: (id: number) => HttpUtil.post(`/panel/api/nodes/xray/restart/${id}`),
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const pushConfigMut = useMutation({
    mutationFn: async (id: number): Promise<Msg<PushConfigResult>> => {
      const raw = await HttpUtil.post(`/panel/api/nodes/pushConfig/${id}`, {});
      return parseMsg(raw, PushConfigResultSchema, 'nodes/pushConfig');
    },
    onSuccess: (msg) => { if (msg?.success) invalidate(); },
  });

  const fetchPortGroups = useCallback(async (): Promise<Msg<PortGroup[]>> => {
    const raw = await HttpUtil.get('/panel/api/nodes/portGroup/list');
    return parseMsg(raw, PortGroupListSchema, 'nodes/portGroup/list');
  }, []);

  const createPortGroup = useCallback(async (name: string, ports: { port: number; protocol: string; comment?: string }[]) => {
    return HttpUtil.post('/panel/api/nodes/portGroup/add', { name, ports });
  }, []);

  const updatePortGroup = useCallback(async (id: number, name: string, ports: { port: number; protocol: string; comment?: string }[]) => {
    return HttpUtil.post(`/panel/api/nodes/portGroup/update/${id}`, { name, ports });
  }, []);

  const deletePortGroup = useCallback(async (id: number) => {
    return HttpUtil.post(`/panel/api/nodes/portGroup/del/${id}`);
  }, []);

  const pushPortGroup = useCallback(async (portGroupId: number, nodeGroup: string) => {
    return HttpUtil.post(`/panel/api/nodes/portGroup/push/${portGroupId}/${nodeGroup}`);
  }, []);

  const fetchNodeGroups = useCallback(async (): Promise<Msg<string[]>> => {
    return HttpUtil.get<string[]>('/panel/api/nodes/groups');
  }, []);

  const setNodeGroup = useCallback(async (nodeId: number, group: string) => {
    return HttpUtil.post(`/panel/api/nodes/${nodeId}/setGroup`, { group });
  }, []);

  const fetchLogs = useCallback(async (id: number, lines = 200): Promise<Msg<string>> => {
    return HttpUtil.get<string>(`/panel/api/nodes/logs/${id}?lines=${lines}`);
  }, []);

  return {
    create: (payload: Partial<NodeRecord>) => createMut.mutateAsync(payload),
    update: (id: number, payload: Partial<NodeRecord>) => updateMut.mutateAsync({ id, payload }),
    remove: (id: number, cleanupRemote = false) => removeMut.mutateAsync({ id, cleanupRemote }),
    setEnable: (id: number, enable: boolean) => setEnableMut.mutateAsync({ id, enable }),
    probe: (id: number) => probeMut.mutateAsync(id),
    bootstrap: (payload: NodeBootstrapFormValues) => bootstrapMut.mutateAsync(payload),
    bootstrapStatus,
    updatePanels: (ids: number[]): Promise<Msg<NodeUpdateResult[]>> => updatePanelsMut.mutateAsync(ids),
    reinstall: (id: number) => reinstallMut.mutateAsync(id),
    rotateCredentials: (id: number) => rotateMut.mutateAsync(id),
    reconcile: (id: number) => reconcileMut.mutateAsync(id),
    fetchFirewallRules,
    allowFirewallPort,
    denyFirewallPort,
    deleteFirewallRule,
    enableFirewall,
    disableFirewall,
    testConnection: async (payload: Partial<NodeRecord>): Promise<Msg<ProbeResult>> => {
      const raw = await HttpUtil.post('/panel/api/nodes/test', payload);
      return parseMsg(raw, ProbeResultSchema, 'nodes/test');
    },
    fetchFingerprint: (payload: Partial<NodeRecord>): Promise<Msg<string>> =>
      HttpUtil.post<string>('/panel/api/nodes/certFingerprint', payload),
    restartAgent: (id: number) => restartAgentMut.mutateAsync(id),
    restartXray: (id: number) => restartXrayMut.mutateAsync(id),
    fetchLogs,
    pushConfig: (id: number) => pushConfigMut.mutateAsync(id),
    fetchPortGroups,
    createPortGroup,
    updatePortGroup,
    deletePortGroup,
    pushPortGroup,
    fetchNodeGroups,
    setNodeGroup,
  };
}
