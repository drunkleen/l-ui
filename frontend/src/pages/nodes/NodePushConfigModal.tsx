import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Space, message } from 'antd';
import { CloudUploadOutlined } from '@ant-design/icons';

import type { NodeRecord } from '@/api/queries/useNodesQuery';
interface PushResult {
  success?: boolean;
  msg?: string;
  obj?: { config_version?: number } | null;
}

interface Props {
  open: boolean;
  node: NodeRecord | null;
  onClose: () => void;
  pushConfig: (id: number) => Promise<PushResult>;
}

export default function NodePushConfigModal({
  open,
  node,
  onClose,
  pushConfig,
}: Props) {
  const { t } = useTranslation();
  const [messageApi, contextHolder] = message.useMessage();
  const [pushing, setPushing] = useState(false);
  const [result, setResult] = useState<PushResult | null>(null);

  async function handlePush() {
    if (!node?.id) return;
    setPushing(true);
    setResult(null);
    try {
      const msg = await pushConfig(node.id);
      if (msg?.success) {
        messageApi.success(t('pages.nodes.toasts.update'));
        setResult(msg);
      } else {
        messageApi.error(msg?.msg || t('somethingWentWrong'));
        setResult(msg);
      }
    } finally {
      setPushing(false);
    }
  }

  function handleClose() {
    setResult(null);
    onClose();
  }

  return (
    <Modal
      open={open}
      onCancel={handleClose}
      footer={null}
      width={480}
      title={t('pages.nodes.pushConfig') || 'Push Config'}
    >
      {contextHolder}
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Alert
          type="info"
          showIcon
          message={node?.name || node?.address || ''}
          description={t('pages.nodes.pushConfigDesc') || 'Push the current Xray config and client list to this node.'}
        />

        {result && (
          <Alert
            type={result.success ? 'success' : 'error'}
            showIcon
            message={result.success ? (t('pages.nodes.pushConfigSuccess') || 'Config pushed successfully') : (t('somethingWentWrong') || 'Push failed')}
            description={
              result.success && result.obj?.config_version != null
                ? `${t('pages.nodes.configVersion') || 'Config version'}: ${result.obj.config_version}`
                : result.msg || ''
            }
          />
        )}

        {!result && (
          <Button
            type="primary"
            icon={<CloudUploadOutlined />}
            loading={pushing}
            onClick={handlePush}
            block
            size="large"
          >
            {t('pages.nodes.pushConfigAction') || 'Push Config to Node'}
          </Button>
        )}

        {result && (
          <Button type="default" onClick={handleClose} block>
            {t('close') || 'Close'}
          </Button>
        )}
      </Space>
    </Modal>
  );
}
