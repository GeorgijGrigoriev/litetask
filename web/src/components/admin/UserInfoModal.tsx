import { Input, Modal, Space } from "antd";

import { useTranslation } from "../../i18n";
import type { User } from "../../types";

type UserInfoModalProps = {
  user: User | null;
  firstName: string;
  lastName: string;
  saving: boolean;
  onFirstNameChange: (value: string) => void;
  onLastNameChange: (value: string) => void;
  onSave: () => void;
  onClose: () => void;
};

function UserInfoModal({
  user,
  firstName,
  lastName,
  saving,
  onFirstNameChange,
  onLastNameChange,
  onSave,
  onClose,
}: UserInfoModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      title={
        user
          ? t("admin.userInfoTitle", { email: user.email })
          : t("admin.userInfoTitleSimple")
      }
      open={!!user}
      onCancel={onClose}
      onOk={onSave}
      okText={t("common.save")}
      cancelText={t("common.cancel")}
      confirmLoading={saving}
    >
      <Space direction="vertical" size="middle" style={{ width: "100%" }}>
        <Input
          placeholder={t("auth.firstName")}
          value={firstName}
          onChange={(e) => onFirstNameChange(e.target.value)}
        />
        <Input
          placeholder={t("auth.lastName")}
          value={lastName}
          onChange={(e) => onLastNameChange(e.target.value)}
        />
      </Space>
    </Modal>
  );
}

export default UserInfoModal;
