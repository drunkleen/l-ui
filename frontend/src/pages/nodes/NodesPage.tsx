import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { Button, Card, Col, ConfigProvider, Layout, Modal, Result, Row, Spin, Statistic, message } from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudServerOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useNodesQuery } from '@/api/queries/useNodesQuery';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { useNodeMutations } from '@/api/queries/useNodeMutations';
import AppSidebar from '@/layouts/AppSidebar';
import NodeList from './NodeList';
import NodeFormModal from './NodeFormModal';
import NodeFirewallModal from './NodeFirewallModal';
import NodeRegistrationModal from './NodeRegistrationModal';
import NodeLogsModal from './NodeLogsModal';
import NodePushConfigModal from './NodePushConfigModal';
import NodePortGroupModal from './NodePortGroupModal';
import { setMessageInstance } from '@/utils/messageBus';
import { HttpUtil } from '@/utils';
import type { PanelUpdateInfo } from '../index/PanelUpdateModal';

export default function NodesPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => { setMessageInstance(messageApi); }, [messageApi]);

  const { nodes, loading, fetched, fetchError, refetch, totals } = useNodesQuery();
  const { create, update, remove, setEnable, testConnection, fetchFingerprint, bootstrap, bootstrapStatus, probe, updatePanels, reinstall, rotateCredentials, reconcile, fetchFirewallRules, allowFirewallPort, denyFirewallPort, deleteFirewallRule, enableFirewall, disableFirewall, restartAgent, restartXray, fetchLogs, pushConfig, fetchPortGroups, createPortGroup, updatePortGroup, deletePortGroup, pushPortGroup, fetchNodeGroups } = useNodeMutations();

  const { data: latestVersion = '' } = useQuery({
    queryKey: ['server', 'panelUpdateInfo'],
    queryFn: async () => {
      const msg = await HttpUtil.get<PanelUpdateInfo>('/panel/api/server/getPanelUpdateInfo');
      return msg?.obj?.latestVersion || '';
    },
    staleTime: 5 * 60 * 1000,
  });

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<'add' | 'edit'>('add');
  const [formNode, setFormNode] = useState<NodeRecord | null>(null);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [firewallOpen, setFirewallOpen] = useState(false);
  const [firewallNode, setFirewallNode] = useState<NodeRecord | null>(null);
  const [regTokenOpen, setRegTokenOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [logsNode, setLogsNode] = useState<NodeRecord | null>(null);
  const [pushConfigOpen, setPushConfigOpen] = useState(false);
  const [pushConfigNode, setPushConfigNode] = useState<NodeRecord | null>(null);
  const [portGroupOpen, setPortGroupOpen] = useState(false);

  const onAdd = useCallback(() => {
    setFormMode('add');
    setFormNode(null);
    setFormOpen(true);
  }, []);

  const onEdit = useCallback((node: NodeRecord) => {
    setFormMode('edit');
    setFormNode({ ...node });
    setFormOpen(true);
  }, []);

  const onSave = useCallback(async (payload: Partial<NodeRecord>) => {
    if (formMode === 'edit' && formNode?.id) {
      return update(formNode.id, payload);
    }
    return create(payload);
  }, [formMode, formNode, update, create]);

  const onDelete = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.deleteConfirmTitle', { name: node.name }),
      content: t('pages.nodes.deleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.toasts.deleted'));
      },
    });
  }, [modal, t, remove, messageApi]);

  const onDeleteCleanup = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.deleteConfirmTitle', { name: node.name }),
      content: t('pages.nodes.deleteConfirmContent') + ' ' + (t('pages.nodes.cleanupRemoteHint') || 'Remove remote files too.'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await remove(node.id, true);
        if (msg?.success) messageApi.success(t('pages.nodes.toasts.deleted'));
      },
    });
  }, [modal, t, remove, messageApi]);

  const onProbe = useCallback(async (node: NodeRecord) => {
    const msg = await probe(node.id);
    if (msg?.success && msg.obj) {
      if (msg.obj.status === 'online') {
        messageApi.success(t('pages.nodes.connectionOk', { ms: msg.obj.latencyMs }));
      } else {
        messageApi.error(msg.obj.error || t('pages.nodes.toasts.probeFailed'));
      }
    }
  }, [probe, t, messageApi]);

  const onToggleEnable = useCallback(async (node: NodeRecord, next: boolean) => {
    await setEnable(node.id, next);
  }, [setEnable]);

  const runUpdate = useCallback(async (ids: number[]) => {
    const msg = await updatePanels(ids);
    if (!msg?.success) {
      messageApi.error(msg?.msg || t('somethingWentWrong'));
      return;
    }
    const results = msg.obj ?? [];
    const ok = results.filter((r) => r.ok).length;
    const failed = results.length - ok;
    if (failed === 0) {
      messageApi.success(t('pages.nodes.toasts.updateStarted'));
    } else {
      const firstError = results.find((r) => !r.ok)?.error ?? '';
      const base = t('pages.nodes.toasts.updateResult', { ok, failed });
      messageApi.warning(firstError ? `${base} — ${firstError}` : base);
    }
    setSelectedIds([]);
  }, [updatePanels, messageApi, t]);

  const onUpdateNode = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.updateConfirmTitle', { count: 1 }),
      content: t('pages.nodes.updateConfirmContent'),
      okText: t('update'),
      cancelText: t('cancel'),
      onOk: () => runUpdate([node.id]),
    });
  }, [modal, t, runUpdate]);

  const onReinstallNode = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.reinstall') || 'Reinstall node bundle',
      content: t('pages.nodes.reinstallConfirm') || 'Reinstall the node bundle on this VPS using the current hub artifacts.',
      okText: t('reinstall') || 'Reinstall',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await reinstall(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.reinstallStarted') || 'Reinstall started');
      },
    });
  }, [modal, t, reinstall, messageApi]);

  const onRotateCredentials = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.rotateCredentials') || 'Rotate credentials',
      content: t('pages.nodes.rotateCredentialsConfirm') || 'Generate a new API token and reconfigure the node.',
      okText: t('confirm') || 'Confirm',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await rotateCredentials(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.rotateCredentialsStarted') || 'Credentials rotated');
      },
    });
  }, [modal, t, rotateCredentials, messageApi]);

  const onReconcileNode = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.reconcile') || 'Reconcile node',
      content: t('pages.nodes.reconcileConfirm') || 'Recheck the node and attempt a recovery restart if needed.',
      okText: t('confirm') || 'Confirm',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await reconcile(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.reconcileStarted') || 'Reconcile started');
      },
    });
  }, [modal, t, reconcile, messageApi]);

  const onManageFirewall = useCallback((node: NodeRecord) => {
    setFirewallNode(node);
    setFirewallOpen(true);
  }, []);

  const onPushConfig = useCallback((node: NodeRecord) => {
    setPushConfigNode(node);
    setPushConfigOpen(true);
  }, []);

  const onPortGroups = useCallback(() => {
    setPortGroupOpen(true);
  }, []);

  const onViewLogs = useCallback((node: NodeRecord) => {
    setLogsNode(node);
    setLogsOpen(true);
  }, []);

  const onRestartAgent = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.restartAgent') || 'Restart agent',
      content: t('pages.nodes.restartAgentConfirm') || `Restart the agent process on ${node.name}?`,
      okText: t('confirm') || 'Confirm',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await restartAgent(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.restartAgentStarted') || 'Agent restart initiated');
      },
    });
  }, [modal, t, restartAgent, messageApi]);

  const onRestartXray = useCallback((node: NodeRecord) => {
    modal.confirm({
      title: t('pages.nodes.restartXray') || 'Restart Xray',
      content: t('pages.nodes.restartXrayConfirm') || `Restart Xray on ${node.name}?`,
      okText: t('confirm') || 'Confirm',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await restartXray(node.id);
        if (msg?.success) messageApi.success(t('pages.nodes.restartXrayStarted') || 'Xray restart initiated');
      },
    });
  }, [modal, t, restartXray, messageApi]);

  const onUpdateSelected = useCallback(() => {
    const eligible = nodes
      .filter((n) => selectedIds.includes(n.id) && n.enable && n.status === 'online')
      .map((n) => n.id);
    if (eligible.length === 0) {
      messageApi.warning(t('pages.nodes.toasts.updateNoneEligible'));
      return;
    }
    modal.confirm({
      title: t('pages.nodes.updateConfirmTitle', { count: eligible.length }),
      content: t('pages.nodes.updateConfirmContent'),
      okText: t('update'),
      cancelText: t('cancel'),
      onOk: () => runUpdate(eligible),
    });
  }, [modal, t, nodes, selectedIds, runUpdate, messageApi]);

  const pageClass = useMemo(() => {
    const classes = ['nodes-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      {modalContextHolder}
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" loading={loading} onClick={() => refetch()}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.totalNodes')}
                            value={String(totals.total)}
                            prefix={<CloudServerOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.onlineNodes')}
                            value={String(totals.online)}
                            prefix={<CheckCircleOutlined style={{ color: 'var(--ant-color-success)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.offlineNodes')}
                            value={String(totals.offline)}
                            prefix={<CloseCircleOutlined style={{ color: 'var(--ant-color-error)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.nodes.avgLatency')}
                            value={totals.avgLatency > 0 ? `${totals.avgLatency} ms` : '-'}
                            prefix={<ThunderboltOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <NodeList
                      nodes={nodes}
                      loading={loading}
                      isMobile={isMobile}
                      latestVersion={latestVersion}
                      selectedIds={selectedIds}
                      onSelectionChange={setSelectedIds}
                      onAdd={onAdd}
                      onEdit={onEdit}
                      onDelete={onDelete}
                      onDeleteCleanup={onDeleteCleanup}
                      onProbe={onProbe}
                      onToggleEnable={onToggleEnable}
                      onUpdateNode={onUpdateNode}
                      onReinstallNode={onReinstallNode}
                      onRotateCredentials={onRotateCredentials}
                      onReconcileNode={onReconcileNode}
                      onManageFirewall={onManageFirewall}
                      onUpdateSelected={onUpdateSelected}
                      onViewLogs={onViewLogs}
                      onRestartAgent={onRestartAgent}
                      onRestartXray={onRestartXray}
                      onPushConfig={onPushConfig}
                      onPortGroups={onPortGroups}
                      onRegistrationTokens={() => setRegTokenOpen(true)}
                    />
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>

        <NodeFormModal
          open={formOpen}
          mode={formMode}
          node={formNode}
          testConnection={testConnection}
          fetchFingerprint={fetchFingerprint}
          save={onSave}
          bootstrap={bootstrap}
          bootstrapStatus={bootstrapStatus}
          onOpenChange={setFormOpen}
          onBootstrapDone={refetch}
        />
        <NodeFirewallModal
          open={firewallOpen}
          node={firewallNode}
          onClose={() => setFirewallOpen(false)}
          fetchFirewallRules={fetchFirewallRules}
          allowFirewallPort={allowFirewallPort}
          denyFirewallPort={denyFirewallPort}
          deleteFirewallRule={deleteFirewallRule}
          enableFirewall={enableFirewall}
          disableFirewall={disableFirewall}
        />
        <NodeRegistrationModal
          open={regTokenOpen}
          onClose={() => setRegTokenOpen(false)}
        />
        <NodeLogsModal
          open={logsOpen}
          node={logsNode}
          onClose={() => setLogsOpen(false)}
          fetchLogs={fetchLogs}
        />
        <NodePushConfigModal
          open={pushConfigOpen}
          node={pushConfigNode}
          onClose={() => setPushConfigOpen(false)}
          pushConfig={pushConfig}
        />
        <NodePortGroupModal
          open={portGroupOpen}
          onClose={() => setPortGroupOpen(false)}
          fetchPortGroups={fetchPortGroups}
          createPortGroup={createPortGroup}
          updatePortGroup={updatePortGroup}
          deletePortGroup={deletePortGroup}
          pushPortGroup={pushPortGroup}
          fetchNodeGroups={fetchNodeGroups}
        />
      </Layout>
    </ConfigProvider>
  );
}
