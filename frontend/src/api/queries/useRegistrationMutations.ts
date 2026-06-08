import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import { z } from 'zod';

const RegistrationTokenSchema = z.object({
  id: z.number(),
  token: z.string(),
  nodeName: z.string().optional(),
  nodeAddress: z.string().optional(),
  consumedByNodeId: z.number().optional(),
  consumedAt: z.number().optional(),
  expiresAt: z.number(),
  createdAt: z.number(),
}).loose();

const RegistrationTokenListSchema = z.array(RegistrationTokenSchema);

export type RegistrationToken = z.infer<typeof RegistrationTokenSchema>;

export function useRegistrationTokens() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.nodes.registrationTokens() });

  const list = useQuery({
    queryKey: keys.nodes.registrationTokens(),
    queryFn: async () => {
      const msg = await HttpUtil.get<unknown>('/panel/api/node-registration/list');
      if (!msg?.success || !msg.obj) return [];
      const parsed = RegistrationTokenListSchema.safeParse(msg.obj);
      return parsed.success ? parsed.data : [];
    },
    staleTime: 10_000,
  });

  const generateMut = useMutation({
    mutationFn: (params: { nodeName?: string; nodeAddress?: string; ttlMinutes?: number }) =>
      HttpUtil.post('/panel/api/node-registration/generate', params),
    onSuccess: () => invalidate(),
  });

  const deleteMut = useMutation({
    mutationFn: (id: number) => HttpUtil.del(`/panel/api/node-registration/${id}`),
    onSuccess: () => invalidate(),
  });

  return {
    tokens: list.data ?? [],
    loading: list.isLoading,
    refetch: list.refetch,
    generate: (params: { nodeName?: string; nodeAddress?: string; ttlMinutes?: number }) =>
      generateMut.mutateAsync(params),
    remove: (id: number) => deleteMut.mutateAsync(id),
  };
}
