import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { HttpUtil } from '@/utils';
import { Sparkline } from '@/components/viz';
import './NodeHistoryPanel.css';

interface NodeRef {
  id: number;
}

interface NodeHistoryPanelProps {
  node: NodeRef;
  bucket?: number;
}

interface SeriesPoint {
  t: number;
  v: number;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const REFRESH_MS = 15000;

function formatBytes(v: number): string {
  if (v >= 1e12) return `${(v / 1e12).toFixed(1)} TB`;
  if (v >= 1e9) return `${(v / 1e9).toFixed(1)} GB`;
  if (v >= 1e6) return `${(v / 1e6).toFixed(1)} MB`;
  if (v >= 1e3) return `${(v / 1e3).toFixed(0)} KB`;
  return `${v.toFixed(0)} B`;
}

export default function NodeHistoryPanel({ node, bucket = 30 }: NodeHistoryPanelProps) {
  const { t } = useTranslation();
  const [cpuPoints, setCpuPoints] = useState<number[]>([]);
  const [cpuLabels, setCpuLabels] = useState<string[]>([]);
  const [memPoints, setMemPoints] = useState<number[]>([]);
  const [memLabels, setMemLabels] = useState<string[]>([]);
  const [netUpPoints, setNetUpPoints] = useState<number[]>([]);
  const [netDownPoints, setNetDownPoints] = useState<number[]>([]);
  const [netLabels, setNetLabels] = useState<string[]>([]);
  const [diskPoints, setDiskPoints] = useState<number[]>([]);
  const [diskLabels, setDiskLabels] = useState<string[]>([]);

  const lastNodeId = useRef<number>(node.id);

  useEffect(() => {
    let cancelled = false;

    const bucketLabel = (unixSec: number) => {
      const d = new Date(unixSec * 1000);
      const hh = String(d.getHours()).padStart(2, '0');
      const mm = String(d.getMinutes()).padStart(2, '0');
      if (bucket >= 60) return `${hh}:${mm}`;
      const ss = String(d.getSeconds()).padStart(2, '0');
      return `${hh}:${mm}:${ss}`;
    };

    const fetchSeries = async (metric: string, clampPct = true) => {
      try {
        const url = `/panel/api/nodes/history/${node.id}/${metric}/${bucket}`;
        const msg = await HttpUtil.get(url) as ApiMsg<SeriesPoint[]>;
        if (msg?.success && Array.isArray(msg.obj)) {
          const vals: number[] = [];
          const labs: string[] = [];
          for (const p of msg.obj) {
            labs.push(bucketLabel(p.t));
            const v = Number(p.v) || 0;
            vals.push(clampPct ? Math.max(0, Math.min(100, v)) : v);
          }
          return { vals, labs };
        }
      } catch (e) {
        console.error('node history fetch failed', metric, e);
      }
      return { vals: [] as number[], labs: [] as string[] };
    };

    const refresh = async () => {
      const [cpu, mem, netUp, netDown, disk] = await Promise.all([
        fetchSeries('cpu'),
        fetchSeries('mem'),
        fetchSeries('netUp', false),
        fetchSeries('netDown', false),
        fetchSeries('diskUsage'),
      ]);
      if (cancelled) return;
      setCpuPoints(cpu.vals);
      setCpuLabels(cpu.labs);
      setMemPoints(mem.vals);
      setMemLabels(mem.labs);
      setNetUpPoints(netUp.vals);
      setNetDownPoints(netDown.vals);
      setNetLabels(netDown.labs);
      setDiskPoints(disk.vals);
      setDiskLabels(disk.labs);
    };

    refresh();
    const timer = window.setInterval(refresh, REFRESH_MS);
    lastNodeId.current = node.id;

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [node.id, bucket]);

  return (
    <div className="node-history-panel">
      <div className="series">
        <div className="series-title">{t('pages.nodes.cpu')}</div>
        <Sparkline
          data={cpuPoints}
          labels={cpuLabels}
          height={120}
          stroke="#008771"
          showGrid
          showAxes
          tickCountX={4}
          maxPoints={cpuPoints.length || 1}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
        />
      </div>
      <div className="series">
        <div className="series-title">{t('pages.nodes.mem')}</div>
        <Sparkline
          data={memPoints}
          labels={memLabels}
          height={120}
          stroke="#7c4dff"
          showGrid
          showAxes
          tickCountX={4}
          maxPoints={memPoints.length || 1}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
        />
      </div>
      <div className="series">
        <div className="series-title">{t('pages.nodes.network') || 'Network'}</div>
        <Sparkline
          data={netUpPoints}
          data2={netDownPoints}
          name1={t('pages.nodes.netUp') || 'Up'}
          name2={t('pages.nodes.netDown') || 'Down'}
          labels={netLabels}
          height={120}
          stroke="#13c2c2"
          stroke2="#fa8c16"
          showGrid
          showAxes
          tickCountX={4}
          valueMin={0}
          valueMax={null}
          maxPoints={Math.max(netUpPoints.length, 1)}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
          yFormatter={formatBytes}
          tooltipFormatter={formatBytes}
        />
      </div>
      <div className="series">
        <div className="series-title">{t('pages.nodes.disk') || 'Disk'}</div>
        <Sparkline
          data={diskPoints}
          labels={diskLabels}
          height={120}
          stroke="#eb2f96"
          showGrid
          showAxes
          tickCountX={4}
          maxPoints={diskPoints.length || 1}
          fillOpacity={0.18}
          markerRadius={2.6}
          showTooltip
        />
      </div>
    </div>
  );
}
