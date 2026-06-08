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

describe('ClientsPage fetch flow', () => {
  it('keeps loaded clients visible when a refetch warning happens', { timeout: 30000 }, async () => {
    mockUseClients.mockReturnValue({
      clients: [
        {
          email: 'alice@example.com',
          enable: true,
          totalGB: 0,
          expiryTime: 0,
          limitIp: 0,
          reset: 0,
          inboundIds: [],
          traffic: null,
          createdAt: Date.now(),
          updatedAt: Date.now(),
        },
      ],
      total: 1,
      filtered: 1,
      hasListData: true,
      summary: { total: 1, active: 1, online: [], depleted: [], expiring: [], deactive: [] },
      allGroups: [],
      setQuery: vi.fn(),
      inbounds: [],
      onlines: [],
      loading: false,
      fetched: true,
      fetchError: 'temporary network hiccup',
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

    expect(document.body.textContent || '').toContain('alice@example.com');
    expect(document.body.textContent || '').toContain('temporary network hiccup');
  });
});
