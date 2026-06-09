import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Col,
  Collapse,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Steps,
  Switch,
  Typography,
  message,
} from 'antd';
import {
  DownloadOutlined,
} from '@ant-design/icons';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { Msg } from '@/utils';
import {
  NodeBootstrapFormSchema,
  NodeFormSchema,
  type NodeBootstrapFormValues,
  type NodeBootstrapJob,
  type NodeFormValues,
  type ProbeResult,
} from '@/schemas/node';
import { antdRule } from '@/utils/zodForm';
import './NodeFormModal.css';

type Mode = 'add' | 'edit';

interface NodeFormModalProps {
  open: boolean;
  mode: Mode;
  node: NodeRecord | null;
  testConnection: (payload: Partial<NodeRecord>) => Promise<Msg<ProbeResult>>;
  fetchFingerprint: (payload: Partial<NodeRecord>) => Promise<Msg<string>>;
  save: (payload: Partial<NodeRecord>) => Promise<Msg<unknown>>;
  bootstrap: (payload: NodeBootstrapFormValues) => Promise<Msg<NodeBootstrapJob>>;
  bootstrapStatus: (id: string) => Promise<Msg<NodeBootstrapJob>>;
  onOpenChange: (open: boolean) => void;
}

type FormValues = NodeFormValues & Partial<NodeBootstrapFormValues>;

function defaultValues(): FormValues {
  return {
    id: 0, name: '', scheme: 'https', address: '', port: 2053,
    basePath: '/', apiToken: '', enable: true, allowPrivateAddress: false,
    tlsVerifyMode: 'verify', pinnedCertSha256: '',
    sshUser: '', sshPassword: '', useTLS: false, domain: '', acmeEmail: '',
    sshPort: 22, bootstrapBase: '/', group: '',
  };
}

