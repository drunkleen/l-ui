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
  Spin,
  Switch,
  Typography,
  message,
} from 'antd';
import {
  CheckCircleFilled,
  CloseCircleFilled,
  DownloadOutlined,
  LoadingOutlined,
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
    id: 0,
    name: '',
    scheme: 'https',
    address: '',
    port: 2053,
    basePath: '/',
    apiToken: '',
    enable: true,
    allowPrivateAddress: false,
    tlsVerifyMode: 'verify',
    pinnedCertSha256: '',
    sshUser: '',
    sshPassword: '',
    useTLS: false,
    domain: '',
    acmeEmail: '',
    sshPort: 22,
    bootstrapBase: '/',
    group: '',
  };
}

export default function NodeFormModal({
  open,
  mode,
  node,
  testConnection,
  fetchFingerprint,
  save,
  bootstrap,
  bootstrapStatus,
  onOpenChange,
}: NodeFormModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<FormValues>();
  const [messageApi, messageContextHolder] = message.useMessage();

  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState(false);
  const [fetchingPin, setFetchingPin] = useState(false);
  const [testResult, setTestResult] = useState<ProbeResult | null>(null);
  const [bootstrapJob, setBootstrapJob] = useState<NodeBootstrapJob | null>(null);

  // Auto-close modal when bootstrap succeeds
  useEffect(() => {
    if (bootstrapJob?.state === 'done') close();
  }, [bootstrapJob]); // eslint-disable-line react-hooks/exhaustive-deps
  const scheme = Form.useWatch('scheme', form) ?? 'https';
  const tlsVerifyMode = Form.useWatch('tlsVerifyMode', form) ?? 'verify';
  const useTLS = Form.useWatch('useTLS', form) ?? false;

  useEffect(() => {
    if (!open) return;
    const base = defaultValues();
    const next: NodeFormValues = mode === 'edit' && node
      ? {
        ...base,
        ...(node as unknown as Partial<NodeFormValues>),
        id: node.id,
        scheme: (node.scheme as 'http' | 'https') || base.scheme,
      }
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
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [open, bootstrapJob, bootstrapStatus]);

  const title = useMemo(
    () => (mode === 'edit' ? t('pages.nodes.editNode') : t('pages.nodes.addNode')),
    [mode, t],
  );

  function buildPayload(values: NodeFormValues): Partial<NodeRecord> {
    return {
      id: values.id || 0,
      name: values.name.trim(),
      scheme: values.scheme,
      address: values.address.trim(),
      port: values.port,
      basePath: values.basePath.trim() || '/',
      apiToken: values.apiToken.trim(),
      enable: values.enable,
      allowPrivateAddress: values.allowPrivateAddress,
      tlsVerifyMode: values.tlsVerifyMode,
      pinnedCertSha256: values.tlsVerifyMode === 'pin' ? values.pinnedCertSha256.trim() : '',
    };
  }

  function buildBootstrapPayload(values: FormValues): NodeBootstrapFormValues {
    const result = NodeBootstrapFormSchema.parse(values);
    return {
      name: result.name,
      address: result.address,
      sshUser: result.sshUser,
      sshPassword: result.sshPassword,
      useTLS: result.useTLS,
      domain: result.domain,
      acmeEmail: result.acmeEmail,
      sshPort: result.sshPort,
      agentPort: result.agentPort,
      bootstrapBase: result.bootstrapBase || '/',
    };
  }

  async function onTest() {
    try {
      await form.validateFields(['address', 'port']);
    } catch {
      return;
    }
    setTesting(true);
    setTestResult(null);
    try {
      const payload = buildPayload(form.getFieldsValue(true));
      const msg = await testConnection(payload);
      if (msg?.success && msg.obj) {
        setTestResult(msg.obj);
      } else {
        setTestResult({ status: 'offline', error: msg?.msg || 'unknown error' });
      }
    } finally {
      setTesting(false);
    }
  }

  async function onFetchPin() {
    try {
      await form.validateFields(['address', 'port']);
    } catch {
      return;
    }
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
    } finally {
      setFetchingPin(false);
    }
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
        if (msg?.success) {
          onOpenChange(false);
        }
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
    } finally {
      setSubmitting(false);
    }
  }

  function close() {
    if (!submitting) onOpenChange(false);
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={title}
        confirmLoading={submitting || (bootstrapJob?.state === 'queued' || bootstrapJob?.state === 'running')}
        okText={mode === 'add' && bootstrapJob?.state === 'done' ? t('close') : (mode === 'add' ? t('pages.nodes.bootstrapNode') : t('save'))}
        cancelText={t('cancel')}
        mask={{ closable: false }}
        width="760px"
        onOk={() => {
          if (mode === 'add' && bootstrapJob?.state === 'done') {
            close();
            return;
          }
          form.submit();
        }}
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
            <>
              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.sshHost')}
                    name="address"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.address, t)]}
                  >
                    <Input placeholder={t('pages.nodes.sshHostPlaceholder')} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.sshUser')}
                    name="sshUser"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.sshUser, t)]}
                  >
                    <Input />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.sshPassword')}
                    name="sshPassword"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.sshPassword, t)]}
                  >
                    <Input.Password />
                  </Form.Item>
                </Col>
                <Col xs={12} md={6}>
                  <Form.Item
                    label={t('pages.nodes.sshPort')}
                    name="sshPort"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.sshPort, t)]}
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={6}>
                  <Form.Item
                    label={t('pages.nodes.agentPort')}
                    name="agentPort"
                    rules={[antdRule(NodeBootstrapFormSchema.shape.agentPort, t)]}
                  >
                    <InputNumber min={1} max={65535} placeholder="auto" style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item
                label={t('pages.nodes.useTls')}
                name="useTLS"
                valuePropName="checked"
                extra={t('pages.nodes.useTlsHint')}
              >
                <Switch />
              </Form.Item>

              {useTLS && (
                <>
                  <Form.Item
                    label={t('pages.nodes.domain')}
                    name="domain"
                    rules={[
                      { required: useTLS, message: t('pages.nodes.domainRequired') },
                      antdRule(NodeBootstrapFormSchema.shape.domain, t),
                    ]}
                    extra={t('pages.nodes.domainHint')}
                  >
                    <Input placeholder={t('pages.nodes.domainPlaceholder')} />
                  </Form.Item>

                  <Form.Item
                    label={t('pages.nodes.acmeEmail')}
                    name="acmeEmail"
                    extra={t('pages.nodes.acmeEmailHint')}
                  >
                    <Input placeholder={t('pages.nodes.acmeEmailPlaceholder')} />
                  </Form.Item>

                  <Alert
                    type="info"
                    showIcon
                    style={{ marginBottom: 16 }}
                    message={t('pages.nodes.useTlsNotice')}
                  />
                </>
              )}

              {bootstrapJob && <BootstrapTimeline job={bootstrapJob} />}
            </>
          ) : (
            <>
              <Row gutter={16}>
                <Col xs={24} md={6}>
                  <Form.Item label={t('pages.nodes.scheme')} name="scheme">
                    <Select
                      options={[
                        { value: 'https', label: 'https' },
                        { value: 'http', label: 'http' },
                      ]}
                      onChange={(value) => {
                        if (value === 'http') form.setFieldValue('tlsVerifyMode', 'skip');
                      }}
                    />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.address')}
                    name="address"
                    rules={[antdRule(NodeFormSchema.shape.address, t)]}
                  >
                    <Input placeholder={t('pages.nodes.addressPlaceholder')} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={6}>
                  <Form.Item
                    label={t('pages.nodes.port')}
                    name="port"
                    rules={[antdRule(NodeFormSchema.shape.port, t)]}
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item label={t('pages.nodes.basePath')} name="basePath">
                    <Input placeholder="/" />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    label={t('pages.nodes.enable')}
                    name="enable"
                    valuePropName="checked"
                  >
                    <Switch />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item
                label={t('pages.nodes.allowPrivateAddress')}
                name="allowPrivateAddress"
                valuePropName="checked"
                extra={t('pages.nodes.allowPrivateAddressHint')}
              >
                <Switch />
              </Form.Item>

              <Form.Item
                label={t('pages.nodes.tlsVerifyMode')}
                name="tlsVerifyMode"
                extra={t('pages.nodes.tlsVerifyModeHint')}
              >
                <Select
                  disabled={scheme === 'http'}
                  options={[
                    { value: 'verify', label: t('pages.nodes.tlsVerify') },
                    { value: 'pin', label: t('pages.nodes.tlsPin') },
                    { value: 'skip', label: t('pages.nodes.tlsSkip') },
                  ]}
                />
              </Form.Item>

              {tlsVerifyMode === 'skip' && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 16 }}
                  title={t('pages.nodes.tlsSkipWarning')}
                />
              )}

              {tlsVerifyMode === 'pin' && (
                <Form.Item
                  label={t('pages.nodes.pinnedCert')}
                  name="pinnedCertSha256"
                  extra={t('pages.nodes.pinnedCertHint')}
                >
                  <Input.Search
                    placeholder={t('pages.nodes.pinnedCertPlaceholder')}
                    enterButton={t('pages.nodes.fetchPin')}
                    loading={fetchingPin}
                    onSearch={onFetchPin}
                  />
                </Form.Item>
              )}

              <Form.Item
                label={t('pages.nodes.group') || 'Group'}
                name="group"
              >
                <Input placeholder={t('pages.nodes.groupPlaceholder') || 'e.g. eu, asia, production'} />
              </Form.Item>

              <Form.Item
                label={t('pages.nodes.apiToken')}
                name="apiToken"
                rules={[antdRule(NodeFormSchema.shape.apiToken, t)]}
                extra={t('pages.nodes.apiTokenHint')}
              >
                <Input.Password placeholder={t('pages.nodes.apiTokenPlaceholder')} />
              </Form.Item>

              <div className="test-row">
                <Button type="default" loading={testing} onClick={onTest}>
                  {t('pages.nodes.testConnection')}
                </Button>
                {testResult && (
                  <div className="test-result">
                    {testResult.status === 'online' ? (
                      <Alert
                        type="success"
                        showIcon
                        title={t('pages.nodes.connectionOk', { ms: testResult.latencyMs })}
                        description={testResult.xrayVersion ? `Xray ${testResult.xrayVersion}` : undefined}
                      />
                    ) : (
                      <Alert
                        type="error"
                        showIcon
                        title={t('pages.nodes.connectionFailed')}
                        description={testResult.error}
                      />
                    )}
                  </div>
                )}
              </div>
            </>
          )}
        </Form>
      </Modal>
    </>
  );
}

