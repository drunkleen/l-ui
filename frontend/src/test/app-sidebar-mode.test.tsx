import type { ReactNode } from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from './test-utils';

const navigate = vi.fn();

vi.mock('@/utils/runtime', () => ({
  getPanelMode: () => window.L_UI_MODE ?? 'hub',
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

vi.mock('@/api/queries/useNodesQuery', () => ({
  useNodesQuery: () => ({ nodes: [] }),
}));

vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: () => ({
    allSetting: {
      subJsonEnable: false,
      subClashEnable: false,
    },
  }),
}));

vi.mock('@/utils', () => ({
  HttpUtil: {
    post: vi.fn(),
  },
}));

vi.mock('react-router-dom', () => ({
  useLocation: () => ({ pathname: '/', hash: '' }),
  useNavigate: () => navigate,
}));

describe('AppSidebar hub mode', () => {
  beforeEach(() => {
    navigate.mockClear();
    window.L_UI_MODE = 'hub';
  });

  it('hides the Xray menu entry in hub mode', { timeout: 15000 }, async () => {
    const { default: AppSidebar } = await import('@/layouts/AppSidebar');
    renderWithProviders(<AppSidebar />);

    expect(document.body.textContent || '').not.toMatch(/xray/i);
  });

  it('shows the Xray menu entry in agent mode', { timeout: 15000 }, async () => {
    window.L_UI_MODE = 'agent';
    const { default: AppSidebar } = await import('@/layouts/AppSidebar');
    renderWithProviders(<AppSidebar />);

    expect(document.body.textContent || '').toMatch(/xray/i);
  });
});
