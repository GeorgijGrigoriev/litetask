import { Divider, Form, Input, Modal, Select, Typography } from "antd";

import { useTranslation, type LangCode } from "../../i18n";
import type { User } from "../../types";

type ProfileModalProps = {
  open: boolean;
  user: User;
  saving: boolean;
  profileFirstName: string;
  profileLastName: string;
  profileUsername: string;
  profileTelegram: string;
  profilePassword: string;
  profileLanguage: LangCode;
  onFirstNameChange: (value: string) => void;
  onLastNameChange: (value: string) => void;
  onUsernameChange: (value: string) => void;
  onTelegramChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onLanguageChange: (value: LangCode) => void;
  onSave: () => void;
  onClose: () => void;
};

function ProfileModal({
  open,
  user,
  saving,
  profileFirstName,
  profileLastName,
  profileUsername,
  profileTelegram,
  profilePassword,
  profileLanguage,
  onFirstNameChange,
  onLastNameChange,
  onUsernameChange,
  onTelegramChange,
  onPasswordChange,
  onLanguageChange,
  onSave,
  onClose,
}: ProfileModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      title={t("profile.title")}
      open={open}
      onCancel={onClose}
      onOk={onSave}
      confirmLoading={saving}
      okText={t("common.save")}
      cancelText={t("common.cancel")}
    >
      <Form
        className="profile-form"
        layout="horizontal"
        colon={false}
        labelAlign="left"
        labelCol={{ flex: "140px" }}
        wrapperCol={{ flex: 1 }}
      >
        <Form.Item label={t("auth.firstName")}>
          <Input
            placeholder={t("auth.firstName")}
            value={profileFirstName}
            onChange={(e) => onFirstNameChange(e.target.value)}
          />
        </Form.Item>
        <Form.Item label={t("auth.lastName")}>
          <Input
            placeholder={t("auth.lastName")}
            value={profileLastName}
            onChange={(e) => onLastNameChange(e.target.value)}
          />
        </Form.Item>
        <Form.Item label={t("auth.username")}>
          {user.username ? (
            <Input value={user.username} disabled />
          ) : (
            <Input
              placeholder={t("profile.usernameOnce")}
              value={profileUsername}
              onChange={(e) => onUsernameChange(e.target.value)}
              autoCapitalize="none"
              autoCorrect="off"
            />
          )}
        </Form.Item>
        <Form.Item label="Email">
          <Input value={user.email} disabled />
        </Form.Item>
        <Form.Item label="Telegram">
          <Input
            placeholder={t("profile.telegramPlaceholder")}
            value={profileTelegram}
            onChange={(e) => onTelegramChange(e.target.value)}
          />
        </Form.Item>
        <Form.Item label={t("profile.language")}>
          <Select
            value={profileLanguage}
            onChange={onLanguageChange}
            options={[
              { value: "ru", label: "Русский" },
              { value: "en", label: "English" },
            ]}
          />
        </Form.Item>
        <Divider style={{ margin: "12px 0" }} />
        <Typography.Text type="secondary" style={{ display: "block" }}>
          {t("profile.changePassword")}
        </Typography.Text>
        <Form.Item
          label={t("profile.newPassword")}
          extra={t("profile.passwordHint")}
        >
          <Input.Password
            placeholder={t("profile.newPasswordPlaceholder")}
            value={profilePassword}
            onChange={(e) => onPasswordChange(e.target.value)}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export default ProfileModal;
