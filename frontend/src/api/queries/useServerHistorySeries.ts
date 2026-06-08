import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';

export interface HistoryPoint {
  t: number;
  v: number;
}

export interface HistorySeriesResult {
  labels: string[];
  data: number[];
  data2: number[];
  data3: number[];
  loading: boolean;
  error: string;
  refetch: () => Promise<unknown>;
}

interface Options {
  metric: string;
  bucket?: number;
  secondaryMetric?: string;
  tertiaryMetric?: string;
  enabled?: boolean;
}

async function fetchMetric(metric: string, bucket: number): Promise<HistoryPoint[]> {
  const msg = await HttpUtil.get<HistoryPoint[]>(`/panel/api/server/history/${metric}/${bucket}`, undefined, { silent: true });
  if (!msg?.success || !Array.isArray(msg.obj)) {
    throw new Error(msg?.msg || `Failed to fetch ${metric} history`);
  }
  return msg.obj;
}

function alignSeries(base: HistoryPoint[], other: HistoryPoint[]): number[] {
  const byTs = new Map<number, number>();
  for (const p of other) byTs.set(Number(p.t) || 0, Number(p.v) || 0);
  return base.map((p) => byTs.get(Number(p.t) || 0) ?? 0);
}

export function useServerHistorySeries({ metric, bucket = 30, secondaryMetric, tertiaryMetric, enabled = true }: Options): HistorySeriesResult {
  const query = useQuery({
    queryKey: ['server', 'history', metric, bucket, secondaryMetric || '', tertiaryMetric || ''],
    queryFn: async () => {
      const [primary, secondary, tertiary] = await Promise.all([
        fetchMetric(metric, bucket),
        secondaryMetric ? fetchMetric(secondaryMetric, bucket) : Promise.resolve([] as HistoryPoint[]),
        tertiaryMetric ? fetchMetric(tertiaryMetric, bucket) : Promise.resolve([] as HistoryPoint[]),
      ]);
      return {
        primary,
        secondary,
        tertiary,
      };
    },
    enabled,
    refetchInterval: 10_000,
    staleTime: 8_000,
  });

  const series = useMemo(() => {
    const primary = query.data?.primary ?? [];
    const secondary = query.data?.secondary ?? [];
    const tertiary = query.data?.tertiary ?? [];
    const labels: string[] = [];
    const data: number[] = [];
    const baseTs: number[] = [];
    for (const p of primary) {
      baseTs.push(Number(p.t) || 0);
      data.push(Number(p.v) || 0);
      const d = new Date((Number(p.t) || 0) * 1000);
      const hh = String(d.getHours()).padStart(2, '0');
      const mm = String(d.getMinutes()).padStart(2, '0');
      const ss = String(d.getSeconds()).padStart(2, '0');
      labels.push(bucket >= 60 ? `${hh}:${mm}` : `${hh}:${mm}:${ss}`);
    }
    return {
      labels,
      data,
      data2: secondaryMetric ? alignSeries(primary, secondary) : [],
      data3: tertiaryMetric ? alignSeries(primary, tertiary) : [],
    };
  }, [bucket, query.data, secondaryMetric, tertiaryMetric]);

  return {
    labels: series.labels,
    data: series.data,
    data2: series.data2,
    data3: series.data3,
    loading: query.isFetching && !query.data,
    error: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}
