import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, InputNumber, Modal, Select, Space, Spin, Table, Tag, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DeleteOutlined, EditOutlined, PlusOutlined, SendOutlined } from '@ant-design/icons';

import type { PortGroup, PortGroupEntry } from '@/schemas/node';

interface Props {
  open: boolean;
  onClose: () => void;
  fetchPortGroups: () => Promise<{ success?: boolean; obj?: PortGroup[] | null; msg?: string }>;
  createPortGroup: (name: string, ports: { port: number; protocol: string; comment?: string }[]) => Promise<{ success?: boolean; msg?: string }>;
  updatePortGroup: (id: number, name: string, ports: { port: number; protocol: string; comment?: string }[]) => Promise<{ success?: boolean; msg?: string }>;
  deletePortGroup: (id: number) => Promise<{ success?: boolean; msg?: string }>;
  pushPortGroup: (portGroupId: number, nodeGroup: string) => Promise<{ success?: boolean; msg?: string }>;
  fetchNodeGroups: () => Promise<{ success?: boolean; obj?: string[] | null; msg?: string }>;
}

interface PortGroupFormState {
  name: string;
  entries: PortGroupEntry[];
}

const defaultProtocol = 'tcp';

function parsePorts(portsStr: string): PortGroupEntry[] {
  try {
    return JSON.parse(portsStr) as PortGroupEntry[];
  } catch {
    return [];
  }
}

