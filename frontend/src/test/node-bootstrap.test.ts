import { describe, expect, it } from 'vitest';
import { NodeBootstrapFormSchema, NodeBootstrapJobSchema } from '@/schemas/node';

describe('node bootstrap schemas', () => {
  it('fills bootstrap defaults', () => {
    const parsed = NodeBootstrapFormSchema.parse({
      name: 'de-fra-1',
      address: '203.0.113.10',
      sshUser: 'root',
      sshPassword: 'secret',
    });

    expect(parsed.sshPort).toBe(22);
    expect(parsed.agentPort).toBeUndefined();
    expect(parsed.bootstrapBase).toBe('/');
    expect(parsed.useTLS).toBe(false);
    expect(parsed.domain).toBe('');
  });

  it('accepts an explicit agent port', () => {
    const parsed = NodeBootstrapFormSchema.parse({
      name: 'de-fra-1',
      address: '203.0.113.10',
      sshUser: 'root',
      sshPassword: 'secret',
      agentPort: 2443,
    });

    expect(parsed.agentPort).toBe(2443);
  });

  it('parses tls bootstrap fields', () => {
    const parsed = NodeBootstrapFormSchema.parse({
      name: 'de-fra-1',
      address: '203.0.113.10',
      sshUser: 'root',
      sshPassword: 'secret',
      useTLS: true,
      domain: 'node.example.com',
      acmeEmail: 'admin@example.com',
    });

    expect(parsed.useTLS).toBe(true);
    expect(parsed.domain).toBe('node.example.com');
    expect(parsed.acmeEmail).toBe('admin@example.com');
  });

  it('parses bootstrap job state', () => {
    const parsed = NodeBootstrapJobSchema.parse({
      id: 'bootstrap-xyz',
      state: 'running',
      step: 'download-install',
      steps: [
        { name: 'detect-arch', ok: true, output: 'x86_64' },
      ],
    });

    expect(parsed.id).toBe('bootstrap-xyz');
    expect(parsed.state).toBe('running');
    expect(parsed.steps?.[0]?.name).toBe('detect-arch');
  });
});
