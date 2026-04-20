/**
 * 配置相关类型定义
 * 与基线 /config 返回结构保持一致（内部使用驼峰形式）
 */

import type { GeminiKeyConfig, ProviderKeyConfig, OpenAIProviderConfig } from './provider';
import type { AmpcodeConfig } from './ampcode';

export interface QuotaExceededConfig {
  switchProject?: boolean;
  switchPreviewModel?: boolean;
}

export interface ErrorCooldownsConfig {
  paymentRequiredSeconds?: number;
  notFoundSeconds?: number;
  modelNotSupportedSeconds?: number;
  transientErrorSeconds?: number;
  rateLimitBaseSeconds?: number;
  rateLimitMaxSeconds?: number;
}

export interface AuthUploadConfig {
  maxJsonSizeMb?: number;
  maxArchiveSizeMb?: number;
  maxArchiveEntries?: number;
  maxExpandedSizeMb?: number;
}

export interface Config {
  debug?: boolean;
  proxyUrl?: string;
  authUpload?: AuthUploadConfig;
  authProbeModels?: Record<string, string>;
  requestRetry?: number;
  maxRetryCredentials?: number;
  maxRetryInterval?: number;
  retryModelNotSupported?: boolean;
  retryThinkingValidationError?: boolean;
  errorCooldowns?: ErrorCooldownsConfig;
  quotaExceeded?: QuotaExceededConfig;
  usageStatisticsEnabled?: boolean;
  requestLog?: boolean;
  loggingToFile?: boolean;
  logsMaxTotalSizeMb?: number;
  wsAuth?: boolean;
  forceModelPrefix?: boolean;
  routingStrategy?: string;
  apiKeys?: string[];
  ampcode?: AmpcodeConfig;
  geminiApiKeys?: GeminiKeyConfig[];
  codexApiKeys?: ProviderKeyConfig[];
  claudeApiKeys?: ProviderKeyConfig[];
  vertexApiKeys?: ProviderKeyConfig[];
  openaiCompatibility?: OpenAIProviderConfig[];
  oauthExcludedModels?: Record<string, string[]>;
  raw?: Record<string, unknown>;
}

export type RawConfigSection =
  | 'debug'
  | 'proxy-url'
  | 'auth-upload'
  | 'auth-probe-models'
  | 'request-retry'
  | 'error-cooldowns'
  | 'quota-exceeded'
  | 'usage-statistics-enabled'
  | 'request-log'
  | 'logging-to-file'
  | 'logs-max-total-size-mb'
  | 'ws-auth'
  | 'force-model-prefix'
  | 'routing/strategy'
  | 'api-keys'
  | 'ampcode'
  | 'gemini-api-key'
  | 'codex-api-key'
  | 'claude-api-key'
  | 'vertex-api-key'
  | 'openai-compatibility'
  | 'oauth-excluded-models';

export interface ConfigCache {
  data: Config;
  timestamp: number;
}
