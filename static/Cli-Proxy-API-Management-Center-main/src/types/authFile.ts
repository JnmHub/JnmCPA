/**
 * 认证文件相关类型
 * 基于原项目 src/modules/auth-files.js
 */

export type AuthFileType =
  | 'qwen'
  | 'kimi'
  | 'gemini'
  | 'gemini-cli'
  | 'aistudio'
  | 'claude'
  | 'codex'
  | 'antigravity'
  | 'iflow'
  | 'vertex'
  | 'empty'
  | 'unknown';

export interface AuthFileItem {
  name: string;
  type?: AuthFileType | string;
  provider?: string;
  size?: number;
  authIndex?: string | number | null;
  runtimeOnly?: boolean | string;
  disabled?: boolean;
  unavailable?: boolean;
  status?: string;
  statusMessage?: string;
  statusCode?: number | string | null;
  errorCode?: string | null;
  errorMessage?: string | null;
  lastRefresh?: string | number;
  nextRetryAfter?: string | number | null;
  usable?: boolean | string | number | null;
  cooling?: boolean | string | number | null;
  lastResult?: Record<string, unknown> | null;
  lastError?: Record<string, unknown> | null;
  modified?: number;
  [key: string]: unknown;
}

export type AuthFilesListQuery = {
  page?: number;
  pageSize?: number;
  provider?: string;
  search?: string;
  problemOnly?: boolean;
  disabled?: boolean | null;
  usable?: boolean | null;
  cooling?: boolean | null;
  statusCode?: string;
  sort?: string;
};

export interface AuthFilesResponse {
  files: AuthFileItem[];
  total?: number;
  page?: number;
  pageSize?: number;
  totalPages?: number;
  typeCounts?: Record<string, number>;
}
