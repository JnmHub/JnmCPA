import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal } from '@/components/ui/Modal';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import type { AuthFileItem } from '@/types';
import { parsePriorityValue } from '@/features/authFiles/constants';

type AuthFileWeightModalProps = {
  open: boolean;
  file: AuthFileItem | null;
  saving: boolean;
  disableControls: boolean;
  onClose: () => void;
  onSave: (priority: number | null) => Promise<void> | void;
};

export function AuthFileWeightModal({
  open,
  file,
  saving,
  disableControls,
  onClose,
  onSave,
}: AuthFileWeightModalProps) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');

  useEffect(() => {
    if (!open || !file) return;
    const current = parsePriorityValue(file.priority ?? file['priority']);
    setValue(current !== undefined ? String(current) : '');
  }, [file, open]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      closeDisabled={saving}
      width={520}
      title={t('auth_files.weight_modal_title', { name: file?.name ?? '' })}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button
            onClick={() => void onSave(parsePriorityValue(value) ?? null)}
            loading={saving}
            disabled={disableControls || saving}
          >
            {t('common.save')}
          </Button>
        </>
      }
    >
      <Input
        label={t('auth_files.priority_label')}
        value={value}
        placeholder={t('auth_files.priority_placeholder')}
        hint={t('auth_files.priority_hint')}
        disabled={disableControls || saving}
        onChange={(event) => setValue(event.target.value)}
      />
    </Modal>
  );
}
