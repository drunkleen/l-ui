import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders } from './test-utils';
import { Status } from '@/models/status';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

vi.mock('@/api/queries/useServerHistorySeries', () => ({
  useServerHistorySeries: (opts: { metric: string }) => {
    const series: Record<string, number[]> = {
      cpu: [12, 18, 16],
      mem: [41, 43, 45],
      diskUsage: [55, 56, 57],
      netUp: [1024, 2048, 1536],
      netDown: [4096, 2048, 3072],
      load1: [0.5, 0.6, 0.7],
      load5: [0.4, 0.5, 0.6],
      load15: [0.3, 0.4, 0.5],
    };
    return {
      labels: ['00:00', '00:10', '00:20'],
      data: series[opts.metric] ?? [],
      data2: opts.metric === 'netUp' ? series.netDown : opts.metric === 'load1' ? series.load5 : [],
      data3: opts.metric === 'load1' ? series.load15 : [],
      loading: false,
      error: '',
      refetch: vi.fn(),
    };
  },
}));

describe('HubOverview', () => {
  it('renders host summary and node cards', { timeout: 15000 }, async () => {
    window.L_UI_BASE_PATH = '/';
    const { default: HubOverview } = await import('@/pages/index/HubOverview');

    const status = new Status({
      cpu: 18.4,
      mem: { current: 1024, total: 2048 },
      disk: { current: 512, total: 1024 },
      netIO: { up: 2048, down: 4096 },
      uptime: 3600,
      loads: [0.5, 0.4, 0.3],
    });

    const node: NodeRecord = {
      id: 1,
      name: 'de-fra-1',
      address: '203.0.113.10',
      port: 2053,
      status: 'online',
      cpuPct: 25.2,
      memPct: 62.8,
      diskCurrent: 600,
      diskTotal: 1000,
      latencyMs: 28,
      lastHeartbeat: Math.floor(Date.now() / 1000) - 120,
      netUp: 4096,
      netDown: 8192,
    };

    renderWithProviders(
      <HubOverview
        context={{ mode: 'hub', version: 'v1.2.3', dbType: 'sqlite', apiPrefix: '/panel/api', localXrayEnabled: false }}
        status={status}
        nodes={[node]}
      />,
    );

    expect(document.body.textContent || '').toContain('Host summary');
    expect(document.body.textContent || '').toContain('VPS nodes (1)');
    expect(document.body.textContent || '').toContain('de-fra-1');
    expect(document.body.textContent || '').toContain('Stale');
  });
});