export default function NodeFormModal({
  open, mode, node, testConnection, fetchFingerprint, save,
  bootstrap, bootstrapStatus, onOpenChange,
}: NodeFormModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<FormValues>();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [fetchingPin, setFetchingPin] = useState(false);
  const [testResult, setTestResult] = useState<ProbeResult | null>(null);
  const [bootstrapJob, setBootstrapJob] = useState<NodeBootstrapJob | null>(null);

  useEffect(() => {
    if (bootstrapJob?.state === 'done') close();
  }, [bootstrapJob]);

  const scheme = Form.useWatch('scheme', form) ?? 'https';
  const tlsVerifyMode = Form.useWatch('tlsVerifyMode', form) ?? 'verify';
  const useTLS = Form.useWatch('useTLS', form) ?? false;

  useEffect(() => {
    if (!open) return;
    const base = defaultValues();
    const next: NodeFormValues = mode === 'edit' && node
      ? { ...base, ...(node as unknown as Partial<NodeFormValues>), id: node.id, scheme: (node.scheme as 'http' | 'https') || base.scheme }
      : base;
    if (next.scheme === 'http') next.tlsVerifyMode = 'skip';
    form.resetFields();
    form.setFieldsValue(next);
    setTestResult(null);
    setBootstrapJob(null);
  }, [open, mode, node, form]);

  useEffect(() => {
    if (!open || !bootstrapJob || (bootstrapJob.state !== 'queued' && bootstrapJob.state !== 'running')) return;
    let cancelled = false;
    const refresh = async () => {
      const msg = await bootstrapStatus(bootstrapJob.id);
      if (cancelled || !msg?.success || !msg.obj) return;
      setBootstrapJob(msg.obj);
    };
    void refresh();
    const timer = window.setInterval(refresh, 1200);
    return () => { cancelled = true; window.clearInterval(timer); };
  }, [open, bootstrapJob, bootstrapStatus]);

  const title = useMemo(
    () => (mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode')),
    [mode, t],
  );

  function buildPayload(values: NodeFormValues): Partial<NodeRecord> {
    return {
      id: values.id || 0, name: values.name.trim(), scheme: values.scheme,
      address: values.address.trim(), port: values.port,
      basePath: values.basePath.trim() || '/', apiToken: values.apiToken.trim(),
      enable: values.enable, allowPrivateAddress: values.allowPrivateAddress,
      tlsVerifyMode: values.tlsVerifyMode,
      pinnedCertSha256: values.tlsVerifyMode === 'pin' ? values.pinnedCertSha256.trim() : '',
    };
  }

  function buildBootstrapPayload(values: FormValues): NodeBootstrapFormValues {
    const result = NodeBootstrapFormSchema.parse(values);
    return {
      name: result.name, address: result.address, sshUser: result.sshUser,
      sshPassword: result.sshPassword, useTLS: result.useTLS,
      domain: result.domain, acmeEmail: result.acmeEmail,
      sshPort: result.sshPort, agentPort: result.agentPort,
      bootstrapBase: result.bootstrapBase || '/',
    };
  }

  async function onTest() {
    try { await form.validateFields(['address', 'port']); } catch { return; }
    setTesting(true);
    setTestResult(null);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
      const msg = await testConnection(payload);
      setTestResult(msg?.success && msg.obj
        ? msg.obj
        : { status: 'offline', error: msg?.msg || 'unknown error' });
    } finally { setTesting(false); }
  }

  async function onFetchPin() {
    try { await form.validateFields(['address', 'port']); } catch { return; }
    setFetchingPin(true);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
      const msg = await fetchFingerprint(payload);
      if (msg?.success && msg.obj) {
        form.setFieldValue('pinnedCertSha256', msg.obj);
        messageApi.success(t('pages.nodes.pinFetched'));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.pinFetchFailed'));
      }
    } finally { setFetchingPin(false); }
  }

  async function onFinish(values: FormValues) {
    setSubmitting(true);
    try {
      if (mode === 'edit') {
        const result = NodeFormSchema.safeParse(values);
        if (!result.success) {
          messageApi.error(t(result.error.issues[0]?.message ?? 'pages.nodes.toasts.fillRequired'));
          return;
        }
        const payload = buildPayload(result.data);
        const test = await testConnection(payload);
        const probe = test?.success ? test.obj : null;
        if (!probe || probe.status !== 'online') {
          setTestResult(probe ?? { status: 'offline', error: test?.msg || t('pages.nodes.connectionFailed') });
          return;
        }
        setTestResult(probe);
        const msg = await save(payload);
        if (msg?.success) onOpenChange(false);
        return;
      }
      const payload = buildBootstrapPayload(values);
      if (payload.useTLS && !payload.domain.trim()) {
        messageApi.error(t('pages.nodes.domainRequired'));
        return;
      }
      const msg = await bootstrap(payload);
      setBootstrapJob(msg?.obj ?? null);
      if (msg?.success) {
        messageApi.success(msg.msg || t('pages.nodes.bootstrapQueued'));
      } else {
        messageApi.error(msg?.msg || t('pages.nodes.bootstrapFailed'));
      }
    } finally { setSubmitting(false); }
  }

  function close() { if (!submitting) onOpenChange(false); }

  const onOk = useCallback(() => {
    if (mode === 'add' && bootstrapJob?.state === 'done') { close(); return; }
    form.submit();
  }, [mode, bootstrapJob, form]);

  const isRunning = bootstrapJob?.state === 'queued' || bootstrapJob?.state === 'running';

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        confirmLoading={submitting || isRunning}
        okText={mode === 'add' && bootstrapJob?.state === 'done' ? t('close') : (mode === 'add' ? t('pages.nodes.bootstrapNode') : t('save'))}
        cancelText={t('cancel')}
        mask={{ closable: false }}
        width={mode === 'add' ? '680px' : '720px'}
        className="node-form-modal"
        onOk={onOk}
        onCancel={close}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={defaultValues()}
          onFinish={onFinish}
        >
          <Form.Item
            label={t('pages.nodes.name')}
            name="name"
            rules={[antdRule(mode === 'add' ? NodeBootstrapFormSchema.shape.name : NodeFormSchema.shape.name, t)]}
          >
            <Input placeholder={t('pages.nodes.namePlaceholder')} />
          </Form.Item>

          {mode === 'add' ? (
            <div className="bootstrap-form">
              <Row gutter={8}>
                <Col span={10}>
                  <Form.Item label="SSH Host" name="address" rules={[antdRule(NodeBootstrapFormSchema.shape.address, t)]}>
                    <Input placeholder="192.168.1.1" />
                  </Form.Item>
                </Col>
                <Col span={7}>
                  <Form.Item label="User" name="sshUser" rules={[antdRule(NodeBootstrapFormSchema.shape.sshUser, t)]}>
                    <Input placeholder="root" />
                  </Form.Item>
                </Col>
                <Col span={7}>
                  <Form.Item label="Password" name="sshPassword" rules={[antdRule(NodeBootstrapFormSchema.shape.sshPassword, t)]}>
                    <Input.Password />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={8}>
                <Col span={8}>
                  <Form.Item label="SSH Port" name="sshPort" rules={[antdRule(NodeBootstrapFormSchema.shape.sshPort, t)]}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label="Agent Port" name="agentPort" rules={[antdRule(NodeBootstrapFormSchema.shape.agentPort, t)]}>
                    <InputNumber min={1} max={65535} placeholder="auto" style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label=" " colon={false}>
                    <Space style={{ paddingTop: 4 }}>
                      <Form.Item name="useTLS" valuePropName="checked" noStyle>
                        <Switch size="small" />
                      </Form.Item>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>TLS</Typography.Text>
                    </Space>
                  </Form.Item>
                </Col>
              </Row>

              {useTLS && (
                <div className="tls-fields">
                  <Row gutter={8}>
                    <Col span={12}>
                      <Form.Item label={t('pages.nodes.domain')} name="domain" rules={[{ required: true, message: t('pages.nodes.domainRequired') }, antdRule(NodeBootstrapFormSchema.shape.domain, t)]}>
                        <Input placeholder={t('pages.nodes.domainPlaceholder')} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item label={t('pages.nodes.acmeEmail')} name="acmeEmail">
                        <Input placeholder={t('pages.nodes.acmeEmailPlaceholder')} />
                      </Form.Item>
                    </Col>
                  </Row>
                </div>
              )}

              {bootstrapJob && <BootstrapTimeline job={bootstrapJob} />}
            </div>
          ) : (
            <div className="edit-form">
              <Row gutter={8}>
                <Col span={5}>
                  <Form.Item label={t('pages.nodes.scheme')} name="scheme">
                    <Select
                      options={[{ value: 'https', label: 'https' }, { value: 'http', label: 'http' }]}
                      onChange={(v) => { if (v === 'http') form.setFieldValue('tlsVerifyMode', 'skip'); }}
                    />
                  </Form.Item>
                </Col>
                <Col span={14}>
                  <Form.Item label={t('pages.nodes.address')} name="address" rules={[antdRule(NodeFormSchema.shape.address, t)]}>
                    <Input placeholder={t('pages.nodes.addressPlaceholder')} />
                  </Form.Item>
                </Col>
                <Col span={5}>
                  <Form.Item label={t('pages.nodes.port')} name="port" rules={[antdRule(NodeFormSchema.shape.port, t)]}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={8}>
                <Col span={12}>
                  <Form.Item label={t('pages.nodes.basePath')} name="basePath">
                    <Input placeholder="/" />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item label={t('pages.nodes.enable')} name="enable" valuePropName="checked">
                    <Switch />
                  </Form.Item>
                </Col>
                <Col span={6}>
                  <Form.Item label={t('pages.nodes.allowPrivateAddress')} name="allowPrivateAddress" valuePropName="checked">
                    <Switch />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={8}>
                <Col span={8}>
                  <Form.Item label={t('pages.nodes.tlsVerifyMode')} name="tlsVerifyMode">
                    <Select
                      disabled={scheme === 'http'}
                      options={[
                        { value: 'verify', label: t('pages.nodes.tlsVerify') },
                        { value: 'pin', label: t('pages.nodes.tlsPin') },
                        { value: 'skip', label: t('pages.nodes.tlsSkip') },
                      ]}
                    />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label={t('pages.nodes.group')} name="group">
                    <Input placeholder={t('pages.nodes.groupPlaceholder')} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item label={t('pages.nodes.apiToken')} name="apiToken" rules={[antdRule(NodeFormSchema.shape.apiToken, t)]}>
                    <Input.Password placeholder="Bearer token" />
                  </Form.Item>
                </Col>
              </Row>

              {tlsVerifyMode === 'skip' && (
                <Alert type="warning" showIcon style={{ marginBottom: 12 }} title={t('pages.nodes.tlsSkipWarning')} />
              )}

              {tlsVerifyMode === 'pin' && (
                <Form.Item label={t('pages.nodes.pinnedCert')} name="pinnedCertSha256" extra={t('pages.nodes.pinnedCertHint')}>
                  <Input.Search
                    placeholder={t('pages.nodes.pinnedCertPlaceholder')}
                    enterButton={t('pages.nodes.fetchPin')}
                    loading={fetchingPin}
                    onSearch={onFetchPin}
                  />
                </Form.Item>
              )}

              <div className="test-row">
                <Button type="default" loading={testing} onClick={onTest}>
                  {t('pages.nodes.testConnection')}
                </Button>
                {testResult && (
                  <div className="test-result">
                    {testResult.status === 'online' ? (
                      <Alert type="success" showIcon title={t('pages.nodes.connectionOk', { ms: testResult.latencyMs })} description={testResult.xrayVersion ? `Xray ${testResult.xrayVersion}` : undefined} />
                    ) : (
                      <Alert type="error" showIcon title={t('pages.nodes.connectionFailed')} description={testResult.error} />
                    )}
                  </div>
                )}
              </div>
            </div>
          )}
        </Form>
      </Modal>
    </>
  );
}

