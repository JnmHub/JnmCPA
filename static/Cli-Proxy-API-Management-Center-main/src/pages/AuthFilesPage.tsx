import {
  useCallback,
  type CSSProperties,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
} from 'react';
import { createPortal } from 'react-dom';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { animate } from 'motion/mini';
import type { AnimationPlaybackControlsWithThen } from 'motion-dom';
import { useInterval } from '@/hooks/useInterval';
import { useDebounce } from '@/hooks/useDebounce';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { usePageTransitionLayer } from '@/components/common/PageTransitionLayer';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Select } from '@/components/ui/Select';
import { IconFilterAll } from '@/components/ui/icons';
import { EmptyState } from '@/components/ui/EmptyState';
import { ToggleSwitch } from '@/components/ui/ToggleSwitch';
import { copyToClipboard } from '@/utils/clipboard';
import { downloadBlob } from '@/utils/download';
import type { AuthFileItem } from '@/types';
import {
  MAX_CARD_PAGE_SIZE,
  MIN_CARD_PAGE_SIZE,
  QUOTA_PROVIDER_TYPES,
  clampCardPageSize,
  getAuthFileIcon,
  getTypeColor,
  getTypeLabel,
  isRuntimeOnlyAuthFile,
  normalizeProviderKey,
  type QuotaProviderType,
  type ResolvedTheme,
} from '@/features/authFiles/constants';
import { AuthDeleteStatsCard } from '@/features/authFiles/components/AuthDeleteStatsCard';
import { AuthFileCountCard } from '@/features/authFiles/components/AuthFileCountCard';
import { AuthFileCard } from '@/features/authFiles/components/AuthFileCard';
import { AuthFileQuotaModal } from '@/features/authFiles/components/AuthFileQuotaModal';
import { AuthFileWeightModal } from '@/features/authFiles/components/AuthFileWeightModal';
import { AuthFileModelsModal } from '@/features/authFiles/components/AuthFileModelsModal';
import { AuthFilesPrefixProxyEditorModal } from '@/features/authFiles/components/AuthFilesPrefixProxyEditorModal';
import { OAuthExcludedCard } from '@/features/authFiles/components/OAuthExcludedCard';
import { OAuthModelAliasCard } from '@/features/authFiles/components/OAuthModelAliasCard';
import { useAuthDeleteStats } from '@/features/authFiles/hooks/useAuthDeleteStats';
import { useAuthFileCounts } from '@/features/authFiles/hooks/useAuthFileCounts';
import { useAuthFilesData } from '@/features/authFiles/hooks/useAuthFilesData';
import { useAuthFilesModels } from '@/features/authFiles/hooks/useAuthFilesModels';
import { useAuthFilesOauth } from '@/features/authFiles/hooks/useAuthFilesOauth';
import { useAuthFilesPrefixProxyEditor } from '@/features/authFiles/hooks/useAuthFilesPrefixProxyEditor';
import { useAuthFilesStats } from '@/features/authFiles/hooks/useAuthFilesStats';
import { useAuthFilesStatusBarCache } from '@/features/authFiles/hooks/useAuthFilesStatusBarCache';
import {
  isAuthFilesSortMode,
  readAuthFilesUiState,
  readPersistedAuthFilesCompactMode,
  writeAuthFilesUiState,
  writePersistedAuthFilesCompactMode,
  type AuthFilesSortMode,
} from '@/features/authFiles/uiState';
import { useAuthStore, useNotificationStore, useThemeStore } from '@/stores';
import {
  authFilesApi,
  type AuthProbeBatchJob,
  type AuthProbeSingleResponse,
} from '@/services/api/authFiles';
import styles from './AuthFilesPage.module.scss';

const easePower3Out = (progress: number) => 1 - (1 - progress) ** 4;
const easePower2In = (progress: number) => progress ** 3;
const BATCH_BAR_BASE_TRANSFORM = 'translateX(-50%)';
const BATCH_BAR_HIDDEN_TRANSFORM = 'translateX(-50%) translateY(56px)';
const DEFAULT_REGULAR_PAGE_SIZE = 9;
const DEFAULT_COMPACT_PAGE_SIZE = 12;

type EnabledFilterMode = 'all' | 'enabled' | 'disabled';
type AvailabilityFilterMode = 'all' | 'usable' | 'cooling' | 'unusable';

