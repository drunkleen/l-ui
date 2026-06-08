import { z } from 'zod';

export const NodeRecordSchema = z.object({
  id: z.number(),
  name: z.string().optional(),
  scheme: z.string().optional(),
  address: z.string().optional(),
  port: z.number().optional(),
  basePath: z.string().optional(),
  apiToken: z.string().optional(),
  enable: z.boolean().optional(),
  status: z.string().optional(),
  latencyMs: z.number().optional(),
  cpuPct: z.number().optional(),
  memPct: z.number().optional(),
  diskCurrent: z.number().optional(),
  diskTotal: z.number().optional(),
  netUp: z.number().optional(),
  netDown: z.number().optional(),
  xrayVersion: z.string().optional(),
  panelVersion: z.string().optional(),
  uptimeSecs: z.number().optional(),
  inboundCount: z.number().optional(),
  clientCount: z.number().optional(),
  onlineCount: z.number().optional(),
  depletedCount: z.number().optional(),
  lastHeartbeat: z.number().optional(),
  lastError: z.string().optional(),
  allowPrivateAddress: z.boolean().optional(),
  tlsVerifyMode: z.enum(['verify', 'skip', 'pin']).optional(),
  pinnedCertSha256: z.string().optional(),
  group: z.string().optional(),
  configVersion: z.number().optional(),
}).loose();

export const NodeListSchema = z.array(NodeRecordSchema);

export const ProbeResultSchema = z.object({
  status: z.string(),
  latencyMs: z.number().optional(),
  xrayVersion: z.string().optional(),
  error: z.string().optional(),
}).loose();

export const UfwRuleSchema = z.object({
  number: z.number(),
  port: z.string(),
  protocol: z.string().optional(),
  action: z.string(),
  comment: z.string().optional(),
}).loose();

export const UfwStatusSchema = z.object({
  active: z.boolean().optional(),
  installed: z.boolean().optional(),
  rules: z.array(UfwRuleSchema),
}).loose();

export const PushConfigResultSchema = z.object({
  config_version: z.number().optional(),
}).loose();

export const PortGroupEntrySchema = z.object({
  port: z.number().int().min(1).max(65535),
  protocol: z.string(),
  comment: z.string().optional(),
}).loose();

export const PortGroupSchema = z.object({
  id: z.number(),
  name: z.string(),
  ports: z.string(),
  createdAt: z.number().optional(),
  updatedAt: z.number().optional(),
}).loose();

export const PortGroupListSchema = z.array(PortGroupSchema);

export const NodeBootstrapStepSchema = z.object({
  name: z.string(),
  ok: z.boolean(),
  output: z.string().optional(),
}).loose();

export const NodeBootstrapResultSchema = z.object({
  node: NodeRecordSchema,
  steps: z.array(NodeBootstrapStepSchema),
}).loose();

export const NodeBootstrapJobSchema = z.object({
  id: z.string(),
  state: z.enum(['queued', 'running', 'done', 'failed']),
  step: z.string().optional(),
  error: z.string().optional(),
  node: NodeRecordSchema.optional(),
  steps: z.array(NodeBootstrapStepSchema).optional(),
}).loose();

export const NodeFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  scheme: z.enum(['http', 'https']),
  address: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  port: z.number().int().min(1).max(65535),
  basePath: z.string(),
  apiToken: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  enable: z.boolean(),
  allowPrivateAddress: z.boolean(),
  tlsVerifyMode: z.enum(['verify', 'skip', 'pin']),
  pinnedCertSha256: z.string().optional().default(''),
  group: z.string().optional().default(''),
});

export const NodeBootstrapFormSchema = z.object({
  name: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  address: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  sshUser: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  sshPassword: z.string().min(1, 'pages.nodes.toasts.fillRequired'),
  useTLS: z.boolean().default(false),
  domain: z.string().trim().optional().default(''),
  acmeEmail: z.string().trim().optional().default(''),
  sshPort: z.number().int().min(1).max(65535).default(22),
  agentPort: z.preprocess(
    (value) => (value === '' || value == null ? undefined : value),
    z.number().int().min(1).max(65535).optional(),
  ),
  bootstrapBase: z.string().optional().default('/'),
});

export type NodeRecord = z.infer<typeof NodeRecordSchema>;
export type ProbeResult = z.infer<typeof ProbeResultSchema>;
export type UfwRule = z.infer<typeof UfwRuleSchema>;
export type UfwStatus = z.infer<typeof UfwStatusSchema>;
export type NodeFormValues = z.infer<typeof NodeFormSchema>;
export type PushConfigResult = z.infer<typeof PushConfigResultSchema>;
export type PortGroupEntry = z.infer<typeof PortGroupEntrySchema>;
export type PortGroup = z.infer<typeof PortGroupSchema>;
export type NodeBootstrapStep = z.infer<typeof NodeBootstrapStepSchema>;
export type NodeBootstrapResult = z.infer<typeof NodeBootstrapResultSchema>;
export type NodeBootstrapJob = z.infer<typeof NodeBootstrapJobSchema>;
export type NodeBootstrapFormValues = z.infer<typeof NodeBootstrapFormSchema>;
