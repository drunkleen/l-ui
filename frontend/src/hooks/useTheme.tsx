import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

// ── Catppuccin Mocha palette (dark mode, default) ─────────────────
const MOCHA = {
  base: '#1e1e2e', mantle: '#181825', crust: '#11111b',
  surface0: '#313244', surface1: '#45475a', surface2: '#585b70',
  text: '#cdd6f4', blue: '#89b4fa', green: '#a6e3a1',
  yellow: '#f9e2af', red: '#f38ba8',
};

// ── Catppuccin Frappé palette (soft dark mode) ────────────────────
const FRAPPE = {
  base: '#303446', mantle: '#292c3c', crust: '#232634',
  surface0: '#414559', surface1: '#51576d', surface2: '#626880',
  text: '#c6d0f5', blue: '#8caaee', green: '#a6d189',
  yellow: '#e5c890', red: '#e78284',
};

// ── Catppuccin Latte palette (light mode) ─────────────────────────
const LATTE = {
  base: '#eff1f5', mantle: '#e6e9ef', crust: '#dce0e8',
  surface0: '#ccd0da', surface1: '#bcc0cc', surface2: '#acb0be',
  text: '#4c4f69', blue: '#1e66f5', green: '#40a02b',
  yellow: '#df8e1d', red: '#d20f39',
};

const STORAGE_KEY = 'theme-index';
const THEME_NAMES = ['mocha', 'frappe', 'latte'] as const;
type ThemeName = (typeof THEME_NAMES)[number];

function readIndex(): number {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw === null) return 0; // default Mocha
    const n = parseInt(raw, 10);
    if (n >= 0 && n <= 2) return n;
  } catch { /* ignore */ }
  return 0;
}

function applyDom(index: number) {
  try {
    const isDark = index < 2;
    document.body.setAttribute('class', isDark ? 'dark' : 'light');
    if (index === 1) {
      document.documentElement.setAttribute('data-theme', 'frappe');
    } else {
      document.documentElement.removeAttribute('data-theme');
    }
    const msg = document.getElementById('message');
    if (msg) msg.className = isDark ? 'dark' : 'light';
  } catch {
    // Ignore environments that block DOM or storage access during bootstrap.
  }
}

// module load so the document is in the right theme before React mounts.
const initialIndex = readIndex();
applyDom(initialIndex);

function paletteFor(index: number) {
  switch (index) {
    case 0: return MOCHA;
    case 1: return FRAPPE;
    default: return LATTE;
  }
}

function buildTokens(p: typeof MOCHA) {
  return {
    colorBgBase: p.mantle, colorBgLayout: p.mantle,
    colorBgContainer: p.surface0, colorBgElevated: p.surface1,
    colorPrimary: p.blue, colorSuccess: p.green,
    colorWarning: p.yellow, colorError: p.red, colorInfo: p.blue,
    colorTextBase: p.text, colorBorder: p.surface2,
  };
}

function buildLayoutTokens(p: typeof MOCHA) {
  return {
    bodyBg: p.mantle, headerBg: p.crust, headerColor: p.text,
    footerBg: p.mantle, siderBg: p.crust,
    triggerBg: p.surface0, triggerColor: p.text,
  };
}

function buildMenuTokens(p: typeof MOCHA) {
  return {
    darkItemBg: p.crust, darkSubMenuItemBg: p.mantle, darkPopupBg: p.surface0,
  };
}

function buildCardTokens(p: typeof MOCHA) {
  return { colorBorderSecondary: p.surface1 };
}

const STATISTIC_TOKENS = { contentFontSize: 17, titleFontSize: 11 };

export function buildAntdThemeConfig(index: number): ThemeConfig {
  const isDark = index < 2;
  const p = paletteFor(index);
  const algorithm = isDark ? antdTheme.darkAlgorithm : antdTheme.defaultAlgorithm;
  return {
    algorithm,
    token: buildTokens(p),
    components: {
      Layout: buildLayoutTokens(p),
      Menu: buildMenuTokens(p),
      Card: buildCardTokens(p),
      Statistic: STATISTIC_TOKENS,
    },
  };
}

export function pauseAnimationsUntilLeave(elementId: string): void {
  document.documentElement.setAttribute('data-theme-animations', 'off');
  const el = document.getElementById(elementId);
  if (!el) return;
  const restore = () => {
    document.documentElement.removeAttribute('data-theme-animations');
    el.removeEventListener('mouseleave', restore);
    el.removeEventListener('touchend', restore);
  };
  el.addEventListener('mouseleave', restore);
  el.addEventListener('touchend', restore);
}

export type { ThemeName };
export { THEME_NAMES };

interface ThemeContextValue {
  themeIndex: number;
  themeName: ThemeName;
  isDark: boolean;
  isUltra: boolean; // Frappé (soft dark) = ultra
  toggleTheme: () => void;
  toggleUltra: () => void; // cycles to next theme (same as toggleTheme in 3-mode)
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [themeIndex, setThemeIndex] = useState<number>(initialIndex);

  useEffect(() => {
    applyDom(themeIndex);
    try {
      localStorage.setItem(STORAGE_KEY, String(themeIndex));
    } catch {
      // Ignore storage failures so the app still renders.
    }
  }, [themeIndex]);

  const toggleTheme = useCallback(() => {
    setThemeIndex((prev) => (prev + 1) % 3);
  }, []);

  const isDark = themeIndex < 2;
  const isUltra = themeIndex === 1; // Frappé = the "ultra" dark mode
  const themeName = THEME_NAMES[themeIndex];

  const antdThemeConfig = useMemo(() => buildAntdThemeConfig(themeIndex), [themeIndex]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      themeIndex, themeName, isDark, isUltra,
      toggleTheme, toggleUltra: toggleTheme,
      antdThemeConfig,
    }),
    [themeIndex, themeName, isDark, isUltra, toggleTheme, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