export function AuthFilesPage() {
  const { t } = useTranslation();
  const showNotification = useNotificationStore((state) => state.showNotification);
  const showConfirmation = useNotificationStore((state) => state.showConfirmation);
  const connectionStatus = useAuthStore((state) => state.connectionStatus);
  const resolvedTheme: ResolvedTheme = useThemeStore((state) => state.resolvedTheme);
  const isMobile = useMediaQuery('(max-width: 768px)');
  const pageTransitionLayer = usePageTransitionLayer();
  const isCurrentLayer = pageTransitionLayer ? pageTransitionLayer.status === 'current' : true;
  const navigate = useNavigate();

  const [filter, setFilter] = useState<'all' | string>('all');
  const [problemOnly, setProblemOnly] = useState(false);
  const [compactMode, setCompactMode] = useState(false);
  const [search, setSearch] = useState('');
  const [enabledFilter, setEnabledFilter] = useState<EnabledFilterMode>('all');
  const [availabilityFilter, setAvailabilityFilter] = useState<AvailabilityFilterMode>('all');
  const [statusCodeFilter, setStatusCodeFilter] = useState('');
  const [page, setPage] = useState(1);
  const [pageSizeByMode, setPageSizeByMode] = useState({
    regular: DEFAULT_REGULAR_PAGE_SIZE,
    compact: DEFAULT_COMPACT_PAGE_SIZE,
  });
  const [pageSizeInput, setPageSizeInput] = useState('9');
  const [viewMode, setViewMode] = useState<'diagram' | 'list'>('list');
  const [sortMode, setSortMode] = useState<AuthFilesSortMode>('default');
  const [batchActionBarVisible, setBatchActionBarVisible] = useState(false);
  const [uiStateHydrated, setUiStateHydrated] = useState(false);
  const [probeBatchJob, setProbeBatchJob] = useState<AuthProbeBatchJob | null>(null);
  const [probeBatchLoading, setProbeBatchLoading] = useState(false);
  const [probeSingleLoading, setProbeSingleLoading] = useState<Record<string, boolean>>({});
  const [weightModalTarget, setWeightModalTarget] = useState<AuthFileItem | null>(null);
  const [weightSaving, setWeightSaving] = useState(false);
  const [quotaModalTarget, setQuotaModalTarget] = useState<{
    file: AuthFileItem;
    quotaType: QuotaProviderType;
  } | null>(null);
  const [resetRetryLoading, setResetRetryLoading] = useState(false);
  const [exportFilteredLoading, setExportFilteredLoading] = useState(false);
  const floatingBatchActionsRef = useRef<HTMLDivElement>(null);
  const batchActionAnimationRef = useRef<AnimationPlaybackControlsWithThen | null>(null);
  const previousSelectionCountRef = useRef(0);
  const selectionCountRef = useRef(0);

  const debouncedSearch = useDebounce(search, 350);
  const debouncedStatusCodeFilter = useDebounce(statusCodeFilter, 350);
  const pageSize = compactMode ? pageSizeByMode.compact : pageSizeByMode.regular;

  const listQuery = useMemo(
    () => ({
      page,
      pageSize,
      provider: filter !== 'all' ? filter : undefined,
      search: debouncedSearch.trim() || undefined,
      problemOnly,
      disabled: enabledFilter === 'enabled' ? false : enabledFilter === 'disabled' ? true : null,
      usable:
        availabilityFilter === 'usable' ? true : availabilityFilter === 'unusable' ? false : null,
      cooling: availabilityFilter === 'cooling' ? true : null,
      statusCode: debouncedStatusCodeFilter.trim() || undefined,
      sort: sortMode,
    }),
    [
      availabilityFilter,
      debouncedSearch,
      debouncedStatusCodeFilter,
      enabledFilter,
      filter,
      page,
      pageSize,
      problemOnly,
      sortMode,
    ]
  );

  const { keyStats, usageDetails, loadKeyStats, refreshKeyStats } = useAuthFilesStats();
  const {
    data: authDeleteStats,
    range: authDeleteStatsRange,
    setRange: setAuthDeleteStatsRange,
    loading: authDeleteStatsLoading,
    error: authDeleteStatsError,
    lastRefreshedAt: authDeleteStatsLastRefreshedAt,
    refresh: refreshAuthDeleteStats,
  } = useAuthDeleteStats({ enabled: isCurrentLayer });
  const {
    counts: authFileCounts,
    loading: authFileCountsLoading,
    error: authFileCountsError,
    refresh: refreshAuthFileCounts,
  } = useAuthFileCounts({
    enabled: isCurrentLayer,
    query: listQuery,
  });
  const {
    files,
    total,
    currentPage,
    totalPages,
    typeCounts,
    selectedFiles,
    selectionCount,
    loading,
    error,
    uploading,
    deleting,
    deletingAll,
    statusUpdating,
    batchStatusUpdating,
    fileInputRef,
    loadFiles,
    handleUploadClick,
    handleFileChange,
    handleDelete,
    handleDeleteAll,
    handleDownload,
    handleStatusToggle,
    toggleSelect,
    selectAllVisible,
    invertVisibleSelection,
    deselectAll,
    batchDownload,
    batchSetStatus,
    batchDelete,
  } = useAuthFilesData({ refreshKeyStats, listQuery });

  const statusBarCache = useAuthFilesStatusBarCache(files, usageDetails);

  const oauthProviderFiles = useMemo(
    () =>
      Object.keys(typeCounts)
        .filter((type) => type && type !== 'all')
        .map(
          (type) =>
            ({
              name: type,
              type,
              provider: type,
            }) satisfies AuthFileItem
        ),
    [typeCounts]
  );

  const {
    excluded,
    excludedError,
    modelAlias,
    modelAliasError,
    allProviderModels,
    loadExcluded,
    loadModelAlias,
    deleteExcluded,
    deleteModelAlias,
    handleMappingUpdate,
    handleDeleteLink,
    handleToggleFork,
    handleRenameAlias,
    handleDeleteAlias,
  } = useAuthFilesOauth({ viewMode, files: oauthProviderFiles });

  const {
    modelsModalOpen,
    modelsLoading,
    modelsList,
    modelsFileName,
    modelsFileType,
    modelsError,
    showModels,
    closeModelsModal,
  } = useAuthFilesModels();

  const {
    prefixProxyEditor,
    prefixProxyUpdatedText,
    prefixProxyDirty,
    openPrefixProxyEditor,
    closePrefixProxyEditor,
    handlePrefixProxyChange,
    handlePrefixProxySave,
  } = useAuthFilesPrefixProxyEditor({
    disableControls: connectionStatus !== 'connected',
    loadFiles,
    loadKeyStats: refreshKeyStats,
  });

  const disableControls = connectionStatus !== 'connected';
  const normalizedFilter = normalizeProviderKey(String(filter));
  const quotaFilterType: QuotaProviderType | null = QUOTA_PROVIDER_TYPES.has(
    normalizedFilter as QuotaProviderType
  )
    ? (normalizedFilter as QuotaProviderType)
    : null;

  useEffect(() => {
    const persistedCompactMode = readPersistedAuthFilesCompactMode();
    if (typeof persistedCompactMode === 'boolean') {
      setCompactMode(persistedCompactMode);
    }

    const persisted = readAuthFilesUiState();
    if (persisted) {
      if (typeof persisted.filter === 'string' && persisted.filter.trim()) {
        setFilter(persisted.filter);
      }
      if (typeof persisted.problemOnly === 'boolean') {
        setProblemOnly(persisted.problemOnly);
      }
      if (typeof persistedCompactMode !== 'boolean' && typeof persisted.compactMode === 'boolean') {
        setCompactMode(persisted.compactMode);
      }
      if (typeof persisted.search === 'string') {
        setSearch(persisted.search);
      }
      if (
        persisted.enabledFilter === 'all' ||
        persisted.enabledFilter === 'enabled' ||
        persisted.enabledFilter === 'disabled'
      ) {
        setEnabledFilter(persisted.enabledFilter);
      }
      if (
        persisted.availabilityFilter === 'all' ||
        persisted.availabilityFilter === 'usable' ||
        persisted.availabilityFilter === 'cooling' ||
        persisted.availabilityFilter === 'unusable'
      ) {
        setAvailabilityFilter(persisted.availabilityFilter);
      }
      if (typeof persisted.statusCodeFilter === 'string') {
        setStatusCodeFilter(persisted.statusCodeFilter);
      }
      if (typeof persisted.page === 'number' && Number.isFinite(persisted.page)) {
        setPage(Math.max(1, Math.round(persisted.page)));
      }
      const legacyPageSize =
        typeof persisted.pageSize === 'number' && Number.isFinite(persisted.pageSize)
          ? clampCardPageSize(persisted.pageSize)
          : null;
      const regularPageSize =
        typeof persisted.regularPageSize === 'number' && Number.isFinite(persisted.regularPageSize)
          ? clampCardPageSize(persisted.regularPageSize)
          : (legacyPageSize ?? DEFAULT_REGULAR_PAGE_SIZE);
      const compactPageSize =
        typeof persisted.compactPageSize === 'number' && Number.isFinite(persisted.compactPageSize)
          ? clampCardPageSize(persisted.compactPageSize)
          : (legacyPageSize ?? DEFAULT_COMPACT_PAGE_SIZE);
      setPageSizeByMode({
        regular: regularPageSize,
        compact: compactPageSize,
      });
      if (isAuthFilesSortMode(persisted.sortMode)) {
        setSortMode(persisted.sortMode);
      }
    }

    setUiStateHydrated(true);
  }, []);

  useEffect(() => {
    if (!uiStateHydrated) return;

    writeAuthFilesUiState({
      filter,
      problemOnly,
      compactMode,
      search,
      enabledFilter,
      availabilityFilter,
      statusCodeFilter,
      page,
      pageSize,
      regularPageSize: pageSizeByMode.regular,
      compactPageSize: pageSizeByMode.compact,
      sortMode,
    });
    writePersistedAuthFilesCompactMode(compactMode);
  }, [
    compactMode,
    filter,
    enabledFilter,
    availabilityFilter,
    page,
    pageSize,
    pageSizeByMode,
    problemOnly,
    search,
    statusCodeFilter,
    sortMode,
    uiStateHydrated,
  ]);

  useEffect(() => {
    setPageSizeInput(String(pageSize));
  }, [pageSize]);

  useEffect(() => {
    if (currentPage !== page) {
      setPage(currentPage);
    }
  }, [currentPage, page]);

  const setCurrentModePageSize = useCallback(
    (next: number) => {
      setPageSizeByMode((current) =>
        compactMode ? { ...current, compact: next } : { ...current, regular: next }
      );
    },
    [compactMode]
  );

  const commitPageSizeInput = (rawValue: string) => {
    const trimmed = rawValue.trim();
    if (!trimmed) {
      setPageSizeInput(String(pageSize));
      return;
    }

    const value = Number(trimmed);
    if (!Number.isFinite(value)) {
      setPageSizeInput(String(pageSize));
      return;
    }

    const next = clampCardPageSize(value);
    setCurrentModePageSize(next);
    setPageSizeInput(String(next));
    setPage(1);
  };

  const handlePageSizeChange = (event: ChangeEvent<HTMLInputElement>) => {
    const rawValue = event.currentTarget.value;
    setPageSizeInput(rawValue);

    const trimmed = rawValue.trim();
    if (!trimmed) return;

    const parsed = Number(trimmed);
    if (!Number.isFinite(parsed)) return;

    const rounded = Math.round(parsed);
    if (rounded < MIN_CARD_PAGE_SIZE || rounded > MAX_CARD_PAGE_SIZE) return;

    setCurrentModePageSize(rounded);
    setPage(1);
  };

  const handleSortModeChange = useCallback(
    (value: string) => {
      if (!isAuthFilesSortMode(value) || value === sortMode) return;
      setSortMode(value);
      setPage(1);
    },
    [sortMode]
  );

  const handleHeaderRefresh = useCallback(async () => {
    await Promise.all([
      loadFiles(),
      refreshAuthFileCounts(),
      refreshKeyStats(),
      loadExcluded(),
      loadModelAlias(),
      refreshAuthDeleteStats(),
    ]);
  }, [
    loadFiles,
    refreshAuthFileCounts,
    refreshKeyStats,
    loadExcluded,
    loadModelAlias,
    refreshAuthDeleteStats,
  ]);

  const handleProbeBatch = useCallback(async () => {
    setProbeBatchLoading(true);
    try {
      const payload = await authFilesApi.startProbeBatch(listQuery);
      const job = payload?.job ?? null;
      setProbeBatchJob(job);
      if (!job || !job.id) {
        setProbeBatchLoading(false);
        showNotification(t('auth_files.probe_batch_empty'), 'info');
        return;
      }
      showNotification(t('auth_files.probe_batch_started'), 'success');
    } catch (err) {
      setProbeBatchLoading(false);
      const message = err instanceof Error ? err.message : t('common.unknown_error');
      showNotification(t('auth_files.probe_batch_failed', { message }), 'error');
    }
  }, [listQuery, showNotification, t]);

  const handleResetAllRetryTimes = useCallback(() => {
    showConfirmation({
      title: t('auth_files.reset_retry_filtered_title'),
      message: t('auth_files.reset_retry_filtered_confirm', { total }),
      variant: 'primary',
      confirmText: t('common.confirm'),
      onConfirm: async () => {
        setResetRetryLoading(true);
        try {
          const payload = await authFilesApi.resetAllRetryTimes(listQuery);
          await Promise.allSettled([
            loadFiles(),
            refreshAuthFileCounts(),
            refreshAuthDeleteStats(),
            refreshKeyStats(),
          ]);
          showNotification(
            t('auth_files.reset_retry_filtered_success', {
              reset: payload?.reset ?? 0,
              total: payload?.total ?? 0,
            }),
            'success'
          );
        } catch (err) {
          const message = err instanceof Error ? err.message : t('common.unknown_error');
          showNotification(t('auth_files.reset_retry_filtered_failed', { message }), 'error');
        } finally {
          setResetRetryLoading(false);
        }
      },
    });
  }, [
    loadFiles,
      refreshAuthDeleteStats,
      refreshAuthFileCounts,
      refreshKeyStats,
      listQuery,
      showConfirmation,
      showNotification,
      total,
      t,
    ]);

  const handleExportFiltered = useCallback(async () => {
    setExportFilteredLoading(true);
    try {
      const response = await authFilesApi.exportByQuery(listQuery);
      const blob = new Blob([response.data], { type: 'application/zip' });
      const filename = `auth-files-export-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.zip`;
      downloadBlob({ filename, blob });
      showNotification(t('auth_files.export_filtered_success', { total }), 'success');
    } catch (err) {
      const message = err instanceof Error ? err.message : t('common.unknown_error');
      showNotification(t('auth_files.export_filtered_failed', { message }), 'error');
    } finally {
      setExportFilteredLoading(false);
    }
  }, [listQuery, showNotification, t, total]);

  const formatProbeModelSource = useCallback(
    (source: AuthProbeSingleResponse['probe_model_source']) => {
      const normalized = String(source ?? '')
        .trim()
        .toLowerCase();
      if (!normalized) {
        return t('auth_files.probe_model_source_unknown');
      }
      return t(`auth_files.probe_model_source_${normalized}`, {
        defaultValue: t('auth_files.probe_model_source_unknown'),
      });
    },
    [t]
  );

  const compactFileLabel = useCallback((value: string, maxLength = 32) => {
    const trimmed = String(value ?? '').trim();
    if (trimmed.length <= maxLength) return trimmed;
    const keep = Math.max(8, Math.floor((maxLength - 1) / 2));
    return `${trimmed.slice(0, keep)}…${trimmed.slice(-keep)}`;
  }, []);

  const handleProbeSingle = useCallback(
    async (file: AuthFileItem) => {
      const name = String(file.name ?? '').trim();
      if (!name) return;

      setProbeSingleLoading((current) => ({ ...current, [name]: true }));
      try {
        const payload = await authFilesApi.probeOne(name);
        const statusCode =
          typeof payload?.status_code === 'number' && Number.isFinite(payload.status_code)
            ? String(payload.status_code)
            : t('auth_files.status_meta_empty');
        const model = String(payload?.probe_model ?? '').trim() || '-';
        const sourceLabel = formatProbeModelSource(payload?.probe_model_source);
        const detail =
          String(payload?.error_message ?? '').trim() ||
          String(payload?.error_code ?? '').trim() ||
          t('auth_files.health_status_no_message');

        if (payload?.deleted) {
          showNotification(
            t('auth_files.probe_single_deleted', {
              name: compactFileLabel(name),
              statusCode,
              model,
              source: sourceLabel,
            }),
            'success'
          );
        } else if (payload?.success) {
          showNotification(
            t('auth_files.probe_single_success', {
              name: compactFileLabel(name),
              statusCode,
              model,
              source: sourceLabel,
            }),
            'success'
          );
        } else {
          showNotification(
            t('auth_files.probe_single_failed', {
              name: compactFileLabel(name),
              statusCode,
              model,
              source: sourceLabel,
              message: detail,
            }),
            'error'
          );
        }

        await Promise.allSettled([
          loadFiles(),
          refreshAuthFileCounts(),
          refreshAuthDeleteStats(),
          refreshKeyStats(),
        ]);
      } catch (err) {
        const message = err instanceof Error ? err.message : t('common.unknown_error');
        showNotification(t('auth_files.probe_single_request_failed', { name, message }), 'error');
      } finally {
        setProbeSingleLoading((current) => {
          const next = { ...current };
          delete next[name];
          return next;
        });
      }
    },
    [
      compactFileLabel,
      formatProbeModelSource,
      loadFiles,
      refreshAuthDeleteStats,
      refreshAuthFileCounts,
      refreshKeyStats,
      showNotification,
      t,
    ]
  );

  const handleShowQuota = useCallback((file: AuthFileItem, quotaType: QuotaProviderType) => {
    setQuotaModalTarget({ file, quotaType });
  }, []);

  const handleEditWeight = useCallback((file: AuthFileItem) => {
    setWeightModalTarget(file);
  }, []);

  const handleSaveWeight = useCallback(
    async (priority: number | null) => {
      if (!weightModalTarget) return;
      setWeightSaving(true);
      try {
        await authFilesApi.patchFields({
          name: weightModalTarget.name,
          priority: priority ?? 0,
        });
        await Promise.allSettled([loadFiles(), refreshAuthFileCounts(), refreshKeyStats()]);
        showNotification(
          t('auth_files.weight_saved_success', {
            name: weightModalTarget.name,
            weight: priority ?? 0,
          }),
          'success'
        );
        setWeightModalTarget(null);
      } catch (err) {
        const message = err instanceof Error ? err.message : t('common.unknown_error');
        showNotification(t('auth_files.weight_saved_failed', { message }), 'error');
      } finally {
        setWeightSaving(false);
      }
    },
    [loadFiles, refreshAuthFileCounts, refreshKeyStats, showNotification, t, weightModalTarget]
  );

  useHeaderRefresh(handleHeaderRefresh);

  useEffect(() => {
    if (!isCurrentLayer) return;
    void loadFiles();
  }, [isCurrentLayer, loadFiles]);

  useEffect(() => {
    if (!isCurrentLayer) return;
    void loadKeyStats().catch(() => {});
    loadExcluded();
    loadModelAlias();
  }, [isCurrentLayer, loadKeyStats, loadExcluded, loadModelAlias]);

  useInterval(
    () => {
      void refreshKeyStats().catch(() => {});
    },
    isCurrentLayer ? 240_000 : null
  );

  useInterval(
    () => {
      void refreshAuthDeleteStats().catch(() => {});
    },
    isCurrentLayer ? 60_000 : null
  );

  useInterval(
    () => {
      if (!probeBatchJob?.id) return;
      void authFilesApi
        .getProbeBatch(probeBatchJob.id)
        .then(async (payload) => {
          const nextJob = payload?.job ?? null;
          if (!nextJob) return;
          setProbeBatchJob(nextJob);
          if (nextJob.status === 'completed' || nextJob.status === 'failed') {
            setProbeBatchLoading(false);
            await Promise.allSettled([
              loadFiles(),
              refreshAuthFileCounts(),
              refreshAuthDeleteStats(),
              refreshKeyStats(),
            ]);
            if (nextJob.status === 'completed') {
              showNotification(
                t('auth_files.probe_batch_completed', {
                  total: nextJob.total ?? 0,
                  deleted: nextJob.deleted ?? 0,
                  succeeded: nextJob.succeeded ?? 0,
                  failed: nextJob.failed ?? 0,
                }),
                'success'
              );
            } else if (nextJob.last_error) {
              showNotification(
                t('auth_files.probe_batch_failed', { message: nextJob.last_error }),
                'error'
              );
            }
          }
        })
        .catch(() => {
          setProbeBatchLoading(false);
        });
    },
    isCurrentLayer &&
      probeBatchJob &&
      probeBatchJob.status !== 'completed' &&
      probeBatchJob.status !== 'failed'
      ? 2_000
      : null
  );

  const existingTypes = useMemo(() => {
    const types = new Set<string>(['all']);
    Object.keys(typeCounts).forEach((type) => {
      if (type && type !== 'all') {
        types.add(type);
      }
    });
    if (filter !== 'all') {
      types.add(filter);
    }
    return Array.from(types);
  }, [filter, typeCounts]);

  const sortOptions = useMemo(
    () => [
      { value: 'default', label: t('auth_files.sort_default') },
      { value: 'az', label: t('auth_files.sort_az') },
      { value: 'priority', label: t('auth_files.sort_priority') },
    ],
    [t]
  );

  const enabledFilterOptions = useMemo(
    () => [
      { value: 'all', label: t('auth_files.enabled_filter_all') },
      { value: 'enabled', label: t('auth_files.enabled_filter_enabled') },
      { value: 'disabled', label: t('auth_files.enabled_filter_disabled') },
    ],
    [t]
  );

  const availabilityFilterOptions = useMemo(
    () => [
      { value: 'all', label: t('auth_files.availability_filter_all') },
      { value: 'usable', label: t('auth_files.availability_filter_usable') },
      { value: 'cooling', label: t('auth_files.availability_filter_cooling') },
      { value: 'unusable', label: t('auth_files.availability_filter_unusable') },
    ],
    [t]
  );

  const pageItems = files;
  const selectablePageItems = useMemo(
    () => pageItems.filter((file) => !isRuntimeOnlyAuthFile(file)),
    [pageItems]
  );
  const selectedNames = useMemo(() => Array.from(selectedFiles), [selectedFiles]);
  const selectedHasStatusUpdating = useMemo(
    () => selectedNames.some((name) => statusUpdating[name] === true),
    [selectedNames, statusUpdating]
  );
  const batchStatusButtonsDisabled =
    disableControls ||
    selectedNames.length === 0 ||
    batchStatusUpdating ||
    selectedHasStatusUpdating;

  const copyTextWithNotification = useCallback(
    async (text: string) => {
      const copied = await copyToClipboard(text);
      showNotification(
        copied
          ? t('notification.link_copied', { defaultValue: 'Copied to clipboard' })
          : t('notification.copy_failed', { defaultValue: 'Copy failed' }),
        copied ? 'success' : 'error'
      );
    },
    [showNotification, t]
  );

  const openExcludedEditor = useCallback(
    (provider?: string) => {
      const providerValue = (provider || (filter !== 'all' ? String(filter) : '')).trim();
      const params = new URLSearchParams();
      if (providerValue) {
        params.set('provider', providerValue);
      }
      const nextSearch = params.toString();
      navigate(`/auth-files/oauth-excluded${nextSearch ? `?${nextSearch}` : ''}`, {
        state: { fromAuthFiles: true },
      });
    },
    [filter, navigate]
  );

  const openModelAliasEditor = useCallback(
    (provider?: string) => {
      const providerValue = (provider || (filter !== 'all' ? String(filter) : '')).trim();
      const params = new URLSearchParams();
      if (providerValue) {
        params.set('provider', providerValue);
      }
      const nextSearch = params.toString();
      navigate(`/auth-files/oauth-model-alias${nextSearch ? `?${nextSearch}` : ''}`, {
        state: { fromAuthFiles: true },
      });
    },
    [filter, navigate]
  );

  useLayoutEffect(() => {
    if (typeof window === 'undefined') return;

    const actionsEl = floatingBatchActionsRef.current;
    if (!actionsEl) {
      document.documentElement.style.removeProperty('--auth-files-action-bar-height');
      return;
    }

    const updatePadding = () => {
      const height = actionsEl.getBoundingClientRect().height;
      document.documentElement.style.setProperty('--auth-files-action-bar-height', `${height}px`);
    };

    updatePadding();
    window.addEventListener('resize', updatePadding);

    const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(updatePadding);
    ro?.observe(actionsEl);

    return () => {
      ro?.disconnect();
      window.removeEventListener('resize', updatePadding);
      document.documentElement.style.removeProperty('--auth-files-action-bar-height');
    };
  }, [batchActionBarVisible, selectionCount]);

  useEffect(() => {
    selectionCountRef.current = selectionCount;
    if (selectionCount > 0) {
      setBatchActionBarVisible(true);
    }
  }, [selectionCount]);

  useLayoutEffect(() => {
    if (!batchActionBarVisible) return;
    const currentCount = selectionCount;
    const previousCount = previousSelectionCountRef.current;
    const actionsEl = floatingBatchActionsRef.current;
    if (!actionsEl) return;

    batchActionAnimationRef.current?.stop();
    batchActionAnimationRef.current = null;

    if (currentCount > 0 && previousCount === 0) {
      batchActionAnimationRef.current = animate(
        actionsEl,
        {
          transform: [BATCH_BAR_HIDDEN_TRANSFORM, BATCH_BAR_BASE_TRANSFORM],
          opacity: [0, 1],
        },
        {
          duration: 0.28,
          ease: easePower3Out,
          onComplete: () => {
            actionsEl.style.transform = BATCH_BAR_BASE_TRANSFORM;
            actionsEl.style.opacity = '1';
          },
        }
      );
    } else if (currentCount === 0 && previousCount > 0) {
      batchActionAnimationRef.current = animate(
        actionsEl,
        {
          transform: [BATCH_BAR_BASE_TRANSFORM, BATCH_BAR_HIDDEN_TRANSFORM],
          opacity: [1, 0],
        },
        {
          duration: 0.22,
          ease: easePower2In,
          onComplete: () => {
            if (selectionCountRef.current === 0) {
              setBatchActionBarVisible(false);
            }
          },
        }
      );
    }

    previousSelectionCountRef.current = currentCount;
  }, [batchActionBarVisible, selectionCount]);

  useEffect(
    () => () => {
      batchActionAnimationRef.current?.stop();
      batchActionAnimationRef.current = null;
    },
    []
  );

  const renderFilterTags = () => (
    <div className={styles.filterRail}>
      <div className={styles.filterTags}>
        {existingTypes.map((type) => {
          const isActive = filter === type;
          const iconSrc = getAuthFileIcon(type, resolvedTheme);
          const color =
            type === 'all'
              ? { bg: 'var(--bg-tertiary)', text: 'var(--text-primary)' }
              : getTypeColor(type, resolvedTheme);
          const buttonStyle = {
            '--filter-color': color.text,
            '--filter-surface': color.bg,
            '--filter-active-text': resolvedTheme === 'dark' ? '#111827' : '#ffffff',
          } as CSSProperties;

          return (
            <button
              key={type}
              className={`${styles.filterTag} ${isActive ? styles.filterTagActive : ''}`}
              style={buttonStyle}
              onClick={() => {
                setFilter(type);
                setPage(1);
              }}
            >
              <span className={styles.filterTagLabel}>
                {type === 'all' ? (
                  <span className={`${styles.filterTagIconWrap} ${styles.filterAllIconWrap}`}>
                    <IconFilterAll className={styles.filterAllIcon} size={16} />
                  </span>
                ) : (
                  <span className={styles.filterTagIconWrap}>
                    {iconSrc ? (
                      <img src={iconSrc} alt="" className={styles.filterTagIcon} />
                    ) : (
                      <span className={styles.filterTagIconFallback}>
                        {getTypeLabel(t, type).slice(0, 1).toUpperCase()}
                      </span>
                    )}
                  </span>
                )}
                <span className={styles.filterTagText}>{getTypeLabel(t, type)}</span>
              </span>
              <span className={styles.filterTagCount}>{typeCounts[type] ?? 0}</span>
            </button>
          );
        })}
      </div>
    </div>
  );

  const titleNode = (
    <div className={styles.titleWrapper}>
      <span>{t('auth_files.title_section')}</span>
      {total > 0 && <span className={styles.countBadge}>{total}</span>}
    </div>
  );

  const deleteAllButtonLabel = problemOnly
    ? filter === 'all'
      ? t('auth_files.delete_problem_button')
      : t('auth_files.delete_problem_button_with_type', { type: getTypeLabel(t, filter) })
    : filter === 'all'
      ? t('auth_files.delete_all_button')
      : `${t('common.delete')} ${getTypeLabel(t, filter)}`;
  const probeBatchLabel =
    probeBatchLoading && probeBatchJob
      ? t('auth_files.probe_batch_progress', {
          processed: probeBatchJob.processed ?? 0,
          total: probeBatchJob.total ?? 0,
        })
      : t('auth_files.probe_batch_button');

  return (
    <div className={styles.container}>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>{t('auth_files.title')}</h1>
        <p className={styles.description}>{t('auth_files.description')}</p>
      </div>

      <AuthDeleteStatsCard
        data={authDeleteStats}
        range={authDeleteStatsRange}
        onRangeChange={setAuthDeleteStatsRange}
        loading={authDeleteStatsLoading}
        error={authDeleteStatsError}
        onRefresh={refreshAuthDeleteStats}
        isDark={resolvedTheme === 'dark'}
        isMobile={isMobile}
        lastRefreshedAt={authDeleteStatsLastRefreshedAt}
      />

      <AuthFileCountCard
        counts={authFileCounts}
        loading={authFileCountsLoading}
        error={authFileCountsError}
        onRefresh={refreshAuthFileCounts}
      />

      <Card
        title={titleNode}
        extra={
          <div className={styles.headerActions}>
            <Button variant="secondary" size="sm" onClick={handleHeaderRefresh} disabled={loading}>
              {t('common.refresh')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => void handleProbeBatch()}
              disabled={disableControls || probeBatchLoading}
              loading={probeBatchLoading}
              title={
                probeBatchJob?.current_file
                  ? t('auth_files.probe_batch_current', { name: probeBatchJob.current_file })
                  : undefined
              }
            >
              {probeBatchLabel}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleResetAllRetryTimes}
              disabled={disableControls || resetRetryLoading || total === 0}
              loading={resetRetryLoading}
            >
              {t('auth_files.reset_retry_filtered_button')}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => void handleExportFiltered()}
              disabled={disableControls || exportFilteredLoading || total === 0}
              loading={exportFilteredLoading}
            >
              {t('auth_files.export_filtered_button')}
            </Button>
            <Button
              size="sm"
              onClick={handleUploadClick}
              disabled={disableControls || uploading}
              loading={uploading}
            >
              {t('auth_files.upload_button')}
            </Button>
            <Button
              variant="danger"
              size="sm"
              onClick={() =>
                handleDeleteAll({
                  filter,
                  problemOnly,
                  query: listQuery,
                  total,
                })
              }
              disabled={disableControls || loading || deletingAll || total === 0}
              loading={deletingAll}
            >
              {deleteAllButtonLabel}
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".json,.zip,application/json,application/zip"
              multiple
              style={{ display: 'none' }}
              onChange={handleFileChange}
            />
          </div>
        }
      >
        {error && <div className={styles.errorBox}>{error}</div>}

        <div className={styles.filterSection}>
          {renderFilterTags()}

          <div className={styles.filterContent}>
            <div className={styles.filterControlsPanel}>
              <div className={styles.filterControls}>
                <div className={styles.filterItem}>
                  <label>{t('auth_files.search_label')}</label>
                  <Input
                    value={search}
                    onChange={(e) => {
                      setSearch(e.target.value);
                      setPage(1);
                    }}
                    placeholder={t('auth_files.search_placeholder')}
                  />
                </div>
                <div className={styles.filterItem}>
                  <label>{t('auth_files.status_code_filter_label')}</label>
                  <Input
                    value={statusCodeFilter}
                    onChange={(e) => {
                      setStatusCodeFilter(e.target.value);
                      setPage(1);
                    }}
                    placeholder={t('auth_files.status_code_filter_placeholder')}
                  />
                </div>
                <div className={styles.filterItem}>
                  <label>{t('auth_files.page_size_label')}</label>
                  <input
                    className={styles.pageSizeSelect}
                    type="number"
                    min={MIN_CARD_PAGE_SIZE}
                    max={MAX_CARD_PAGE_SIZE}
                    step={1}
                    value={pageSizeInput}
                    onChange={handlePageSizeChange}
                    onBlur={(e) => commitPageSizeInput(e.currentTarget.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.currentTarget.blur();
                      }
                    }}
                  />
                </div>
                <div className={styles.filterItem}>
                  <label>{t('auth_files.sort_label')}</label>
                  <Select
                    className={styles.sortSelect}
                    value={sortMode}
                    options={sortOptions}
                    onChange={handleSortModeChange}
                    ariaLabel={t('auth_files.sort_label')}
                    fullWidth
                  />
                </div>
                <div className={styles.filterItem}>
                  <label>{t('auth_files.enabled_filter_label')}</label>
                  <Select
                    className={styles.sortSelect}
                    value={enabledFilter}
                    options={enabledFilterOptions}
                    onChange={(value) => {
                      if (value === 'all' || value === 'enabled' || value === 'disabled') {
                        setEnabledFilter(value);
                        setPage(1);
                      }
                    }}
                    ariaLabel={t('auth_files.enabled_filter_label')}
                    fullWidth
                  />
                </div>
                <div className={styles.filterItem}>
                  <label>{t('auth_files.availability_filter_label')}</label>
                  <Select
                    className={styles.sortSelect}
                    value={availabilityFilter}
                    options={availabilityFilterOptions}
                    onChange={(value) => {
                      if (
                        value === 'all' ||
                        value === 'usable' ||
                        value === 'cooling' ||
                        value === 'unusable'
                      ) {
                        setAvailabilityFilter(value);
                        setPage(1);
                      }
                    }}
                    ariaLabel={t('auth_files.availability_filter_label')}
                    fullWidth
                  />
                </div>
                <div className={`${styles.filterItem} ${styles.filterToggleItem}`}>
                  <label>{t('auth_files.display_options_label')}</label>
                  <div className={styles.filterToggleGroup}>
                    <div className={styles.filterToggleCard}>
                      <ToggleSwitch
                        checked={problemOnly}
                        onChange={(value) => {
                          setProblemOnly(value);
                          setPage(1);
                        }}
                        ariaLabel={t('auth_files.problem_filter_only')}
                        label={
                          <span className={styles.filterToggleLabel}>
                            {t('auth_files.problem_filter_only')}
                          </span>
                        }
                      />
                    </div>
                    <div className={styles.filterToggleCard}>
                      <ToggleSwitch
                        checked={compactMode}
                        onChange={(value) => setCompactMode(value)}
                        ariaLabel={t('auth_files.compact_mode_label')}
                        label={
                          <span className={styles.filterToggleLabel}>
                            {t('auth_files.compact_mode_label')}
                          </span>
                        }
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {loading ? (
              <div className={styles.hint}>{t('common.loading')}</div>
            ) : pageItems.length === 0 ? (
              <EmptyState
                title={t('auth_files.search_empty_title')}
                description={t('auth_files.search_empty_desc')}
              />
            ) : (
              <div
                className={`${styles.fileGrid} ${quotaFilterType ? styles.fileGridQuotaManaged : ''} ${compactMode ? styles.fileGridCompact : ''}`}
              >
                {pageItems.map((file) => (
                  <AuthFileCard
                    key={file.name}
                    file={file}
                    compact={compactMode}
                    selected={selectedFiles.has(file.name)}
                    resolvedTheme={resolvedTheme}
                    disableControls={disableControls}
                    deleting={deleting}
                    probeLoading={probeSingleLoading[file.name] === true}
                    statusUpdating={statusUpdating}
                    quotaFilterType={quotaFilterType}
                    keyStats={keyStats}
                    statusBarCache={statusBarCache}
                    onShowModels={showModels}
                    onEditWeight={handleEditWeight}
                    onShowQuota={handleShowQuota}
                    onProbe={handleProbeSingle}
                    onDownload={handleDownload}
                    onOpenPrefixProxyEditor={openPrefixProxyEditor}
                    onDelete={handleDelete}
                    onToggleStatus={handleStatusToggle}
                    onToggleSelect={toggleSelect}
                  />
                ))}
              </div>
            )}

            {!loading && totalPages > 1 && (
              <div className={styles.pagination}>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setPage(Math.max(1, currentPage - 1))}
                  disabled={currentPage <= 1}
                >
                  {t('auth_files.pagination_prev')}
                </Button>
                <div className={styles.pageInfo}>
                  {t('auth_files.pagination_info', {
                    current: currentPage,
                    total: totalPages,
                    count: total,
                  })}
                </div>
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => setPage(Math.min(totalPages, currentPage + 1))}
                  disabled={currentPage >= totalPages}
                >
                  {t('auth_files.pagination_next')}
                </Button>
              </div>
            )}
          </div>
        </div>
      </Card>

      <OAuthExcludedCard
        disableControls={disableControls}
        excludedError={excludedError}
        excluded={excluded}
        onAdd={() => openExcludedEditor()}
        onEdit={openExcludedEditor}
        onDelete={deleteExcluded}
      />

      <OAuthModelAliasCard
        disableControls={disableControls}
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        onAdd={() => openModelAliasEditor()}
        onEditProvider={openModelAliasEditor}
        onDeleteProvider={deleteModelAlias}
        modelAliasError={modelAliasError}
        modelAlias={modelAlias}
        allProviderModels={allProviderModels}
        onUpdate={handleMappingUpdate}
        onDeleteLink={handleDeleteLink}
        onToggleFork={handleToggleFork}
        onRenameAlias={handleRenameAlias}
        onDeleteAlias={handleDeleteAlias}
      />

      <AuthFileModelsModal
        open={modelsModalOpen}
        fileName={modelsFileName}
        fileType={modelsFileType}
        loading={modelsLoading}
        error={modelsError}
        models={modelsList}
        excluded={excluded}
        onClose={closeModelsModal}
        onCopyText={copyTextWithNotification}
      />

      <AuthFileQuotaModal
        open={quotaModalTarget !== null}
        file={quotaModalTarget?.file ?? null}
        quotaType={quotaModalTarget?.quotaType ?? null}
        disableControls={disableControls}
        onClose={() => setQuotaModalTarget(null)}
      />

      <AuthFileWeightModal
        open={weightModalTarget !== null}
        file={weightModalTarget}
        saving={weightSaving}
        disableControls={disableControls}
        onClose={() => setWeightModalTarget(null)}
        onSave={handleSaveWeight}
      />

      <AuthFilesPrefixProxyEditorModal
        disableControls={disableControls}
        editor={prefixProxyEditor}
        updatedText={prefixProxyUpdatedText}
        dirty={prefixProxyDirty}
        onClose={closePrefixProxyEditor}
        onCopyText={copyTextWithNotification}
        onSave={handlePrefixProxySave}
        onChange={handlePrefixProxyChange}
      />

      {batchActionBarVisible && typeof document !== 'undefined'
        ? createPortal(
            <div className={styles.batchActionContainer} ref={floatingBatchActionsRef}>
              <div className={styles.batchActionBar}>
                <div className={styles.batchActionLeft}>
                  <span className={styles.batchSelectionText}>
                    {t('auth_files.batch_selected', { count: selectionCount })}
                  </span>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => selectAllVisible(pageItems)}
                    disabled={selectablePageItems.length === 0}
                  >
                    {t('auth_files.batch_select_page')}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => invertVisibleSelection(pageItems)}
                    disabled={selectablePageItems.length === 0}
                  >
                    {t('auth_files.batch_invert_page')}
                  </Button>
                  <Button variant="ghost" size="sm" onClick={deselectAll}>
                    {t('auth_files.batch_deselect')}
                  </Button>
                </div>
                <div className={styles.batchActionRight}>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => void batchDownload(selectedNames)}
                    disabled={disableControls || selectedNames.length === 0}
                  >
                    {t('auth_files.batch_download')}
                  </Button>
                  <Button
                    size="sm"
                    onClick={() => batchSetStatus(selectedNames, true)}
                    disabled={batchStatusButtonsDisabled}
                  >
                    {t('auth_files.batch_enable')}
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => batchSetStatus(selectedNames, false)}
                    disabled={batchStatusButtonsDisabled}
                  >
                    {t('auth_files.batch_disable')}
                  </Button>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => batchDelete(selectedNames)}
                    disabled={disableControls || selectedNames.length === 0}
                  >
                    {t('common.delete')}
                  </Button>
                </div>
              </div>
            </div>,
            document.body
          )
        : null}
    </div>
  );
}
