import { Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import { message } from 'antd';
import 'antd/dist/reset.css';

import { readyI18n } from '@/i18n/react';
import { ThemeProvider } from '@/hooks/useTheme';
import { QueryProvider } from '@/api/QueryProvider';
import SubPage from '@/pages/sub/SubPage';

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
  root.innerHTML = '<div style="min-height:100vh;display:flex;align-items:center;justify-content:center;font-family:sans-serif;">Loading subscription…</div>';
  createRoot(root).render(
    <Suspense fallback={null}>
      <ThemeProvider>
        <QueryProvider>
          <SubPage />
        </QueryProvider>
      </ThemeProvider>
    </Suspense>,
  );
}

void readyI18n().catch((error) => {
  console.error('subpage i18n bootstrap failed:', error);
  showBootstrapError(error);
});
