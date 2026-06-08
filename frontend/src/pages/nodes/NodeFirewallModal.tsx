import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, InputNumber, Modal, Select, Space, Spin, Switch, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DeleteOutlined } from '@ant-design/icons';

import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { UfwRule, UfwStatus } from '@/schemas/node';

interface Props {
  open: boolean;
  node: NodeRecord | null;
  onClose: () => void;
  fetchFirewallRules: (id: number) => Promise<{ success?: boolean; obj?: UfwStatus | null; msg?: string }>;
  allowFirewallPort: (id: number, port: number, protocol: string) => Promise<{ success?: boolean; msg?: string }>;
  denyFirewallPort: (id: number, port: number, protocol: string) => Promise<{ success?: boolean; msg?: string }>;
  deleteFirewallRule: (id: number, ruleNumber: string) => Promise<{ success?: boolean; msg?: string }>;
  enableFirewall: (id: number) => Promise<{ success?: boolean; msg?: string }>;
  disableFirewall: (id: number) => Promise<{ success?: boolean; msg?: string }>;
}

export default function NodeFirewallModal({
  open,
  node,
  onClose,
  fetchFirewallRules,
  allowFirewallPort,
  denyFirewallPort,
  deleteFirewallRule,
  enableFirewall,
  disableFirewall,
}: Props) {
  const { t } = useTranslation();
  const [messageApi, contextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState<'allow' | 'deny' | 'enable' | 'disable' | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [status, setStatus] = useState<UfwStatus | null>(null);
  const [port, setPort] = useState<number | null>(null);
  const [protocol, setProtocol] = useState<'tcp' | 'udp'>('tcp');

  function refresh() {
    if (!node?.id) return;
    fetchFirewallRules(node.id)
      .then((msg) => {
        if (msg?.success) setStatus(msg.obj ?? null);
        else messageApi.error(msg?.msg || t('somethingWentWrong'));
      });
  }

  useEffect(() => {
    if (!open || !node?.id) return;
    let cancelled = false;
    setLoading(true);
    fetchFirewallRules(node.id)
      .then((msg) => {
        if (cancelled) return;
        if (!msg?.success) {
          messageApi.error(msg?.msg || t('somethingWentWrong'));
          setStatus(null);
          return;
        }
        setStatus(msg.obj ?? null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, node?.id, fetchFirewallRules, messageApi, t]);

  const columns: ColumnsType<UfwRule> = useMemo(() => [
    { title: '#', dataIndex: 'number', width: 64 },
    { title: t('pages.nodes.port') || 'Port', dataIndex: 'port', width: 120 },
    {
      title: t('pages.nodes.status') || 'Action',
      dataIndex: 'action',
      width: 100,
      render: (value: string) => <Tag color={value === 'allow' ? 'green' : 'red'}>{value.toUpperCase()}</Tag>,
    },
    {
      title: t('description') || 'Comment',
      dataIndex: 'comment',
      ellipsis: true,
      render: (value: string) => value || '-',
    },
    {
      title: '',
      width: 48,
      render: (_value, record) => (
        <Button
          type="text"
          size="small"
          danger
          icon={<DeleteOutlined />}
          loading={deleting === String(record.number)}
          onClick={async () => {
            if (!node?.id) return;
            setDeleting(String(record.number));
            try {
              const msg = await deleteFirewallRule(node.id, String(record.number));
              if (msg?.success) {
                messageApi.success(t('pages.nodes.toasts.delete'));
                refresh();
              } else {
                messageApi.error(msg?.msg || t('somethingWentWrong'));
              }
            } finally {
              setDeleting(null);
            }
          }}
        />
      ),
    },
  ], [t, node?.id, deleteFirewallRule, deleting, messageApi]);

  async function apply(action: 'allow' | 'deny') {
    if (!node?.id || !port) return;
    setSaving(action);
    try {
      const msg = action === 'allow'
        ? await allowFirewallPort(node.id, port, protocol)
        : await denyFirewallPort(node.id, port, protocol);
      if (!msg?.success) {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
        return;
      }
      messageApi.success(t('pages.nodes.toasts.update'));
      refresh();
    } finally {
      setSaving(null);
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={840}
      title={t('pages.nodes.firewall') || 'Firewall'}
    >
      {contextHolder}
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Alert
          type={status?.active ? 'success' : status?.installed ? 'warning' : 'error'}
          showIcon
          message={
            !status?.installed
              ? (t('pages.nodes.ufwNotInstalled') || 'UFW not installed on this node')
              : status?.active
                ? (t('pages.nodes.firewallEnabled') || 'UFW active')
                : (t('pages.nodes.firewallDisabled') || 'UFW inactive')
          }
          description={node ? `${node.name || node.address || ''}` : ''}
        />

        {status?.installed && (
          <Space>
            <span style={{ fontSize: 13, opacity: 0.7 }}>{t('pages.nodes.firewallToggle') || 'Enable UFW'}</span>
            <Switch
              checked={!!status?.active}
              loading={saving === 'enable' || saving === 'disable'}
              onChange={async (v) => {
                if (!node?.id) return;
                setSaving(v ? 'enable' : 'disable');
                try {
                  const msg = v ? await enableFirewall(node.id) : await disableFirewall(node.id);
                  if (msg?.success) {
                    messageApi.success(t('pages.nodes.toasts.update'));
                    refresh();
                  } else {
                    messageApi.error(msg?.msg || t('somethingWentWrong'));
                  }
                } finally {
                  setSaving(null);
                }
              }}
            />
          </Space>
        )}

        <Space wrap>
          <InputNumber min={1} max={65535} value={port ?? undefined} onChange={(v) => setPort(typeof v === 'number' ? v : null)} placeholder="Port" />
          <Select
            value={protocol}
            onChange={(v) => setProtocol(v)}
            options={[{ value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP' }]}
            style={{ width: 120 }}
          />
          <Button type="primary" loading={saving === 'allow'} onClick={() => apply('allow')} disabled={!port}>{t('allow') || 'Allow'}</Button>
          <Button danger loading={saving === 'deny'} onClick={() => apply('deny')} disabled={!port}>{t('deny') || 'Deny'}</Button>
        </Space>

        <Spin spinning={loading}>
          <Table
            size="small"
            rowKey={(row) => String(row.number)}
            dataSource={status?.rules ?? []}
            columns={columns}
            pagination={false}
            locale={{ emptyText: t('noData') }}
          />
        </Spin>
      </Space>
    </Modal>
  );
}
