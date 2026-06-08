import { Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import { message, Spin } from 'antd';
import 'antd/dist/reset.css';

import { setupAxios } from '@/api/axios-init';
import { applyDocumentTitle } from '@/utils';
import { readyI18n } from '@/i18n/react';
import { ThemeProvider } from '@/hooks/useTheme';
import { QueryProvider } from '@/api/QueryProvider';
import LoginPage from '@/pages/login/LoginPage';

setupAxios();
applyDocumentTitle();

const messageContainer = document.getElementById('message');
if (messageContainer) {
  message.config({ getContainer: () => messageContainer });
}

function showBootstrapError(error: unknown): void {
  const text = error instanceof Error ? error.stack || error.message : String(error);
  if (messageContainer) {
    messageContainer.textContent = `Startup error: ${text}`;
    messageContainer.setAttribute('style', 'padding:16px;color:#b42318;background:#fff1f0;font:14px/1.5 monospace;white-space:pre-wrap;');
  }
}

window.addEventListener('error', (event) => showBootstrapError(event.error || event.message));
window.addEventListener('unhandledrejection', (event) => showBootstrapError(event.reason));

const root = document.getElementById('app');
if (root) {
  root.innerHTML = '<div style="min-height:100vh;display:flex;align-items:center;justify-content:center;font-family:sans-serif;">Loading sign in…</div>';
  createRoot(root).render(
    <Suspense fallback={<div className="login-loading"><Spin size="large" /></div>}>
      <ThemeProvider>
        <QueryProvider>
          <LoginPage />
        </QueryProvider>
      </ThemeProvider>
    </Suspense>,
  );
}

void readyI18n().catch((error) => {
  console.error('login i18n bootstrap failed:', error);
  showBootstrapError(error);
});
