import { Divider, Form, Input, Modal, Typography } from "antd";

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
  onFirstNameChange: (value: string) => void;
  onLastNameChange: (value: string) => void;
  onUsernameChange: (value: string) => void;
  onTelegramChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
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
  onFirstNameChange,
  onLastNameChange,
  onUsernameChange,
  onTelegramChange,
  onPasswordChange,
  onSave,
  onClose,
}: ProfileModalProps) {
  return (
    <Modal
      title="Настройки профиля"
      open={open}
      onCancel={onClose}
      onOk={onSave}
      confirmLoading={saving}
      okText="Сохранить"
      cancelText="Отмена"
    >
      <Form
        className="profile-form"
        layout="horizontal"
        colon={false}
        labelAlign="left"
        labelCol={{ flex: "140px" }}
        wrapperCol={{ flex: 1 }}
      >
        <Form.Item label="Имя">
          <Input
            placeholder="Имя"
            value={profileFirstName}
            onChange={(e) => onFirstNameChange(e.target.value)}
          />
        </Form.Item>
        <Form.Item label="Фамилия">
          <Input
            placeholder="Фамилия"
            value={profileLastName}
            onChange={(e) => onLastNameChange(e.target.value)}
          />
        </Form.Item>
        <Form.Item label="Юзернейм">
          {user.username ? (
            <Input value={user.username} disabled />
          ) : (
            <Input
              placeholder="Юзернейм (указывается один раз)"
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
            placeholder="Telegram (@username)"
            value={profileTelegram}
            onChange={(e) => onTelegramChange(e.target.value)}
          />
        </Form.Item>
        <Divider style={{ margin: "12px 0" }} />
        <Typography.Text type="secondary" style={{ display: "block" }}>
          Смена пароля
        </Typography.Text>
        <Form.Item
          label="Новый пароль"
          extra="Пароль минимум 6 символов. Оставьте пустым, если не хотите менять."
        >
          <Input.Password
            placeholder="Новый пароль (опционально)"
            value={profilePassword}
            onChange={(e) => onPasswordChange(e.target.value)}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export default ProfileModal;
