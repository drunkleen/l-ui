import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, fieldLabels } from './test-utils';
import { NodeBootstrapFormSchema } from '@/schemas/node';

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

describe('NodeFormModal bootstrap flow', () => {
  it('keeps the agent port in the add-node payload', { timeout: 15000 }, async () => {
    const { default: NodeFormModal } = await import('@/pages/nodes/NodeFormModal');

    renderWithProviders(
      <NodeFormModal
        open
        mode="add"
        node={null}
        testConnection={vi.fn()}
        fetchFingerprint={vi.fn()}
        save={vi.fn()}
        bootstrap={vi.fn()}
        bootstrapStatus={vi.fn()}
        onOpenChange={vi.fn()}
      />,
    );

    expect(fieldLabels().length).toBeGreaterThan(0);
    const parsed = NodeBootstrapFormSchema.parse({
      name: 'node-1',
      address: '203.0.113.10',
      sshUser: 'root',
      sshPassword: 'secret',
      useTLS: false,
      domain: '',
      acmeEmail: '',
      sshPort: 2222,
      agentPort: 2443,
      bootstrapBase: '/',
    });
    expect(parsed.agentPort).toBe(2443);
  });
});
