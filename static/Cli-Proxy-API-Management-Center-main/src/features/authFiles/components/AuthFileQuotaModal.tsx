import { useTranslation } from 'react-i18next';
import { useState, useCallback } from 'react';
import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import type { AuthFileItem } from '@/types';
import type { QuotaProviderType } from '@/features/authFiles/constants';
import { AuthFileQuotaSection } from '@/features/authFiles/components/AuthFileQuotaSection';
import styles from '@/pages/AuthFilesPage.module.scss';

type AuthFileQuotaModalProps = {
  open: boolean;
  file: AuthFileItem | null;
  quotaType: QuotaProviderType | null;
  disableControls: boolean;
  onClose: () => void;
};

export function AuthFileQuotaModal({
  open,
  file,
  quotaType,
  disableControls,
  onClose,
}: AuthFileQuotaModalProps) {
  const { t } = useTranslation();
  const [refreshNonce, setRefreshNonce] = useState(0);

  const handleRefresh = useCallback(() => {
    setRefreshNonce((value) => value + 1);
  }, []);

  if (!open || !file || !quotaType) {
    return null;
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      width={720}
      title={
        <div className={styles.quotaModalTitle}>
          <span className={styles.quotaModalTitleText}>
            {t('auth_files.quota_modal_title', { name: file.name })}
          </span>
          <Button variant="secondary" size="sm" onClick={handleRefresh} disabled={disableControls}>
            {t('common.refresh')}
          </Button>
        </div>
      }
    >
      <div className={styles.quotaModalBody}>
        <AuthFileQuotaSection
          file={file}
          quotaType={quotaType}
          disableControls={disableControls}
          autoRefreshOnMount
          refreshNonce={refreshNonce}
        />
      </div>
    </Modal>
  );
}
