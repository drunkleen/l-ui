import { describe, expect, it } from 'vitest';

import { ClientPageResponseSchema } from '@/schemas/client';

describe('client page schema', () => {
  it('accepts the slim list payload shape from the backend', () => {
    const parsed = ClientPageResponseSchema.safeParse({
      items: [
        {
          email: 'alice@example.com',
          subId: 'sub-1',
          enable: true,
          totalGB: 1024,
          expiryTime: 0,
          limitIp: 1,
          reset: 0,
          group: 'team-a',
          comment: 'primary',
          inboundIds: [1, 2],
          traffic: { up: 10, down: 20, total: 1024, expiryTime: 0, enable: true },
          createdAt: 1,
          updatedAt: 2,
        },
      ],
      total: 1,
      filtered: 1,
      page: 1,
      pageSize: 25,
      summary: { total: 1, active: 1, online: [], depleted: [], expiring: [], deactive: [] },
      groups: ['team-a'],
    });

    expect(parsed.success).toBe(true);
    expect(parsed.data?.items[0].inboundIds).toEqual([1, 2]);
  });
});
