import { describe, expect, it } from 'vitest';

import { UfwStatusSchema } from '@/schemas/node';

describe('node firewall schema', () => {
  it('parses ufw status payloads', () => {
    const parsed = UfwStatusSchema.safeParse({
      enabled: true,
      rules: [
        { number: 1, port: '2053', protocol: 'tcp', action: 'ALLOW IN' },
        { number: 2, port: '2053', protocol: 'udp', action: 'DENY IN' },
      ],
    });

    expect(parsed.success).toBe(true);
    expect(parsed.data?.rules[0].port).toBe('2053');
  });
});
