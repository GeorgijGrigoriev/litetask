import { Input, Modal, Space } from "antd";

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
  return (
    <Modal
      title={user ? `Данные пользователя: ${user.email}` : "Данные пользователя"}
      open={!!user}
      onCancel={onClose}
      onOk={onSave}
      okText="Сохранить"
      cancelText="Отмена"
      confirmLoading={saving}
    >
      <Space direction="vertical" size="middle" style={{ width: "100%" }}>
        <Input
          placeholder="Имя"
          value={firstName}
          onChange={(e) => onFirstNameChange(e.target.value)}
        />
        <Input
          placeholder="Фамилия"
          value={lastName}
          onChange={(e) => onLastNameChange(e.target.value)}
        />
      </Space>
    </Modal>
  );
}

export default UserInfoModal;
