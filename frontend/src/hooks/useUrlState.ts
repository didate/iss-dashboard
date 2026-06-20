import { useSearchParams } from 'react-router-dom';
import { useCallback } from 'react';

export function useUrlState(key: string, defaultValue = ''): [string, (v: string) => void] {
  const [params, setParams] = useSearchParams();
  const value = params.get(key) || defaultValue;
  const setValue = useCallback((v: string) => {
    // Read current URL directly to avoid stale closure issues
    const current = new URLSearchParams(window.location.search);
    if (v === '' || v === defaultValue) current.delete(key);
    else current.set(key, v);
    setParams(current, { replace: false });
  }, [key, defaultValue, setParams]);
  return [value, setValue];
}

export function useUrlStateInt(key: string, defaultValue = 1): [number, (v: number) => void] {
  const [str, setStr] = useUrlState(key, String(defaultValue));
  return [parseInt(str, 10) || defaultValue, (v: number) => setStr(String(v))];
}

export function useUrlStateList(key: string): [string[], (v: string[]) => void] {
  const [str, setStr] = useUrlState(key, '');
  const list = str ? str.split(',') : [];
  return [list, (v: string[]) => setStr(v.join(','))];
}
