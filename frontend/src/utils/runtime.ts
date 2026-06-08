export type PanelMode = 'hub' | 'agent';

export function getPanelMode(): PanelMode {
  return window.L_UI_MODE === 'agent' ? 'agent' : 'hub';
}

export function getApiPrefix(): string {
  return window.L_UI_API_PREFIX || (getPanelMode() === 'agent' ? '/api/v1' : '/panel/api');
}

export function isHubMode(): boolean {
  return getPanelMode() === 'hub';
}
