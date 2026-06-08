import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from './test-utils';

const mockUseClients = vi.fn();

vi.mock('@/hooks/useClients', () => ({
  useClients: () => mockUseClients(),
}));

vi.mock('@/api/queries/useNodesQuery', () => ({
  useNodesQuery: () => ({ nodes: [] }),
}));

vi.mock('@/hooks/useTheme', () => ({
  useTheme: () => ({
    isDark: false,
    isUltra: false,
    toggleTheme: vi.fn(),
    toggleUltra: vi.fn(),
    antdThemeConfig: {},
  }),
  pauseAnimationsUntilLeave: vi.fn(),
  ThemeProvider: ({ children }: { children: ReactNode }) => children,
}));

vi.mock('@/hooks/useMediaQuery', () => ({
  useMediaQuery: () => ({ isMobile: false }),
}));

vi.mock('@/hooks/useDatepicker', () => ({
  useDatepicker: () => ({ datepicker: 'en' }),
}));

vi.mock('@/hooks/useWebSocket', () => ({
  useWebSocket: vi.fn(),
}));

vi.mock('@/layouts/AppSidebar', () => ({
  default: () => null,
}));

vi.mock('@/components/utility', () => ({
  LazyMount: ({ when, children }: { when?: boolean; children: ReactNode }) => (when ? children : null),
}));

describe('ClientsPage loading state', () => {
  it('renders the loading spacer before data arrives', { timeout: 15000 }, async () => {
    mockUseClients.mockReturnValue({
      clients: [],
      total: 0,
      filtered: 0,
      hasListData: false,
      summary: { total: 0, active: 0, online: [], depleted: [], expiring: [], deactive: [] },
      allGroups: [],
      setQuery: vi.fn(),
      inbounds: [],
      onlines: [],
      loading: true,
      fetched: false,
      fetchError: '',
      subSettings: { enable: false, subURI: '', subJsonURI: '', subJsonEnable: false, subClashURI: '', subClashEnable: false },
      ipLimitEnable: false,
      tgBotEnable: false,
      expireDiff: 0,
      trafficDiff: 0,
      pageSize: 25,
      create: vi.fn(),
      bulkCreate: vi.fn(),
      update: vi.fn(),
      remove: vi.fn(),
      bulkDelete: vi.fn(),
      bulkAdjust: vi.fn(),
      bulkAddToGroup: vi.fn(),
      bulkRemoveFromGroup: vi.fn(),
      attach: vi.fn(),
      bulkAttach: vi.fn(),
      detach: vi.fn(),
      bulkDetach: vi.fn(),
      move: vi.fn(),
      bulkMove: vi.fn(),
      resetTraffic: vi.fn(),
      resetAllTraffics: vi.fn(),
      delDepleted: vi.fn(),
      setEnable: vi.fn(),
      applyTrafficEvent: vi.fn(),
      applyClientStatsEvent: vi.fn(),
      refresh: vi.fn(),
      hydrate: vi.fn(),
    });

    const { default: ClientsPage } = await import('@/pages/clients/ClientsPage');
    renderWithProviders(<ClientsPage />);

    expect(document.querySelector('.loading-spacer')).toBeTruthy();
  });
});
