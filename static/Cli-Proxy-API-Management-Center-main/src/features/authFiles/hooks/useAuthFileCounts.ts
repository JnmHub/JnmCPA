import { useCallback, useEffect, useState } from 'react';
import { authFilesApi, type AuthFileCountResponse } from '@/services/api/authFiles';
import type { AuthFilesListQuery } from '@/types';

type UseAuthFileCountsOptions = {
  enabled?: boolean;
  query?: AuthFilesListQuery;
};

type UseAuthFileCountsResult = {
  counts: AuthFileCountResponse | null;
  loading: boolean;
  error: string;
  refresh: () => Promise<AuthFileCountResponse | null>;
};

export function useAuthFileCounts(options: UseAuthFileCountsOptions = {}): UseAuthFileCountsResult {
  const { enabled = true, query } = options;
  const [counts, setCounts] = useState<AuthFileCountResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const loadCounts = useCallback(async () => {
    if (!enabled) {
      return null;
    }
    setLoading(true);
    try {
      const payload = await authFilesApi.count(query);
      setCounts(payload);
      setError('');
      return payload;
    } catch (err) {
      const message =
        err instanceof Error && err.message ? err.message : 'Failed to load auth file counts';
      setError(message);
      return null;
    } finally {
      setLoading(false);
    }
  }, [enabled, query]);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    void loadCounts();
  }, [enabled, loadCounts]);

  return {
    counts,
    loading,
    error,
    refresh: loadCounts,
  };
}
