export type PayloadParamValueType = 'string' | 'number' | 'boolean' | 'json';
export type PayloadParamValidationErrorCode =
  | 'payload_invalid_number'
  | 'payload_invalid_boolean'
  | 'payload_invalid_json';

export type VisualConfigFieldPath =
  | 'port'
  | 'logsMaxTotalSizeMb'
  | 'authUpload.maxJsonSizeMb'
  | 'authUpload.maxArchiveSizeMb'
  | 'authUpload.maxArchiveEntries'
  | 'authUpload.maxExpandedSizeMb'
  | 'probeBatchConcurrency'
  | 'probeModelDefaults'
  | 'requestRetry'
  | 'maxRetryCredentials'
  | 'maxRetryInterval'
  | 'cooldowns.paymentRequiredSeconds'
  | 'cooldowns.notFoundSeconds'
  | 'cooldowns.modelNotSupportedSeconds'
  | 'cooldowns.transientErrorSeconds'
  | 'cooldowns.rateLimitBaseSeconds'
  | 'cooldowns.rateLimitMaxSeconds'
  | 'streaming.keepaliveSeconds'
  | 'streaming.bootstrapRetries'
  | 'streaming.nonstreamKeepaliveInterval';

export type VisualConfigValidationErrorCode = 'port_range' | 'non_negative_integer';

export type VisualConfigValidationErrors = Partial<
  Record<VisualConfigFieldPath, VisualConfigValidationErrorCode>
>;

export type PayloadParamEntry = {
  id: string;
  path: string;
  valueType: PayloadParamValueType;
  value: string;
};

export type PayloadModelEntry = {
  id: string;
  name: string;
  protocol?: string;
};

export type PayloadRule = {
  id: string;
  models: PayloadModelEntry[];
  params: PayloadParamEntry[];
};

export type PayloadFilterRule = {
  id: string;
  models: PayloadModelEntry[];
  params: string[];
};

export interface StreamingConfig {
  keepaliveSeconds: string;
  bootstrapRetries: string;
  nonstreamKeepaliveInterval: string;
}

export interface ErrorCooldownsVisualConfig {
  paymentRequiredSeconds: string;
  notFoundSeconds: string;
  modelNotSupportedSeconds: string;
  transientErrorSeconds: string;
  rateLimitBaseSeconds: string;
  rateLimitMaxSeconds: string;
}

export type VisualConfigValues = {
  host: string;
  port: string;
  tlsEnable: boolean;
  tlsCert: string;
  tlsKey: string;
  rmAllowRemote: boolean;
  rmSecretKey: string;
  rmOperatorSecretKey: string;
  rmPanelTitle: string;
  rmDisableControlPanel: boolean;
  rmPanelRepo: string;
  authDir: string;
  authUploadMaxJsonSizeMb: string;
  authUploadMaxArchiveSizeMb: string;
  authUploadMaxArchiveEntries: string;
  authUploadMaxExpandedSizeMb: string;
  apiKeysText: string;
  debug: boolean;
  commercialMode: boolean;
  loggingToFile: boolean;
  logsMaxTotalSizeMb: string;
  usageStatisticsEnabled: boolean;
  proxyUrl: string;
  probeBatchConcurrency: string;
  probeModelDefaults: string;
  autoDelete401: boolean;
  autoDelete429: boolean;
  retryModelNotSupported: boolean;
  forceModelPrefix: boolean;
  requestRetry: string;
  maxRetryCredentials: string;
  maxRetryInterval: string;
  cooldowns: ErrorCooldownsVisualConfig;
  quotaSwitchProject: boolean;
  quotaSwitchPreviewModel: boolean;
  routingStrategy: 'round-robin' | 'fill-first';
  wsAuth: boolean;
  payloadDefaultRules: PayloadRule[];
  payloadDefaultRawRules: PayloadRule[];
  payloadOverrideRules: PayloadRule[];
  payloadOverrideRawRules: PayloadRule[];
  payloadFilterRules: PayloadFilterRule[];
  streaming: StreamingConfig;
};

export const makeClientId = () => {
  if (typeof globalThis.crypto?.randomUUID === 'function') return globalThis.crypto.randomUUID();
  return `${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 10)}`;
};

export const DEFAULT_VISUAL_VALUES: VisualConfigValues = {
  host: '',
  port: '',
  tlsEnable: false,
  tlsCert: '',
  tlsKey: '',
  rmAllowRemote: false,
  rmSecretKey: '',
  rmOperatorSecretKey: '',
  rmPanelTitle: '',
  rmDisableControlPanel: false,
  rmPanelRepo: '',
  authDir: '',
  authUploadMaxJsonSizeMb: '10',
  authUploadMaxArchiveSizeMb: '100',
  authUploadMaxArchiveEntries: '10000',
  authUploadMaxExpandedSizeMb: '512',
  apiKeysText: '',
  debug: false,
  commercialMode: false,
  loggingToFile: false,
  logsMaxTotalSizeMb: '',
  usageStatisticsEnabled: false,
  proxyUrl: '',
  probeBatchConcurrency: '1',
  probeModelDefaults: '',
  autoDelete401: true,
  autoDelete429: true,
  retryModelNotSupported: true,
  forceModelPrefix: false,
  requestRetry: '',
  maxRetryCredentials: '',
  maxRetryInterval: '',
  cooldowns: {
    paymentRequiredSeconds: '',
    notFoundSeconds: '',
    modelNotSupportedSeconds: '',
    transientErrorSeconds: '',
    rateLimitBaseSeconds: '',
    rateLimitMaxSeconds: '',
  },
  quotaSwitchProject: true,
  quotaSwitchPreviewModel: true,
  routingStrategy: 'round-robin',
  wsAuth: false,
  payloadDefaultRules: [],
  payloadDefaultRawRules: [],
  payloadOverrideRules: [],
  payloadOverrideRawRules: [],
  payloadFilterRules: [],
  streaming: {
    keepaliveSeconds: '',
    bootstrapRetries: '',
    nonstreamKeepaliveInterval: '',
  },
};
