import { Input, Modal } from "antd";

import { useTranslation } from "../../i18n";
import type { User } from "../../types";

type PasswordModalProps = {
  user: User | null;
  newPassword: string;
  updatingPassword: boolean;
  onPasswordChange: (value: string) => void;
  onSave: () => void;
  onClose: () => void;
};

function PasswordModal({
  user,
  newPassword,
  updatingPassword,
  onPasswordChange,
  onSave,
  onClose,
}: PasswordModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      title={
        user
          ? t("admin.passwordModalTitle", { email: user.email })
          : t("admin.passwordModalTitleSimple")
      }
      open={!!user}
      okText={t("admin.updatePassword")}
      cancelText={t("common.cancel")}
      onOk={onSave}
      onCancel={onClose}
      confirmLoading={updatingPassword}
    >
      <Input.Password
        placeholder={t("profile.newPassword")}
        value={newPassword}
        onChange={(e) => onPasswordChange(e.target.value)}
        onPressEnter={onSave}
      />
      <div className="auth-error" style={{ marginTop: 8 }}>
        {t("auth.passwordMinLength")}
      </div>
    </Modal>
  );
}

export default PasswordModal;
