import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  Input,
  InputNumber,
  List,
  Modal,
  Row,
  Space,
  Tag,
  Typography,
  message,
} from 'antd';
import {
  CopyOutlined,
  DeleteOutlined,
  KeyOutlined,
  LinkOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import dayjs from 'dayjs';
import { useRegistrationTokens, type RegistrationToken } from '@/api/queries/useRegistrationMutations';

interface NodeRegistrationModalProps {
  open: boolean;
  onClose: () => void;
}

export default function NodeRegistrationModal({ open, onClose }: NodeRegistrationModalProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const { tokens, loading, refetch, generate, remove } = useRegistrationTokens();
  const [nodeName, setNodeName] = useState('');
  const [nodeAddress, setNodeAddress] = useState('');
  const [ttlMinutes, setTtlMinutes] = useState(1440);
  const [generating, setGenerating] = useState(false);
  const [lastToken, setLastToken] = useState<string | null>(null);

  useEffect(() => {
    if (open) refetch();
  }, [open, refetch]);

  const validTokens = useMemo(() => tokens.filter((t) => !t.consumedAt), [tokens]);

  const onGenerate = async () => {
    setGenerating(true);
    try {
      const msg = await generate({
        nodeName: nodeName || undefined,
        nodeAddress: nodeAddress || undefined,
        ttlMinutes,
      });
      if (msg?.success) {
        const tokenVal = (msg.obj as Record<string, unknown>)?.token as string;
        if (tokenVal) {
          setLastToken(tokenVal);
          messageApi.success(t('pages.nodes.tokenGenerated') || 'Token generated');
        }
        setNodeName('');
        setNodeAddress('');
      }
    } finally {
      setGenerating(false);
    }
  };

  const onDelete = async (id: number) => {
    const msg = await remove(id);
    if (msg?.success) messageApi.success(t('deleteSuccess'));
  };

  const installCommand = useMemo(() => {
    if (!lastToken) return '';
    const hubEndpoint = window.location.origin + ((window as unknown as Record<string, string>).L_UI_BASE_PATH || '/');
    return `curl -fsSL https://get.l-ui.dev/agent.sh | \\
  LUI_REGISTRATION_TOKEN=${lastToken} \\
  LUI_HUB_ENDPOINT=${hubEndpoint} \\
  sh`;
  }, [lastToken]);

  const tokenStatus = (token: RegistrationToken) => {
    if (token.consumedAt) return <Tag color="green">{t('pages.nodes.consumed') || 'Consumed'}</Tag>;
    const now = Date.now();
    if (token.expiresAt && token.expiresAt < now) return <Tag color="red">{t('pages.nodes.expired') || 'Expired'}</Tag>;
    const remaining = Math.max(0, Math.floor((token.expiresAt - now) / 60000));
    if (remaining < 60) return <Tag color="orange">{remaining}m</Tag>;
    return <Tag color="blue">{Math.floor(remaining / 60)}h</Tag>;
  };

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={
          <Space>
            <KeyOutlined />
            {t('pages.nodes.registrationTokens') || 'Registration Tokens'}
          </Space>
        }
        footer={null}
        width={640}
        onCancel={onClose}
      >
        {lastToken && (
          <Card
            size="small"
            title={t('pages.nodes.installCommand') || 'Install command'}
            style={{ marginBottom: 16 }}
            extra={
              <Button
                size="small"
                icon={<CopyOutlined />}
                onClick={() => {
                  navigator.clipboard.writeText(installCommand).catch(() => {});
                  messageApi.success(t('copied') || 'Copied');
                }}
              >
                {t('copy') || 'Copy'}
              </Button>
            }
          >
            <Typography.Paragraph
              code
              copyable
              style={{
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
                fontSize: 12,
                margin: 0,
                padding: 8,
                background: 'var(--ant-color-bg-layout)',
                borderRadius: 4,
              }}
            >
              {installCommand}
            </Typography.Paragraph>
          </Card>
        )}

        <Card size="small" title={t('pages.nodes.generateToken') || 'Generate Token'} style={{ marginBottom: 16 }}>
          <Row gutter={16}>
            <Col xs={24} md={8}>
              <Input
                placeholder={t('pages.nodes.nodeName') || 'Node name (optional)'}
                value={nodeName}
                onChange={(e) => setNodeName(e.target.value)}
              />
            </Col>
            <Col xs={24} md={8}>
              <Input
                placeholder={t('pages.nodes.nodeAddress') || 'Node address (optional)'}
                value={nodeAddress}
                onChange={(e) => setNodeAddress(e.target.value)}
              />
            </Col>
            <Col xs={12} md={4}>
              <InputNumber
                min={5}
                max={43200}
                value={ttlMinutes}
                onChange={(v) => setTtlMinutes(v ?? 1440)}
                style={{ width: '100%' }}
                addonAfter={t('minutes') || 'min'}
              />
            </Col>
            <Col xs={12} md={4}>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                loading={generating}
                onClick={onGenerate}
                style={{ width: '100%' }}
              >
                {t('generate') || 'Generate'}
              </Button>
            </Col>
          </Row>
        </Card>

        <Card
          size="small"
          title={
            <Space>
              <LinkOutlined />
              {t('pages.nodes.activeTokens') || 'Active Tokens'}
              <Tag>{validTokens.length}</Tag>
            </Space>
          }
          loading={loading}
        >
          <List
            size="small"
            dataSource={validTokens}
            locale={{ emptyText: t('noData') }}
            renderItem={(token) => (
              <List.Item
                actions={[
                  token.consumedAt ? null : (
                    <Button
                      key="delete"
                      type="text"
                      danger
                      size="small"
                      icon={<DeleteOutlined />}
                      onClick={() => onDelete(token.id)}
                    />
                  ),
                ].filter(Boolean)}
              >
                <List.Item.Meta
                  title={
                    <Space>
                      <Typography.Text code style={{ fontSize: 11 }}>
                        {token.token.slice(0, 16)}...
                      </Typography.Text>
                      {tokenStatus(token)}
                    </Space>
                  }
                  description={
                    <Space size={4}>
                      {token.nodeName && <span>{token.nodeName}</span>}
                      {token.nodeAddress && <span>@{token.nodeAddress}</span>}
                      <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                        {dayjs(token.createdAt).format('YYYY-MM-DD HH:mm')}
                      </Typography.Text>
                    </Space>
                  }
                />
              </List.Item>
            )}
          />
        </Card>
      </Modal>
    </>
  );
}
