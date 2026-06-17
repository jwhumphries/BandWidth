import {useCallback, useEffect, useState} from 'react';

export type Theme = 'bandwidth' | 'bandwidth-light';

const KEY = 'bandwidth-theme';

function read(): Theme {
  const stored = localStorage.getItem(KEY);
  return stored === 'bandwidth-light' ? 'bandwidth-light' : 'bandwidth';
}

function apply(theme: Theme) {
  document.documentElement.dataset.theme = theme;
}

/** Persisted dark/light theme, defaulting to the dark studio theme. */
export function useTheme() {
  const [theme, setTheme] = useState<Theme>(read);

  useEffect(() => {
    apply(theme);
    localStorage.setItem(KEY, theme);
  }, [theme]);

  const toggle = useCallback(
    () => setTheme(t => (t === 'bandwidth' ? 'bandwidth-light' : 'bandwidth')),
    [],
  );

  return {theme, toggle};
}
