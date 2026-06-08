import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Modal, Select, Typography, message } from 'antd';

import type { NodeRecord } from '@/api/queries/useNodesQuery';
import type { InboundOption } from '@/hooks/useClients';
import type { ClientMoveResult } from '@/schemas/client';

interface ClientMoveModalProps {
  open: boolean;
  count: number;
  emails: string[];
  nodes: NodeRecord[];
  inbounds: InboundOption[];
  defaultSourceNodeId?: number;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: {
    emails: string[];
    sourceNodeId: number;
    targetNodeId: number;
    targetInboundId: number;
  }) => Promise<ClientMoveResult | null>;
}

function nodeLabel(node: NodeRecord): string {
  const name = node.name?.trim() || node.address?.trim() || '';
  return node.port ? `${name}:${node.port}` : name;
}

function inboundLabel(inbound: InboundOption): string {
  return inbound.remark?.trim() || inbound.tag || `#${inbound.id}`;
}

function matchesNode(nodeId: number | null | undefined, targetNodeId: number): boolean {
  if (targetNodeId <= 0) return nodeId == null;
  return nodeId === targetNodeId;
}

export default function ClientMoveModal({
  open,
  count,
  emails,
  nodes,
  inbounds,
  defaultSourceNodeId,
  onOpenChange,
  onSubmit,
}: ClientMoveModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [sourceNodeId, setSourceNodeId] = useState<number>(0);
  const [targetNodeId, setTargetNodeId] = useState<number>(0);
  const [targetInboundId, setTargetInboundId] = useState<number>(0);
  const [submitting, setSubmitting] = useState(false);
  const title = count === 1 ? 'Move 1 client' : `Move ${count} clients`;

  useEffect(() => {
    if (!open) return;
    const firstNode = nodes[0]?.id ?? 0;
    const initialSource = defaultSourceNodeId ?? firstNode;
    const initialTarget = nodes.find((node) => node.id !== initialSource)?.id ?? firstNode;
    setSourceNodeId(initialSource);
    setTargetNodeId(initialTarget);
    setTargetInboundId(0);
  }, [open, nodes, defaultSourceNodeId]);

  const sourceOptions = useMemo(() => nodes.map((node) => ({ value: node.id, label: nodeLabel(node) })), [nodes]);
  const targetOptions = useMemo(() => nodes.map((node) => ({ value: node.id, label: nodeLabel(node) })), [nodes]);
  const inboundOptions = useMemo(() => inbounds
    .filter((inbound) => matchesNode(inbound.nodeId, targetNodeId))
    .map((inbound) => ({ value: inbound.id, label: inboundLabel(inbound) })), [inbounds, targetNodeId]);

  useEffect(() => {
    if (targetInboundId !== 0 && !inboundOptions.some((option) => option.value === targetInboundId)) {
      setTargetInboundId(0);
    }
  }, [targetInboundId, inboundOptions]);

  async function submit() {
    if (count === 0 || emails.length === 0 || sourceNodeId <= 0 || targetNodeId <= 0 || targetInboundId <= 0) return;
    setSubmitting(true);
    try {
      const result = await onSubmit({ emails, sourceNodeId, targetNodeId, targetInboundId });
      if (!result) return;
      const moved = result.moved?.length ?? 0;
      const skipped = result.skipped?.length ?? 0;
      const errors = result.errors?.length ?? 0;
      const summary = `${moved} moved, ${skipped} skipped, ${errors} failed`;
      if (errors > 0 || skipped > 0) messageApi.warning(summary);
      else messageApi.success(summary);
      onOpenChange(false);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={t('pages.clients.moveClientsTitle', { count, defaultValue: title })}
        okText={t('pages.clients.move', { defaultValue: 'Move' })}
        cancelText={t('cancel')}
        okButtonProps={{ disabled: sourceNodeId <= 0 || targetNodeId <= 0 || targetInboundId <= 0, loading: submitting }}
        onCancel={() => onOpenChange(false)}
        onOk={submit}
        destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          {t('pages.clients.moveClientsDesc', { count, defaultValue: `Move ${count} client(s) between nodes and inbounds.` })}
        </Typography.Paragraph>
        {nodes.length === 0 ? (
          <Alert type="info" showIcon message={t('pages.clients.moveClientsNoNodes', { defaultValue: 'No nodes available.' })} />
        ) : (
          <div style={{ display: 'grid', gap: 12 }}>
            <Select
              value={sourceNodeId || undefined}
              onChange={(value) => setSourceNodeId(value)}
              options={sourceOptions}
              placeholder={t('pages.clients.moveSourceNode', { defaultValue: 'Source node' })}
              style={{ width: '100%' }}
              optionFilterProp="label"
            />
            <Select
              value={targetNodeId || undefined}
              onChange={(value) => {
                setTargetNodeId(value);
                setTargetInboundId(0);
              }}
              options={targetOptions}
              placeholder={t('pages.clients.moveTargetNode', { defaultValue: 'Target node' })}
              style={{ width: '100%' }}
              optionFilterProp="label"
            />
            {targetNodeId > 0 && (
              <Select
                value={targetInboundId || undefined}
                onChange={(value) => setTargetInboundId(value)}
                options={inboundOptions}
                placeholder={t('pages.clients.moveTargetInbound', { defaultValue: 'Target inbound' })}
                style={{ width: '100%' }}
                optionFilterProp="label"
                notFoundContent={t('pages.clients.moveNoInbounds', { defaultValue: 'No inbounds on the selected node.' })}
              />
            )}
          </div>
        )}
      </Modal>
    </>
  );
}