// ─── Bootstrap progress timeline ───────────────────────────────────────

interface BootstrapTimelineProps {
  job: NodeBootstrapJob;
}

const STEP_LABELS: Record<string, string> = {
  'detect-arch':      'Detect architecture',
  'detect-arch-retry':'Detect architecture (retry)',
  'map-arch':         'Map architecture',
  'prepare-dirs':     'Prepare directories',
  'build-bundle':     'Build release bundle',
  'upload-bundle':    'Upload bundle to node',
  'install-bundle':   'Extract bundle',
  'write-env':        'Write environment config',
  'install-service':  'Install systemd service',
  'verify-bundle':    'Verify bundle integrity',
  'daemon-reload':    'Reload systemd',
  'enable-service':   'Enable service',
  'restart-service':  'Start service',
  'service-diag':     'Service diagnostics',
  rollback:           'Rollback',
};

function stepLabel(name: string): string {
  return STEP_LABELS[name] ?? name;
}

function BootstrapTimeline({ job }: BootstrapTimelineProps) {
  const { t } = useTranslation();
  const lastStepIdx = (job.steps?.length ?? 1) - 1;
  const hasError = job.state === 'failed' || job.steps?.some((s) => !s.ok);

  function generateLogText(): string {
    const lines: string[] = [];
    lines.push(`Bootstrap job: ${job.id}`);
    lines.push(`State: ${job.state}`);
    lines.push('');
    for (const step of job.steps ?? []) {
      lines.push(`  ${step.ok ? '✓' : '✗'} ${stepLabel(step.name)}`);
      if (step.output) {
        for (const l of step.output.trim().split('\n')) {
          lines.push(`      ${l}`);
        }
      }
      lines.push('');
    }
    if (job.error) {
      lines.push(`Error: ${job.error}`);
    }
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

  return (
    <div className="bootstrap-timeline">
      <Typography.Title level={5} style={{ marginTop: 0, marginBottom: 16 }}>
        {job.state === 'done'
          ? t('pages.nodes.bootstrapSuccess')
          : job.state === 'failed'
            ? t('pages.nodes.bootstrapFailed')
            : t('pages.nodes.bootstrapRunning')}
      </Typography.Title>

      <div className="bootstrap-steps">
        {(job.steps ?? []).map((step, idx) => {
          const isRunning = !step.ok && idx === lastStepIdx && (job.state === 'running' || job.state === 'queued');
          const isPending = !step.ok && !isRunning;
          const icon = step.ok
            ? <CheckCircleFilled style={{ color: '#52c41a', fontSize: 18 }} />
            : isRunning
              ? <Spin indicator={<LoadingOutlined style={{ fontSize: 18, color: '#1677ff' }} spin />} />
              : <CloseCircleFilled style={{ color: '#ff4d4f', fontSize: 18 }} />;

          return (
            <div key={step.name} className={`bootstrap-step${isRunning ? ' is-current' : ''}${isPending ? ' is-pending' : ''}`}>
              <div className="bootstrap-step-icon">{icon}</div>
              <div className="bootstrap-step-body">
                <div className="bootstrap-step-header">
                  <span className="bootstrap-step-name">{stepLabel(step.name)}</span>
                  {isRunning && <span className="bootstrap-step-badge">{t('pages.nodes.inProgress')}</span>}
                </div>
                {step.output && !step.ok && (
                  <Collapse
                    ghost
                    size="small"
                    items={[{
                      key: 'output',
                      label: t('pages.nodes.bootstrapErrorDetails') || 'Error details',
                      children: (
                        <pre className="bootstrap-step-output">{step.output}</pre>
                      ),
                    }]}
                  />
                )}
              </div>
              {/* connector line */}
              {idx < lastStepIdx && <div className="bootstrap-connector" />}
            </div>
          );
        })}
      </div>

      {hasError && (
        <div style={{ marginTop: 16, textAlign: 'center' }}>
          <Space>
            <Button icon={<DownloadOutlined />} onClick={downloadLog}>
              {t('pages.nodes.downloadLog') || 'Download Log'}
            </Button>
          </Space>
        </div>
      )}

      {job.error && (
        <Alert
          type="error"
          showIcon
          style={{ marginTop: 12 }}
          message={t('pages.nodes.bootstrapFailed')}
          description={job.error}
        />
      )}
    </div>
  );
}