export default function NodePortGroupModal({
  open,
  onClose,
  fetchPortGroups,
  createPortGroup,
  updatePortGroup,
  deletePortGroup,
  pushPortGroup,
  fetchNodeGroups,
}: Props) {
  const { t } = useTranslation();
  const [messageApi, contextHolder] = message.useMessage();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [groups, setGroups] = useState<PortGroup[]>([]);
  const [editing, setEditing] = useState<PortGroupFormState | null>(null);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [nodeGroups, setNodeGroups] = useState<string[]>([]);
  const [pushGroupId, setPushGroupId] = useState<number | null>(null);
  const [pushGroupName, setPushGroupName] = useState('');
  const [pushing, setPushing] = useState(false);

  const refresh = useCallback(() => {
    setLoading(true);
    fetchPortGroups()
      .then((msg) => {
        if (msg?.success) setGroups(msg.obj ?? []);
        else messageApi.error(msg?.msg || t('somethingWentWrong'));
      })
      .finally(() => setLoading(false));
    fetchNodeGroups()
      .then((msg) => {
        if (msg?.success) setNodeGroups(msg.obj ?? []);
      });
  }, [fetchPortGroups, fetchNodeGroups, messageApi, t]);

  useEffect(() => {
    if (open) refresh();
  }, [open, refresh]);

  function resetForm() {
    setEditing(null);
    setEditingId(null);
  }

  function startCreate() {
    setEditing({ name: '', entries: [{ port: 0, protocol: defaultProtocol }] });
    setEditingId(null);
  }

  function startEdit(g: PortGroup) {
    setEditing({ name: g.name, entries: parsePorts(g.ports) });
    setEditingId(g.id);
  }

  async function saveForm() {
    if (!editing || !editing.name.trim()) {
      messageApi.error(t('pages.nodes.toasts.fillRequired'));
      return;
    }
    const validEntries = editing.entries.filter((e) => e.port > 0);
    if (validEntries.length === 0) {
      messageApi.error(t('pages.nodes.portGroupAddEntries') || 'Add at least one port');
      return;
    }
    setSaving(true);
    try {
      const msg = editingId
        ? await updatePortGroup(editingId, editing.name.trim(), validEntries)
        : await createPortGroup(editing.name.trim(), validEntries);
      if (msg?.success) {
        messageApi.success(t('save'));
        resetForm();
        refresh();
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } finally {
      setSaving(false);
    }
  }

  function addEntry() {
    if (!editing) return;
    setEditing({ ...editing, entries: [...editing.entries, { port: 0, protocol: defaultProtocol }] });
  }

  function updateEntry(idx: number, field: keyof PortGroupEntry, value: unknown) {
    if (!editing) return;
    const entries = [...editing.entries];
    entries[idx] = { ...entries[idx], [field]: value };
    setEditing({ ...editing, entries });
  }

  function removeEntry(idx: number) {
    if (!editing) return;
    setEditing({ ...editing, entries: editing.entries.filter((_, i) => i !== idx) });
  }

  async function startPush(g: PortGroup) {
    setPushGroupId(g.id);
    setPushGroupName('');
  }

  async function confirmPush() {
    if (pushGroupId == null || !pushGroupName) return;
    setPushing(true);
    try {
      const msg = await pushPortGroup(pushGroupId, pushGroupName);
      if (msg?.success) {
        messageApi.success(t('pages.nodes.toasts.update'));
        setPushGroupId(null);
        setPushGroupName('');
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
      }
    } finally {
      setPushing(false);
    }
  }

  const confirmDelete = useCallback((g: PortGroup) => {
    Modal.confirm({
      title: t('delete'),
      content: t('pages.nodes.deleteConfirmContent'),
      okText: t('delete'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: async () => {
        const msg = await deletePortGroup(g.id);
        if (msg?.success) {
          messageApi.success(t('pages.nodes.toasts.deleted'));
          refresh();
        } else {
          messageApi.error(msg?.msg || t('somethingWentWrong'));
        }
      },
    });
  }, [deletePortGroup, messageApi, refresh, t]);

  const columns: ColumnsType<PortGroup> = useMemo(() => [
    { title: t('name'), dataIndex: 'name', ellipsis: true },
    {
      title: t('pages.nodes.port') || 'Ports',
      key: 'portCount',
      width: 100,
      render: (_value, record) => {
        const entries = parsePorts(record.ports);
        return <Tag>{entries.length} {t('pages.nodes.port') || 'ports'}</Tag>;
      },
    },
    {
      title: t('pages.nodes.actions'),
      width: 180,
      render: (_value, record) => (
        <Space>
          <Button type="text" size="small" icon={<EditOutlined />} onClick={() => startEdit(record)} />
          <Button type="text" size="small" icon={<SendOutlined />} onClick={() => startPush(record)} />
          <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={() => confirmDelete(record)} />
        </Space>
      ),
    },
  ], [t, confirmDelete]);

  return (
    <Modal
      open={open}
      onCancel={onClose}
      footer={null}
      width={800}
      title={t('pages.nodes.portGroups') || 'Port Groups'}
    >
      {contextHolder}
      <Spin spinning={loading}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Button type="primary" icon={<PlusOutlined />} onClick={startCreate}>
            {t('add')}
          </Button>

          {editing && (
            <Space direction="vertical" style={{ width: '100%' }}>
              <Input
                placeholder={t('pages.nodes.portGroupName') || 'Group name'}
                value={editing.name}
                onChange={(e) => setEditing({ ...editing, name: e.target.value })}
              />
              {editing.entries.map((entry, idx) => (
                <Space key={idx} wrap>
                  <InputNumber
                    min={1}
                    max={65535}
                    value={entry.port || undefined}
                    onChange={(v) => updateEntry(idx, 'port', typeof v === 'number' ? v : 0)}
                    placeholder={t('pages.nodes.port') || 'Port'}
                    style={{ width: 100 }}
                  />
                  <Select
                    value={entry.protocol || defaultProtocol}
                    onChange={(v) => updateEntry(idx, 'protocol', v)}
                    options={[{ value: 'tcp', label: 'TCP' }, { value: 'udp', label: 'UDP' }]}
                    style={{ width: 100 }}
                  />
                  <Input
                    placeholder={t('description') || 'Comment'}
                    value={entry.comment || ''}
                    onChange={(e) => updateEntry(idx, 'comment', e.target.value)}
                    style={{ width: 160 }}
                  />
                  <Button danger icon={<DeleteOutlined />} onClick={() => removeEntry(idx)} />
                </Space>
              ))}
              <Space>
                <Button icon={<PlusOutlined />} onClick={addEntry}>{t('pages.nodes.addPort') || 'Add port'}</Button>
                <Button type="primary" loading={saving} onClick={saveForm}>{t('save')}</Button>
                <Button onClick={resetForm}>{t('cancel')}</Button>
              </Space>
            </Space>
          )}

          <Table
            size="small"
            rowKey="id"
            dataSource={groups}
            columns={columns}
            pagination={false}
            locale={{ emptyText: t('noData') }}
          />

          <Modal
            open={pushGroupId != null}
            onCancel={() => setPushGroupId(null)}
            title={t('pages.nodes.pushPortGroup') || 'Push Port Group'}
            footer={null}
            width={400}
          >
            <Space direction="vertical" style={{ width: '100%' }}>
              <Select
                showSearch
                style={{ width: '100%' }}
                placeholder={t('pages.nodes.selectNodeGroup') || 'Select node group'}
                value={pushGroupName || undefined}
                onChange={setPushGroupName}
                options={nodeGroups.map((g) => ({ value: g, label: g }))}
              />
              <Button type="primary" loading={pushing} disabled={!pushGroupName} onClick={confirmPush} block>
                {t('pages.nodes.pushToGroup') || 'Push to Group'}
              </Button>
            </Space>
          </Modal>
        </Space>
      </Spin>
    </Modal>
  );
}