// ─── Bootstrap progress steps (horizontal) ────────────────────────────

const STEP_ORDER = [
  'detect-arch', 'detect-arch-retry', 'map-arch', 'prepare-dirs',
  'build-bundle', 'upload-bundle', 'install-bundle', 'write-env',
  'install-service', 'verify-bundle', 'daemon-reload', 'enable-service',
  'restart-service', 'service-diag', 'rollback',
] as const;

const STEP_LABELS: Record<string, string> = {
  'detect-arch':       'Arch',
  'detect-arch-retry': 'Retry',
  'map-arch':          'Map',
  'prepare-dirs':      'Dirs',
  'build-bundle':      'Bundle',
  'upload-bundle':     'Upload',
  'install-bundle':    'Extract',
  'write-env':         'Config',
  'install-service':   'Systemd',
  'verify-bundle':     'Verify',
  'daemon-reload':     'Reload',
  'enable-service':    'Enable',
  'restart-service':   'Start',
  'service-diag':      'Diag',
  rollback:            'Rollback',
};

interface BootstrapTimelineProps {
  job: NodeBootstrapJob;
}

function BootstrapTimeline({ job }: BootstrapTimelineProps) {
  const { t } = useTranslation();
  const hasError = job.state === 'failed' || job.steps?.some((s) => !s.ok);
  const isRunning = job.state === 'queued' || job.state === 'running';

  // Build step map keyed by name
  const stepMap = useMemo(() => {
    const m = new Map<string, { ok: boolean; output?: string }>();
    for (const s of job.steps ?? []) m.set(s.name, s);
    return m;
  }, [job.steps]);

  // Find current step index in the full ordered list
  const { currentIdx, items } = useMemo(() => {
    let current = -1;
    const stepItems: { title: string; status: 'finish' | 'process' | 'wait' | 'error'; description?: string }[] = [];

    for (let i = 0; i < STEP_ORDER.length; i++) {
      const name = STEP_ORDER[i];
      const step = stepMap.get(name);

      if (!step) {
        // Not yet reached
        stepItems.push({ title: STEP_LABELS[name] ?? name, status: 'wait' });
        continue;
      }

      if (step.ok) {
        stepItems.push({ title: STEP_LABELS[name] ?? name, status: 'finish' });
        continue;
      }

      // Non-ok step — either current or error
      if (isRunning && current < 0) {
        current = i;
        stepItems.push({ title: STEP_LABELS[name] ?? name, status: 'process' });
      } else {
        stepItems.push({ title: STEP_LABELS[name] ?? name, status: 'error' });
        if (current < 0) current = i;
      }
    }

    return { currentIdx: current, items: stepItems };
  }, [stepMap, isRunning]);

  function generateLogText(): string {
    const lines: string[] = [];
    lines.push(`Bootstrap job: ${job.id}`);
    lines.push(`State: ${job.state}`);
    lines.push('');
    for (const step of job.steps ?? []) {
      lines.push(`  ${step.ok ? '✓' : '✗'} ${STEP_LABELS[step.name] ?? step.name}`);
      if (step.output) {
        for (const l of step.output.trim().split('\n')) lines.push(`      ${l}`);
      }
      lines.push('');
    }
    if (job.error) lines.push(`Error: ${job.error}`);
    return lines.join('\n');
  }

  const downloadLog = useCallback(() => {
    const text = generateLogText();
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `bootstrap-${job.id}.log`;
    a.click();
    URL.revokeObjectURL(url);
  }, [job]);

  // Collect error steps for details display
  const errorSteps = useMemo(() =>
    (job.steps ?? []).filter((s) => !s.ok && s.output),
    [job.steps],
  );

  return (
    <div className="bootstrap-steps-wrapper">
      <Typography.Text strong style={{ fontSize: 13, display: 'block', marginBottom: 8 }}>
        {job.state === 'done'
          ? t('pages.nodes.bootstrapSuccess')
          : job.state === 'failed'
            ? t('pages.nodes.bootstrapFailed')
            : t('pages.nodes.bootstrapRunning')}
      </Typography.Text>

      <div className="bootstrap-steps-scroll">
        <Steps
          direction="horizontal"
          size="small"
          current={currentIdx}
          items={items}
          className="bootstrap-steps-horizontal"
        />
      </div>

      {errorSteps.length > 0 && (
        <div style={{ marginTop: 8 }}>
          {errorSteps.map((step) => (
            <Collapse
              key={step.name}
              ghost
              size="small"
              items={[{
                key: step.name,
                label: `${STEP_LABELS[step.name] ?? step.name} — ${t('pages.nodes.bootstrapErrorDetails') || 'details'}`,
                children: <pre className="bootstrap-step-output">{step.output}</pre>,
              }]}
            />
          ))}
        </div>
      )}

      {hasError && (
        <div style={{ marginTop: 8, textAlign: 'center' }}>
          <Button size="small" icon={<DownloadOutlined />} onClick={downloadLog}>
            {t('pages.nodes.downloadLog') || 'Download Log'}
          </Button>
        </div>
      )}

      {job.error && (
        <Alert
          type="error"
          showIcon
          style={{ marginTop: 8 }}
          message={t('pages.nodes.bootstrapFailed')}
          description={job.error}
        />
      )}
    </div>
  );
}
