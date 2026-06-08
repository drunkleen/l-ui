import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import { getApiPrefix, getPanelMode } from '@/utils/runtime';

export interface PanelContext {
  mode: 'hub' | 'agent';
  version: string;
  dbType: string;
  apiPrefix: string;
  localXrayEnabled: boolean;
}

async function fetchPanelContext(): Promise<PanelContext> {
  const msg = await HttpUtil.get<PanelContext>('/panel/api/server/context', undefined, { silent: true });
  if (!msg?.success || !msg.obj) {
    throw new Error(msg?.msg || 'Failed to fetch panel context');
  }
  return msg.obj;
}

function fallbackContext(): PanelContext {
  const mode = getPanelMode();
  return {
    mode,
    version: window.L_UI_CUR_VER || '',
    dbType: window.L_UI_DB_TYPE || '',
    apiPrefix: getApiPrefix(),
    localXrayEnabled: mode === 'agent',
  };
}

export function usePanelContext() {
  const query = useQuery({
    queryKey: keys.server.context(),
    queryFn: fetchPanelContext,
    staleTime: 5 * 60 * 1000,
  });

  const context = useMemo(() => query.data ?? fallbackContext(), [query.data]);

  return {
    context,
    loading: query.isFetching && !query.data,
    error: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}
