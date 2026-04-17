import { apiClient } from './client';

const AUTH_DELETE_STATS_TIMEOUT_MS = 30 * 1000;

export type AuthDeleteStatsRange = '1h' | '24h' | '7d' | '30d';

export interface AuthDeleteStatsTotals {
  all: number;
  status_401: number;
  status_429: number;
}

export interface AuthDeleteStatsBucket {
  start_at: string;
  end_at: string;
  label: string;
  total: number;
  status_401: number;
  status_429: number;
}

export interface AuthDeleteStatsResponse {
  from: string;
  to: string;
  range: string;
  bucket: string;
  bucket_seconds: number;
  totals: AuthDeleteStatsTotals;
  series: AuthDeleteStatsBucket[];
  available_ranges?: string[];
  available_buckets?: string[];
}

export interface AuthDeleteStatsQuery {
  range?: AuthDeleteStatsRange;
  bucket?: string;
  from?: string;
  to?: string;
}

export const authDeleteStatsApi = {
  getStats: (params?: AuthDeleteStatsQuery) =>
    apiClient.get<AuthDeleteStatsResponse>('/auth-delete-stats', {
      params,
      timeout: AUTH_DELETE_STATS_TIMEOUT_MS,
    }),
};
