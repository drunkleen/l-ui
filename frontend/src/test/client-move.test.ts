import { describe, expect, it } from 'vitest';

import { ClientMoveResultSchema, InboundOptionSchema } from '@/schemas/client';

describe('client move schemas', () => {
  it('parses inbound node ids', () => {
    const parsed = InboundOptionSchema.parse({
      id: 7,
      nodeId: 3,
      remark: 'vless-443',
      protocol: 'vless',
    });

    expect(parsed.nodeId).toBe(3);
    expect(parsed.remark).toBe('vless-443');
  });

  it('parses bulk move result shapes', () => {
    const parsed = ClientMoveResultSchema.parse({
      moved: ['alice@example.com'],
      skipped: [{ email: 'bob@example.com', reason: 'not on source node' }],
      errors: [],
    });

    expect(parsed.moved).toEqual(['alice@example.com']);
    expect(parsed.skipped?.[0]?.email).toBe('bob@example.com');
  });
});
