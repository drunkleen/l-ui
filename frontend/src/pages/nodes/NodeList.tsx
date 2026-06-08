import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Badge,
  Button,
  Card,
  Dropdown,
  Space,
  Switch,
  Tag,
  Tooltip,
} from 'antd';
import type { BadgeProps } from 'antd';
import {
  ClusterOutlined,
  CloudDownloadOutlined,
  DeleteOutlined,
  EditOutlined,
  ExclamationCircleOutlined,
  FileTextOutlined,
  KeyOutlined,
  MoreOutlined,
  PlusOutlined,
  ReloadOutlined,
  RightOutlined,
  SafetyOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';

import NodeHistoryPanel from './NodeHistoryPanel';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import { isPanelUpdateAvailable } from '@/lib/panel-version';
import './NodeList.css';

interface NodeListProps {
  nodes: NodeRecord[];
  loading?: boolean;
  isMobile?: boolean;
  latestVersion?: string;
  selectedIds: number[];
  onSelectionChange: (ids: number[]) => void;
  onAdd: () => void;
  onEdit: (node: NodeRecord) => void;
  onDelete: (node: NodeRecord) => void;
  onDeleteCleanup: (node: NodeRecord) => void;
  onProbe: (node: NodeRecord) => void;
  onToggleEnable: (node: NodeRecord, next: boolean) => void;
  onUpdateNode: (node: NodeRecord) => void;
  onReinstallNode: (node: NodeRecord) => void;
  onRotateCredentials: (node: NodeRecord) => void;
  onReconcileNode: (node: NodeRecord) => void;
  onManageFirewall: (node: NodeRecord) => void;
  onUpdateSelected: () => void;
  onViewLogs: (node: NodeRecord) => void;
  onRestartAgent: (node: NodeRecord) => void;
  onRestartXray: (node: NodeRecord) => void;
  onPushConfig: (node: NodeRecord) => void;
  onPortGroups: () => void;
  onRegistrationTokens: () => void;
}

interface NodeRow extends NodeRecord {
  url: string;
  displayUrl: string;
  key: number;
}

function isUpdateEligible(n: NodeRecord): boolean {
  return !!n.enable && n.status === 'online';
}

function badgeStatus(status?: string): BadgeProps['status'] {
  switch (status) {
    case 'online': return 'success';
    case 'offline': return 'error';
    default: return 'default';
  }
}

function StatusDot({ status }: { status?: string }) {
  if (status === 'online') return <span className="online-dot" />;
  return <Badge status={badgeStatus(status)} />;
}

function StatusLabel({ status }: { status?: string }) {
  const { t } = useTranslation();
  return (
    <span style={status === 'online' ? { color: 'var(--ant-color-success)' } : undefined}>
      {t(`pages.nodes.statusValues.${status || 'unknown'}`)}
    </span>
  );
}

function formatPct(p?: number): string {
  if (typeof p !== 'number' || Number.isNaN(p)) return '-';
  return `${p.toFixed(1)}%`;
}

function formatUptime(secs?: number): string {
  if (!secs) return '-';
  const days = Math.floor(secs / 86400);
  const hours = Math.floor((secs % 86400) / 3600);
  if (days > 0) return `${days}d ${hours}h`;
  const mins = Math.floor((secs % 3600) / 60);
  if (hours > 0) return `${hours}h ${mins}m`;
  return `${mins}m`;
}

function useRelativeTime() {
  const { t } = useTranslation();
  return (unixSeconds?: number) => {
    if (!unixSeconds) return t('pages.nodes.never');
    const diffSec = Math.max(0, Math.floor(Date.now() / 1000 - unixSeconds));
    if (diffSec < 5) return t('pages.nodes.justNow');
    if (diffSec < 60) return `${diffSec}s`;
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m`;
    if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h`;
    return `${Math.floor(diffSec / 86400)}d`;
  };
}

export default function NodeList(props: NodeListProps) {
  const {
    nodes, loading, latestVersion,
    selectedIds,
    onAdd, onEdit, onDelete, onDeleteCleanup,
    onProbe, onToggleEnable, onUpdateNode, onReinstallNode,
    onRotateCredentials, onReconcileNode, onManageFirewall,
    onUpdateSelected, onViewLogs, onRestartAgent, onRestartXray,
    onPushConfig, onPortGroups, onRegistrationTokens,
  } = props;
  const { t } = useTranslation();
  const relativeTime = useRelativeTime();

  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());

  function toggleExpanded(id: number) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  const dataSource = useMemo<NodeRow[]>(
    () => nodes.map((n) => {
      const scheme = n.scheme || 'http';
      const address = n.address || '-';
      const port = n.port || 2053;
      const basePath = n.basePath || '/';
      return {
        ...n,
        url: `${scheme}://${address}:${port}${basePath}`,
        displayUrl: `${address}:${port}${basePath}`,
        key: n.id,
      };
    }),
    [nodes],
  );

  if (loading && dataSource.length === 0) {
    return (
      <Card size="small">
        <div className="card-empty">{t('loading')}</div>
      </Card>
    );
  }

  return (
    <Card size="small">
      <div className="toolbar">
        <Space wrap>
          <Button type="primary" icon={<PlusOutlined />} onClick={onAdd}>
            {t('pages.nodes.addNode')}
          </Button>
          {selectedIds.length > 0 && (
            <Button icon={<CloudDownloadOutlined />} onClick={onUpdateSelected}>
              {t('pages.nodes.updateSelected', { count: selectedIds.length })}
            </Button>
          )}
        </Space>
        <Space wrap>
          <Button icon={<KeyOutlined />} onClick={onRegistrationTokens}>
            {t('pages.nodes.registrationTokens') || 'Tokens'}
          </Button>
          <Button icon={<ClusterOutlined />} onClick={onPortGroups}>
            {t('pages.nodes.portGroups') || 'Groups'}
          </Button>
        </Space>
      </div>

      {dataSource.length === 0 ? (
        <div className="card-empty">
          <ClusterOutlined style={{ fontSize: 32, marginBottom: 8 }} />
          <div>{t('noData')}</div>
        </div>
      ) : (
        <div className="node-grid">
          {dataSource.map((record) => (
            <div key={record.id} className={`node-card${expandedIds.has(record.id) ? ' is-expanded' : ''}`}>
              {/* ── Header row ─────────────────────────────── */}
              <div className="card-head" onClick={() => toggleExpanded(record.id)}>
                <RightOutlined className={`card-expand${expandedIds.has(record.id) ? ' is-expanded' : ''}`} />
                <StatusDot status={record.status} />
                <span className="node-name">{record.name}</span>
                {record.group ? <Tag className="node-group-tag">{record.group}</Tag> : null}
                <div className="card-actions" onClick={(e) => e.stopPropagation()}>
                  <Switch
                    checked={!!record.enable}
                    size="small"
                    onChange={(v) => onToggleEnable(record, v)}
                  />
                  <Dropdown
                    trigger={['click']}
                    placement="bottomRight"
                    menu={{
                      items: [
                        {
                          key: 'probe',
                          label: <><ThunderboltOutlined /> {t('pages.nodes.probe')}</>,
                          onClick: () => onProbe(record),
                        },
                        ...(isUpdateEligible(record) ? [{
                          key: 'update',
                          label: <><CloudDownloadOutlined /> {t('pages.nodes.updatePanel')}</>,
                          onClick: () => onUpdateNode(record),
                        }] : []),
                        {
                          key: 'reinstall',
                          label: <><CloudDownloadOutlined /> {t('pages.nodes.reinstall')}</>,
                          onClick: () => onReinstallNode(record),
                        },
                        {
                          key: 'rotate',
                          label: <><KeyOutlined /> {t('pages.nodes.rotateCredentials')}</>,
                          onClick: () => onRotateCredentials(record),
                        },
                        {
                          key: 'reconcile',
                          label: <><CloudDownloadOutlined /> {t('pages.nodes.reconcile')}</>,
                          onClick: () => onReconcileNode(record),
                        },
                        {
                          key: 'firewall',
                          label: <><SafetyOutlined /> {t('pages.nodes.firewall')}</>,
                          onClick: () => onManageFirewall(record),
                        },
                        {
                          key: 'push',
                          label: <><CloudDownloadOutlined /> {t('pages.nodes.pushConfig')}</>,
                          onClick: () => onPushConfig(record),
                        },
                        {
                          key: 'logs',
                          label: <><FileTextOutlined /> {t('pages.nodes.viewLogs')}</>,
                          onClick: () => onViewLogs(record),
                        },
                        {
                          key: 'restartAgent',
                          label: <><ReloadOutlined /> {t('pages.nodes.restartAgent')}</>,
                          onClick: () => onRestartAgent(record),
                        },
                        {
                          key: 'restartXray',
                          label: <><ReloadOutlined /> {t('pages.nodes.restartXray')}</>,
                          onClick: () => onRestartXray(record),
                        },
                        { type: 'divider' },
                        {
                          key: 'edit',
                          label: <><EditOutlined /> {t('edit')}</>,
                          onClick: () => onEdit(record),
                        },
                        {
                          key: 'delete',
                          danger: true,
                          label: <><DeleteOutlined /> {t('delete')}</>,
                          onClick: () => onDelete(record),
                        },
                        {
                          key: 'cleanup',
                          danger: true,
                          label: <><CloudDownloadOutlined /> {t('pages.nodes.cleanupRemoteHint')}</>,
                          onClick: () => onDeleteCleanup(record),
                        },
                      ],
                    }}
                  >
                    <MoreOutlined className="row-action-trigger" />
                  </Dropdown>
                </div>
              </div>

              {/* ── Metrics row ─────────────────────────────── */}
              <div className="card-body">
                <div className="card-metrics">
                  <div className="metric" title={record.url}>
                    <span className="metric-label">{t('pages.nodes.address')}</span>
                    <span className="metric-value metric-address">
                      <a href={record.url} target="_blank" rel="noopener noreferrer">
                        {record.displayUrl}
                      </a>
                    </span>
                  </div>
                  <div className="metric-row">
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.status')}</span>
                      <span className="metric-value">
                        <StatusDot status={record.status} />
                        <StatusLabel status={record.status} />
                        {record.lastError && (
                          <Tooltip title={record.lastError}>
                            <ExclamationCircleOutlined style={{ color: 'var(--ant-color-warning)', marginLeft: 4 }} />
                          </Tooltip>
                        )}
                      </span>
                    </div>
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.cpu')}</span>
                      <span className="metric-value">{formatPct(record.cpuPct)}</span>
                    </div>
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.mem')}</span>
                      <span className="metric-value">{formatPct(record.memPct)}</span>
                    </div>
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.uptime')}</span>
                      <span className="metric-value">{formatUptime(record.uptimeSecs)}</span>
                    </div>
                  </div>
                  <div className="metric-row">
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.xrayVersion')}</span>
                      <span className="metric-value">{record.xrayVersion || '-'}</span>
                    </div>
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.panelVersion')}</span>
                      <span className="metric-value">
                        {record.panelVersion || '-'}
                        {isUpdateEligible(record) && record.panelVersion && isPanelUpdateAvailable(latestVersion || '', record.panelVersion as string) && (
                          <Tag color="orange" style={{ marginLeft: 4, cursor: 'pointer' }} onClick={() => onUpdateNode(record)}>
                            {t('pages.nodes.updateAvailable')}
                          </Tag>
                        )}
                      </span>
                    </div>
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.configVersion')}</span>
                      <span className="metric-value">
                        {record.configVersion != null && record.configVersion > 0
                          ? <Tag color="blue">v{record.configVersion}</Tag>
                          : <Tag color="orange">{t('pages.nodes.notPushed')}</Tag>}
                      </span>
                    </div>
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.latency')}</span>
                      <span className="metric-value">
                        {record.latencyMs && record.latencyMs > 0 ? `${record.latencyMs} ms` : '-'}
                      </span>
                    </div>
                  </div>
                  <div className="metric-row">
                    <div className="metric">
                      <span className="metric-label">{t('clients')}</span>
                      <span className="metric-value">
                        <Space size={4}>
                          <Tag color="green">{record.clientCount || 0}</Tag>
                          {record.onlineCount ? <Tag color="blue">{record.onlineCount} {t('online')}</Tag> : null}
                          {record.depletedCount ? <Tag color="red">{record.depletedCount} {t('depleted')}</Tag> : null}
                        </Space>
                      </span>
                    </div>
                    <div className="metric">
                      <span className="metric-label">{t('pages.nodes.lastHeartbeat')}</span>
                      <span className="metric-value">{relativeTime(record.lastHeartbeat ?? undefined)}</span>
                    </div>
                  </div>
                </div>
              </div>

              {/* ── Expanded history ────────────────────────── */}
              {expandedIds.has(record.id) && (
                <div className="card-history">
                  <NodeHistoryPanel node={record} />
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
