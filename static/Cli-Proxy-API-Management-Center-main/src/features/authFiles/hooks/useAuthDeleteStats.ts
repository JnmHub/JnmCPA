import { useCallback, useEffect, useRef, useState } from 'react';
import {
  authDeleteStatsApi,
  type AuthDeleteStatsRange,
  type AuthDeleteStatsResponse,
} from '@/services/api';

const RANGE_STORAGE_KEY = 'cli-proxy-auth-delete-stats-range-v1';
const DEFAULT_RANGE: AuthDeleteStatsRange = '24h';

const isAuthDeleteStatsRange = (value: unknown): value is AuthDeleteStatsRange =>
  value === '1h' || value === '24h' || value === '7d' || value === '30d';

const loadPersistedRange = (): AuthDeleteStatsRange => {
  try {
    if (typeof localStorage === 'undefined') {
      return DEFAULT_RANGE;
    }
    const raw = localStorage.getItem(RANGE_STORAGE_KEY);
    return isAuthDeleteStatsRange(raw) ? raw : DEFAULT_RANGE;
  } catch {
    return DEFAULT_RANGE;
  }
};

type UseAuthDeleteStatsOptions = {
  enabled?: boolean;
};

type UseAuthDeleteStatsResult = {
  data: AuthDeleteStatsResponse | null;
  range: AuthDeleteStatsRange;
  setRange: (value: AuthDeleteStatsRange) => void;
  loading: boolean;
  error: string;
  lastRefreshedAt: Date | null;
  refresh: () => Promise<AuthDeleteStatsResponse | null>;
};

export function useAuthDeleteStats(
  options: UseAuthDeleteStatsOptions = {}
): UseAuthDeleteStatsResult {
  const { enabled = true } = options;
  const [data, setData] = useState<AuthDeleteStatsResponse | null>(null);
  const [range, setRange] = useState<AuthDeleteStatsRange>(loadPersistedRange);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [lastRefreshedAt, setLastRefreshedAt] = useState<Date | null>(null);
  const requestIdRef = useRef(0);

  const loadStats = useCallback(
    async (force = false) => {
      if (!enabled) {
        return null;
      }
      const requestId = (requestIdRef.current += 1);
      setLoading(true);
      if (force) {
        setError('');
      }

      try {
        const payload = await authDeleteStatsApi.getStats({ range });
        if (requestId !== requestIdRef.current) {
          return null;
        }
        setData(payload);
        setError('');
        setLastRefreshedAt(new Date());
        return payload;
      } catch (err) {
        if (requestId !== requestIdRef.current) {
          return null;
        }
        const message =
          err instanceof Error && err.message ? err.message : 'Failed to load auth delete stats';
        setError(message);
        return null;
      } finally {
        if (requestId === requestIdRef.current) {
          setLoading(false);
        }
      }
    },
    [enabled, range]
  );

  useEffect(() => {
    if (!enabled) {
      return;
    }
    void loadStats();
  }, [enabled, loadStats]);

  useEffect(() => {
    try {
      if (typeof localStorage === 'undefined') {
        return;
      }
      localStorage.setItem(RANGE_STORAGE_KEY, range);
    } catch {
      // Ignore storage errors.
    }
  }, [range]);

  const refresh = useCallback(async () => loadStats(true), [loadStats]);

  return {
    data,
    range,
    setRange,
    loading,
    error,
    lastRefreshedAt,
    refresh,
  };
}
