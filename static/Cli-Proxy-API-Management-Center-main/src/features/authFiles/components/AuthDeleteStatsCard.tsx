import { useCallback, useMemo, useState } from 'react';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
  type ChartOptions,
} from 'chart.js';
import { Line } from 'react-chartjs-2';
import { useTranslation } from 'react-i18next';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import { buildChartOptions, getHourChartMinWidth } from '@/utils/usage/chartConfig';
import {
  authDeleteStatsApi,
  type AuthDeleteStatsBucket,
  type AuthDeleteStatsRange,
  type AuthDeleteStatsResponse,
} from '@/services/api';
import styles from '@/pages/AuthFilesPage.module.scss';

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
);

const CHINA_TIMEZONE = 'Asia/Shanghai';

const formatChinaDateTime = (value: string | Date) =>
  new Intl.DateTimeFormat('zh-CN', {
    timeZone: CHINA_TIMEZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(new Date(value));

const formatChinaTime = (value: string | Date) =>
  new Intl.DateTimeFormat('zh-CN', {
    timeZone: CHINA_TIMEZONE,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(new Date(value));

const formatChinaBucketLabel = (value: string | Date, bucketSeconds: number) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';

  const parts = new Intl.DateTimeFormat('zh-CN', {
    timeZone: CHINA_TIMEZONE,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(date);
  const pick = (type: string) => parts.find((part) => part.type === type)?.value ?? '';
  const monthDay = `${pick('month')}-${pick('day')}`;
  const time = `${pick('hour')}:${pick('minute')}`;

  if (bucketSeconds < 3600) {
    return time;
  }
  if (bucketSeconds < 24 * 3600) {
    return `${monthDay} ${time}`;
  }
  return monthDay;
};

const RANGE_OPTIONS: ReadonlyArray<{ value: AuthDeleteStatsRange; labelKey: string }> = [
  { value: '1h', labelKey: 'auth_delete_stats.range_1h' },
  { value: '24h', labelKey: 'auth_delete_stats.range_24h' },
  { value: '7d', labelKey: 'auth_delete_stats.range_7d' },
  { value: '30d', labelKey: 'auth_delete_stats.range_30d' },
];

type AuthDeleteStatsCardProps = {
  data: AuthDeleteStatsResponse | null;
  range: AuthDeleteStatsRange;
  onRangeChange: (value: AuthDeleteStatsRange) => void;
  loading: boolean;
  error: string;
  onRefresh: () => Promise<unknown>;
  isDark: boolean;
  isMobile: boolean;
  lastRefreshedAt: Date | null;
};

export function AuthDeleteStatsCard({
  data,
  range,
  onRangeChange,
  loading,
  error,
  onRefresh,
  isDark,
  isMobile,
  lastRefreshedAt,
}: AuthDeleteStatsCardProps) {
  const { t } = useTranslation();
  const [detailTarget, setDetailTarget] = useState<AuthDeleteStatsBucket | null>(null);
  const [detailData, setDetailData] = useState<AuthDeleteStatsResponse | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');

  const labels = useMemo(
    () => data?.series?.map((item) => formatChinaBucketLabel(item.start_at, data?.bucket_seconds ?? 0)) ?? [],
    [data]
  );
  const chartSeries = useMemo(() => data?.series ?? [], [data]);
  const chartPeriod = (data?.bucket_seconds ?? 0) < 24 * 60 * 60 ? 'hour' : 'day';

  const chartData = useMemo(
    () => ({
      labels,
      datasets: [
        {
          label: t('auth_delete_stats.legend_401'),
          data: chartSeries.map((item) => item.status_401 ?? 0),
          borderColor: '#ef4444',
          backgroundColor: 'rgba(239, 68, 68, 0.14)',
          pointBackgroundColor: '#ef4444',
          pointBorderColor: '#ef4444',
          fill: false,
        },
        {
          label: t('auth_delete_stats.legend_429'),
          data: chartSeries.map((item) => item.status_429 ?? 0),
          borderColor: '#f59e0b',
          backgroundColor: 'rgba(245, 158, 11, 0.14)',
          pointBackgroundColor: '#f59e0b',
          pointBorderColor: '#f59e0b',
          fill: false,
        },
      ],
    }),
    [chartSeries, labels, t]
  );

  const openBucketDetail = useCallback(
    async (bucket: AuthDeleteStatsBucket | undefined) => {
      if (!bucket) return;

      setDetailTarget(bucket);
      setDetailLoading(true);
      setDetailError('');

      try {
        const payload = await authDeleteStatsApi.getStats({
          from: bucket.start_at,
          to: bucket.end_at,
          bucket: '1m',
        });
        setDetailData(payload);
      } catch (err) {
        const message =
          err instanceof Error && err.message
            ? err.message
            : t('auth_delete_stats.detail_load_failed');
        setDetailError(message);
        setDetailData(null);
      } finally {
        setDetailLoading(false);
      }
    },
    [t]
  );

  const closeBucketDetail = useCallback(() => {
    setDetailTarget(null);
    setDetailData(null);
    setDetailError('');
    setDetailLoading(false);
  }, []);

  const chartOptions = useMemo(() => {
    const baseOptions = buildChartOptions({
      period: chartPeriod,
      labels,
      isDark,
      isMobile,
    });

    return {
      ...baseOptions,
      plugins: {
        ...baseOptions.plugins,
        tooltip: {
          ...baseOptions.plugins?.tooltip,
          callbacks: {
            title(items) {
              const item = items[0];
              const bucket = chartSeries[item?.dataIndex ?? 0];
              return bucket?.label ?? '';
            },
            label(context) {
              const bucket = chartSeries[context.dataIndex];
              const value =
                context.dataset.label === t('auth_delete_stats.legend_401')
                  ? (bucket?.status_401 ?? 0)
                  : (bucket?.status_429 ?? 0);
              return `${context.dataset.label}: ${value}`;
            },
            afterBody(items) {
              const bucket = chartSeries[items[0]?.dataIndex ?? 0];
              return bucket ? `${t('auth_delete_stats.total_label')}: ${bucket.total}` : '';
            },
          },
        },
      },
      onClick: (_event, elements) => {
        if (!elements.length) return;
        void openBucketDetail(chartSeries[elements[0].index]);
      },
    } satisfies ChartOptions<'line'>;
  }, [chartPeriod, chartSeries, isDark, isMobile, labels, openBucketDetail, t]);

  const metaText = useMemo(() => {
    if (!data) {
      return t('auth_delete_stats.range_bucket_meta', {
        range: RANGE_OPTIONS.find((item) => item.value === range)?.value ?? range,
        bucket: '--',
      });
    }
    return t('auth_delete_stats.range_bucket_meta', {
      range: data.range,
      bucket: data.bucket,
    });
  }, [data, range, t]);

  const hasChartData = (data?.totals?.all ?? 0) > 0;
  const detailRows = useMemo(
    () => (detailData?.series ?? []).filter((item) => item.total > 0),
    [detailData]
  );
  const detailTitle = detailTarget
    ? t('auth_delete_stats.detail_title', {
        from: formatChinaDateTime(detailTarget.start_at),
        to: formatChinaDateTime(detailTarget.end_at),
      })
    : t('auth_delete_stats.detail_title_fallback');

  return (
    <Card
      title={
        <div className={styles.titleWrapper}>
          <span>{t('auth_delete_stats.title')}</span>
          {typeof data?.totals?.all === 'number' && data.totals.all > 0 ? (
            <span className={styles.countBadge}>{data.totals.all}</span>
          ) : null}
        </div>
      }
      extra={
        <div className={styles.deleteStatsActions}>
          <div className={styles.deleteStatsRangeButtons}>
            {RANGE_OPTIONS.map((option) => (
              <Button
                key={option.value}
                variant={range === option.value ? 'primary' : 'secondary'}
                size="sm"
                onClick={() => onRangeChange(option.value)}
                disabled={loading}
              >
                {t(option.labelKey)}
              </Button>
            ))}
          </div>
          <Button variant="secondary" size="sm" onClick={() => void onRefresh()} disabled={loading}>
            {loading ? t('common.loading') : t('common.refresh')}
          </Button>
        </div>
      }
      className={styles.deleteStatsCard}
    >
      <div className={styles.deleteStatsIntro}>
        <p className={styles.deleteStatsDescription}>{t('auth_delete_stats.description')}</p>
        <div className={styles.deleteStatsMetaRow}>
          <span className={styles.deleteStatsMeta}>{metaText}</span>
          {lastRefreshedAt ? (
            <span className={styles.deleteStatsMeta}>
              {t('auth_delete_stats.last_updated', {
                time: formatChinaTime(lastRefreshedAt),
              })}
            </span>
          ) : null}
        </div>
      </div>

      <div className={styles.deleteStatsSummary}>
        <div className={styles.deleteStatsSummaryCard}>
          <span className={styles.deleteStatsSummaryLabel}>
            {t('auth_delete_stats.total_label')}
          </span>
          <span className={styles.deleteStatsSummaryValue}>{data?.totals?.all ?? 0}</span>
        </div>
        <div className={styles.deleteStatsSummaryCard}>
          <span className={styles.deleteStatsSummaryLabel}>
            {t('auth_delete_stats.auto_delete_401')}
          </span>
          <span
            className={`${styles.deleteStatsSummaryValue} ${styles.deleteStatsSummaryValue401}`}
          >
            {data?.totals?.status_401 ?? 0}
          </span>
        </div>
        <div className={styles.deleteStatsSummaryCard}>
          <span className={styles.deleteStatsSummaryLabel}>
            {t('auth_delete_stats.auto_delete_429')}
          </span>
          <span
            className={`${styles.deleteStatsSummaryValue} ${styles.deleteStatsSummaryValue429}`}
          >
            {data?.totals?.status_429 ?? 0}
          </span>
        </div>
      </div>

      {error ? <div className={styles.errorBox}>{error}</div> : null}

      {loading && !data ? (
        <div className={styles.hint}>{t('common.loading')}</div>
      ) : hasChartData ? (
        <div className={styles.deleteStatsChartSection}>
          <div
            className={styles.deleteStatsChartLegend}
            aria-label={t('auth_delete_stats.chart_legend_label')}
          >
            {chartData.datasets.map((dataset, index) => (
              <div
                key={`${dataset.label}-${index}`}
                className={styles.deleteStatsLegendItem}
                title={dataset.label}
              >
                <span
                  className={styles.deleteStatsLegendDot}
                  style={{ backgroundColor: String(dataset.borderColor) }}
                />
                <span className={styles.deleteStatsLegendLabel}>{dataset.label}</span>
              </div>
            ))}
          </div>
          <div className={styles.deleteStatsChartArea}>
            <div className={styles.deleteStatsChartScroller}>
              <div
                className={styles.deleteStatsChartCanvas}
                style={
                  chartPeriod === 'hour'
                    ? { minWidth: getHourChartMinWidth(chartData.labels.length, isMobile) }
                    : undefined
                }
              >
                <Line data={chartData} options={chartOptions} />
              </div>
            </div>
          </div>
          <p className={styles.deleteStatsScopeHint}>{t('auth_delete_stats.scope_hint')}</p>
        </div>
      ) : (
        <div className={styles.hint}>{t('auth_delete_stats.empty')}</div>
      )}

      <Modal
        open={detailTarget !== null}
        onClose={closeBucketDetail}
        title={detailTitle}
        width={760}
        footer={
          <div className={styles.deleteStatsDetailFooter}>
            <Button variant="secondary" onClick={closeBucketDetail}>
              {t('common.close')}
            </Button>
          </div>
        }
      >
        <div className={styles.deleteStatsDetailModal}>
          {detailLoading ? (
            <div className={styles.hint}>{t('common.loading')}</div>
          ) : detailError ? (
            <div className={styles.errorBox}>{detailError}</div>
          ) : detailData ? (
            <>
              <div className={styles.deleteStatsDetailSummary}>
                <div className={styles.deleteStatsDetailPill}>
                  <span className={styles.deleteStatsDetailPillLabel}>
                    {t('auth_delete_stats.total_label')}
                  </span>
                  <span className={styles.deleteStatsDetailPillValue}>
                    {detailData.totals?.all ?? 0}
                  </span>
                </div>
                <div className={styles.deleteStatsDetailPill}>
                  <span className={styles.deleteStatsDetailPillLabel}>
                    {t('auth_delete_stats.auto_delete_401')}
                  </span>
                  <span
                    className={`${styles.deleteStatsDetailPillValue} ${styles.deleteStatsSummaryValue401}`}
                  >
                    {detailData.totals?.status_401 ?? 0}
                  </span>
                </div>
                <div className={styles.deleteStatsDetailPill}>
                  <span className={styles.deleteStatsDetailPillLabel}>
                    {t('auth_delete_stats.auto_delete_429')}
                  </span>
                  <span
                    className={`${styles.deleteStatsDetailPillValue} ${styles.deleteStatsSummaryValue429}`}
                  >
                    {detailData.totals?.status_429 ?? 0}
                  </span>
                </div>
              </div>

              {detailRows.length > 0 ? (
                <div className={styles.deleteStatsDetailTableWrapper}>
                  <table className={styles.deleteStatsDetailTable}>
                    <thead>
                      <tr>
                        <th>{t('auth_delete_stats.detail_time')}</th>
                        <th>{t('auth_delete_stats.auto_delete_401')}</th>
                        <th>{t('auth_delete_stats.auto_delete_429')}</th>
                        <th>{t('auth_delete_stats.total_label')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {detailRows.map((item) => (
                        <tr key={item.start_at}>
                          <td>{formatChinaDateTime(item.start_at)}</td>
                          <td>{item.status_401}</td>
                          <td>{item.status_429}</td>
                          <td>{item.total}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className={styles.hint}>{t('auth_delete_stats.detail_empty')}</div>
              )}
            </>
          ) : (
            <div className={styles.hint}>{t('auth_delete_stats.detail_empty')}</div>
          )}
        </div>
      </Modal>
    </Card>
  );
}
