/**
 * Quota management page - coordinates the three quota sections.
 */

import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import { useDebounce } from '@/hooks/useDebounce';
import { useAuthStore } from '@/stores';
import { Input } from '@/components/ui/Input';
import {
  QuotaSection,
  ANTIGRAVITY_CONFIG,
  CLAUDE_CONFIG,
  CODEX_CONFIG,
  GEMINI_CLI_CONFIG,
  KIMI_CONFIG
} from '@/components/quota';
import styles from './QuotaPage.module.scss';

export function QuotaPage() {
  const { t } = useTranslation();
  const connectionStatus = useAuthStore((state) => state.connectionStatus);

  const [search, setSearch] = useState('');
  const [refreshNonce, setRefreshNonce] = useState(0);

  const disableControls = connectionStatus !== 'connected';
  const debouncedSearch = useDebounce(search, 300);

  const handleHeaderRefresh = useCallback(async () => {
    setRefreshNonce((value) => value + 1);
  }, []);

  useHeaderRefresh(handleHeaderRefresh);

  return (
    <div className={styles.container}>
      <div className={styles.pageHeader}>
        <h1 className={styles.pageTitle}>{t('quota_management.title')}</h1>
        <p className={styles.description}>{t('quota_management.description')}</p>
      </div>
      <div className={styles.quotaSearchBar}>
        <Input
          label={t('quota_management.search_label')}
          placeholder={t('quota_management.search_placeholder')}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          disabled={disableControls}
        />
      </div>

      <QuotaSection
        config={CLAUDE_CONFIG}
        disabled={disableControls}
        searchQuery={debouncedSearch}
        refreshNonce={refreshNonce}
      />
      <QuotaSection
        config={ANTIGRAVITY_CONFIG}
        disabled={disableControls}
        searchQuery={debouncedSearch}
        refreshNonce={refreshNonce}
      />
      <QuotaSection
        config={CODEX_CONFIG}
        disabled={disableControls}
        searchQuery={debouncedSearch}
        refreshNonce={refreshNonce}
      />
      <QuotaSection
        config={GEMINI_CLI_CONFIG}
        disabled={disableControls}
        searchQuery={debouncedSearch}
        refreshNonce={refreshNonce}
      />
      <QuotaSection
        config={KIMI_CONFIG}
        disabled={disableControls}
        searchQuery={debouncedSearch}
        refreshNonce={refreshNonce}
      />
    </div>
  );
}
