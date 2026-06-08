import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, InputNumber, Modal, Space, Spin, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import type { NodeRecord } from '@/api/queries/useNodesQuery';

interface NodeLogsModalProps {
  open: boolean;
  node: NodeRecord | null;
  onClose: () => void;
  fetchLogs: (id: number, lines?: number) => Promise<{ obj?: string | null }>;
}

export default function NodeLogsModal({ open, node, onClose, fetchLogs }: NodeLogsModalProps) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<string>('');
  const [lines, setLines] = useState(200);
  const [error, setError] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const loadLogs = useCallback(async () => {
    if (!node) return;
    setLoading(true);
    setError(null);
    try {
      const msg = await fetchLogs(node.id, lines);
      setLogs(typeof msg.obj === 'string' ? msg.obj : '');
    } catch (e) {
      setError(String(e));
      setLogs('');
    } finally {
      setLoading(false);
    }
  }, [node, lines, fetchLogs]);

  useEffect(() => {
    if (open && node) loadLogs();
  }, [open, node, loadLogs]);

  useEffect(() => {
    if (bottomRef.current) {
      bottomRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs]);

  return (
    <Modal
      open={open}
      title={node ? `${node.name} — ${t('pages.nodes.logs') || 'Logs'}` : ''}
      footer={
        <Space>
          <span>{t('pages.nodes.lines') || 'Lines'}:</span>
          <InputNumber
            min={10}
            max={5000}
            value={lines}
            onChange={(v) => setLines(v ?? 200)}
            size="small"
            style={{ width: 80 }}
          />
          <Button icon={<ReloadOutlined />} loading={loading} onClick={loadLogs}>
            {t('refresh')}
          </Button>
          <Button onClick={onClose}>{t('close')}</Button>
        </Space>
      }
      width={800}
      onCancel={onClose}
    >
      <Spin spinning={loading} delay={100}>
        {error ? (
          <Typography.Text type="danger">{error}</Typography.Text>
        ) : logs ? (
          <pre
            style={{
              maxHeight: 500,
              overflow: 'auto',
              fontSize: 11,
              lineHeight: 1.4,
              margin: 0,
              padding: 8,
              background: 'var(--ant-color-bg-layout)',
              borderRadius: 4,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {logs}
            <div ref={bottomRef} />
          </pre>
        ) : (
          <Typography.Text type="secondary">{t('noData')}</Typography.Text>
        )}
      </Spin>
    </Modal>
  );
}
