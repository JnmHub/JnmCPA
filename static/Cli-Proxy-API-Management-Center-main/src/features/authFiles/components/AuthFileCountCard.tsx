import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import type { AuthFileCountResponse } from '@/services/api/authFiles';
import styles from '@/pages/AuthFilesPage.module.scss';

type AuthFileCountCardProps = {
  counts: AuthFileCountResponse | null;
  loading: boolean;
  error: string;
  onRefresh: () => Promise<unknown>;
};

export function AuthFileCountCard({ counts, loading, error, onRefresh }: AuthFileCountCardProps) {
  const { t } = useTranslation();

  const totalTypes = useMemo(() => {
    const entries = counts?.type_counts ? Object.entries(counts.type_counts) : [];
    return entries.filter(([key]) => key !== 'all' && key.trim()).length;
  }, [counts]);

  return (
    <Card
      title={
        <div className={styles.titleWrapper}>
          <span>{t('auth_file_counts.title')}</span>
          {typeof counts?.total === 'number' && counts.total > 0 ? (
            <span className={styles.countBadge}>{counts.total}</span>
          ) : null}
        </div>
      }
      extra={
        <Button variant="secondary" size="sm" onClick={() => void onRefresh()} disabled={loading}>
          {loading ? t('common.loading') : t('common.refresh')}
        </Button>
      }
      className={styles.authFileCountCard}
    >
      <p className={styles.authFileCountDescription}>{t('auth_file_counts.description')}</p>

      {error ? <div className={styles.errorBox}>{error}</div> : null}

      {loading && !counts ? (
        <div className={styles.hint}>{t('common.loading')}</div>
      ) : (
        <div className={styles.authFileCountGrid}>
          <div className={styles.authFileCountTile}>
            <span className={styles.authFileCountLabel}>{t('auth_file_counts.total')}</span>
            <span className={styles.authFileCountValue}>{counts?.total ?? 0}</span>
          </div>
          <div className={styles.authFileCountTile}>
            <span className={styles.authFileCountLabel}>{t('auth_file_counts.enabled')}</span>
            <span className={styles.authFileCountValue}>{counts?.enabled ?? 0}</span>
          </div>
          <div className={styles.authFileCountTile}>
            <span className={styles.authFileCountLabel}>{t('auth_file_counts.disabled')}</span>
            <span className={styles.authFileCountValue}>{counts?.disabled ?? 0}</span>
          </div>
          <div className={styles.authFileCountTile}>
            <span className={styles.authFileCountLabel}>{t('auth_file_counts.usable')}</span>
            <span className={styles.authFileCountValue}>{counts?.usable ?? 0}</span>
          </div>
          <div className={styles.authFileCountTile}>
            <span className={styles.authFileCountLabel}>{t('auth_file_counts.cooling')}</span>
            <span className={styles.authFileCountValue}>{counts?.cooling ?? 0}</span>
          </div>
          <div className={styles.authFileCountTile}>
            <span className={styles.authFileCountLabel}>{t('auth_file_counts.problem')}</span>
            <span className={`${styles.authFileCountValue} ${styles.authFileCountValueDanger}`}>
              {counts?.problem_count ?? 0}
            </span>
          </div>
          <div className={styles.authFileCountTile}>
            <span className={styles.authFileCountLabel}>{t('auth_file_counts.types')}</span>
            <span className={styles.authFileCountValue}>{totalTypes}</span>
          </div>
        </div>
      )}
    </Card>
  );
}
