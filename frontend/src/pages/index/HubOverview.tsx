import { useMemo } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Col, Row, Space, Statistic, Tag, Tooltip } from 'antd';
import {
  CloudServerOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  GlobalOutlined,
  ThunderboltOutlined,
  ToolOutlined,
  WifiOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';

import type { Status } from '@/models/status';
import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { PanelContext } from '@/api/queries/usePanelContext';
import { useServerHistorySeries } from '@/api/queries/useServerHistorySeries';
import { SizeFormatter, TimeFormatter } from '@/utils';
import { Sparkline } from '@/components/viz';
import './HubOverview.css';

interface HubOverviewProps {
  context: PanelContext;
  status: Status;
  nodes: NodeRecord[];
}

function pct(current?: number, total?: number) {
  if (!current || !total) return 0;
  return Math.max(0, Math.min(100, (current * 100) / total));
}

function formatAgo(seconds: number): string {
  if (!seconds || seconds < 0) return 'now';
  return `${TimeFormatter.formatSecond(seconds)} ago`;
}

function hostMetricColor(name: string) {
  if (name === 'disk') return '#13c2c2';
  if (name === 'network') return '#1890ff';
  if (name === 'load') return '#fa8c16';
  if (name === 'mem') return '#7c4dff';
  return '#52c41a';
}

function MetricCard({
  title,
  value,
  suffix,
  icon,
  labels,
  data,
  data2,
  data3,
  stroke,
  stroke2,
  stroke3,
  valueMax,
  unit,
}: {
  title: string;
  value: string;
  suffix?: string;
  icon: ReactNode;
  labels: string[];
  data: number[];
  data2?: number[];
  data3?: number[];
  stroke: string;
  stroke2?: string;
  stroke3?: string;
  valueMax?: number | null;
  unit: string;
}) {
  return (
    <Card hoverable size="small" className="overview-metric-card" title={title}>
      <div className="overview-metric-head">
        <Statistic title={title} value={value} suffix={suffix} prefix={icon} />
      </div>
      <Sparkline
        data={data}
        data2={data2}
        data3={data3}
        labels={labels}
        height={120}
        stroke={stroke}
        stroke2={stroke2}
        stroke3={stroke3}
        maxPoints={60}
        showGrid
        showAxes={false}
        showTooltip
        showMarker={false}
        fillOpacity={0.14}
        strokeWidth={2}
        valueMin={0}
        valueMax={valueMax ?? null}
        yFormatter={(v) => (unit === 'B/s' ? `${SizeFormatter.sizeFormat(v).replace(/\.\d+/, '')}/s` : unit === '%' ? `${v.toFixed(0)}%` : `${Math.round(v)}`)}
      />
    </Card>
  );
}

function NodeCard({ node }: { node: NodeRecord }) {
  const { t } = useTranslation();
  const staleSeconds = node.lastHeartbeat ? Math.max(0, Math.floor(Date.now() / 1000) - node.lastHeartbeat) : Number.POSITIVE_INFINITY;
  const stale = staleSeconds > 45;
  const offline = node.status !== 'online';
  const diskPct = pct(node.diskCurrent, node.diskTotal);
  const cpu = Number(node.cpuPct || 0);
  const mem = Number(node.memPct || 0);
  const latency = Number(node.latencyMs || 0);
  const title = node.name || node.address || `node-${node.id}`;

  return (
    <Card
      hoverable
      size="small"
      className={`overview-node-card${stale ? ' is-stale' : ''}`}
      title={<Space size={8}><span>{title}</span>{offline ? <Tag color="error">Offline</Tag> : stale ? <Tag color="orange">Stale</Tag> : <Tag color="success">Online</Tag>}</Space>}
      extra={<Button type="link" onClick={() => { const base = (window.L_UI_BASE_PATH || '').replace(/\/$/, ''); window.location.href = `${base}/panel/nodes`; }}>{t('menu.nodes')}</Button>}
    >
      <div className="overview-node-subtitle">
        <span>{node.address}:{node.port}</span>
        <Tooltip title={node.lastHeartbeat ? new Date(node.lastHeartbeat * 1000).toLocaleString() : t('pages.nodes.neverSeen') || 'Never seen'}>
          <span><ClockCircleOutlined /> {node.lastHeartbeat ? formatAgo(staleSeconds) : (t('pages.nodes.neverSeen') || 'Never seen')}</span>
        </Tooltip>
      </div>

      <Row gutter={[8, 8]} className="overview-node-stats">
        <Col xs={12} sm={12} md={12}>
          <Statistic title="CPU" value={`${cpu.toFixed(1)}%`} prefix={<DashboardOutlined />} />
        </Col>
        <Col xs={12} sm={12} md={12}>
          <Statistic title="RAM" value={`${mem.toFixed(1)}%`} prefix={<DatabaseOutlined />} />
        </Col>
        <Col xs={12} sm={12} md={12}>
          <Statistic title="Disk" value={`${diskPct.toFixed(1)}%`} prefix={<CloudServerOutlined />} />
          <div className="overview-node-detail">
            {SizeFormatter.sizeFormat(node.diskCurrent || 0)} / {SizeFormatter.sizeFormat(node.diskTotal || 0)}
          </div>
        </Col>
        <Col xs={12} sm={12} md={12}>
          <Statistic title="Latency" value={latency > 0 ? `${latency} ms` : '-'} prefix={<ThunderboltOutlined />} />
        </Col>
      </Row>

      <div className="overview-node-footer">
        <Space wrap size={10}>
          <span><WifiOutlined /> {SizeFormatter.sizeFormat(node.netUp || 0).replace(/\.\d+/, '')}/s up</span>
          <span><GlobalOutlined /> {SizeFormatter.sizeFormat(node.netDown || 0).replace(/\.\d+/, '')}/s down</span>
          {node.xrayVersion && <span><ToolOutlined /> {node.xrayVersion}</span>}
        </Space>
        {node.lastError && <div className="overview-node-error">{node.lastError}</div>}
      </div>
    </Card>
  );
}

export default function HubOverview({ context, status, nodes }: HubOverviewProps) {
  const { t } = useTranslation();
  const cpuHistory = useServerHistorySeries({ metric: 'cpu', bucket: 30 });
  const memHistory = useServerHistorySeries({ metric: 'mem', bucket: 30 });
  const diskHistory = useServerHistorySeries({ metric: 'diskUsage', bucket: 30 });
  const netHistory = useServerHistorySeries({ metric: 'netUp', secondaryMetric: 'netDown', bucket: 30 });
  const loadHistory = useServerHistorySeries({ metric: 'load1', secondaryMetric: 'load5', tertiaryMetric: 'load15', bucket: 30 });

  const nodeCards = useMemo(() => nodes.map((node) => ({ node })), [nodes]);
  const basePath = (window.L_UI_BASE_PATH || '').replace(/\/$/, '');

  return (
    <div className="hub-overview">
      <Card hoverable className="overview-summary-card" title="Host summary" extra={<Button type="link" onClick={() => { window.location.href = `${basePath}/panel/nodes`; }}>{t('menu.nodes')}</Button>}>
        <Row gutter={[12, 12]}>
          <Col xs={12} md={6}><Statistic title="CPU" value={`${status.cpu.percent.toFixed(1)}%`} prefix={<DashboardOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="RAM" value={`${pct(status.mem?.current, status.mem?.total).toFixed(1)}%`} prefix={<DatabaseOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="Disk" value={`${pct(status.disk?.current, status.disk?.total).toFixed(1)}%`} prefix={<CloudServerOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="Network" value={`${SizeFormatter.sizeFormat(status.netIO?.up || 0).replace(/\.\d+/, '')}/s ↑`} prefix={<GlobalOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="Uptime" value={TimeFormatter.formatSecond(status.uptime || 0)} prefix={<ClockCircleOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="Load" value={status.loads?.length ? status.loads.map((v) => v.toFixed(2)).join(' / ') : '-'} prefix={<ThunderboltOutlined />} /></Col>
          <Col xs={12} md={6}><Statistic title="Mode" value={context.mode.toUpperCase()} /></Col>
          <Col xs={12} md={6}><Statistic title="Version" value={context.version || '—'} /></Col>
        </Row>
      </Card>

      <Row gutter={[12, 12]} className="overview-trend-grid">
        <Col xs={24} lg={12}><MetricCard title="CPU trend" value={`${status.cpu.percent.toFixed(1)}%`} icon={<DashboardOutlined />} labels={cpuHistory.labels} data={cpuHistory.data} stroke={hostMetricColor('cpu')} valueMax={100} unit="%" /></Col>
        <Col xs={24} lg={12}><MetricCard title="RAM trend" value={`${pct(status.mem?.current, status.mem?.total).toFixed(1)}%`} icon={<DatabaseOutlined />} labels={memHistory.labels} data={memHistory.data} stroke={hostMetricColor('mem')} valueMax={100} unit="%" /></Col>
        <Col xs={24} lg={12}><MetricCard title="Disk trend" value={`${pct(status.disk?.current, status.disk?.total).toFixed(1)}%`} icon={<CloudServerOutlined />} labels={diskHistory.labels} data={diskHistory.data} stroke={hostMetricColor('disk')} valueMax={100} unit="%" /></Col>
        <Col xs={24} lg={12}><MetricCard title="Network trend" value={`${SizeFormatter.sizeFormat(status.netIO?.up || 0).replace(/\.\d+/, '')}/s`} suffix="↑" icon={<GlobalOutlined />} labels={netHistory.labels} data={netHistory.data} data2={netHistory.data2} stroke={hostMetricColor('network')} stroke2="#13c2c2" unit="B/s" /></Col>
        <Col xs={24}><MetricCard title="Load average" value={status.loads?.length ? status.loads[0].toFixed(2) : '-'} icon={<ThunderboltOutlined />} labels={loadHistory.labels} data={loadHistory.data} data2={loadHistory.data2} data3={loadHistory.data3} stroke={hostMetricColor('load')} stroke2="#f5222d" stroke3="#a0d911" unit="" /></Col>
      </Row>

      <Card hoverable className="overview-node-section" title={`VPS nodes (${nodes.length})`}>
        <Row gutter={[12, 12]}>
          {nodeCards.length === 0 ? (
            <Col span={24}>
              <div className="overview-empty-state">No nodes connected yet.</div>
            </Col>
          ) : nodeCards.map(({ node }) => (
            <Col key={node.id} xs={24} md={12} xl={8}>
              <NodeCard node={node} />
            </Col>
          ))}
        </Row>
      </Card>

      <Card hoverable className="overview-note-card">
        <Space direction="vertical" size={4}>
          <strong>Overview notes</strong>
          <span>Host metrics refresh every 10 seconds. Node cards show a stale warning when no heartbeat arrives.</span>
        </Space>
      </Card>
    </div>
  );
}
