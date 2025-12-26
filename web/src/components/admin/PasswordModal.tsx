import { Input, Modal } from "antd";

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
  return (
    <Modal
      title={user ? `Смена пароля: ${user.email}` : "Смена пароля"}
      open={!!user}
      okText="Обновить пароль"
      cancelText="Отмена"
      onOk={onSave}
      onCancel={onClose}
      confirmLoading={updatingPassword}
    >
      <Input.Password
        placeholder="Новый пароль"
        value={newPassword}
        onChange={(e) => onPasswordChange(e.target.value)}
        onPressEnter={onSave}
      />
      <div className="auth-error" style={{ marginTop: 8 }}>
        Минимум 6 символов
      </div>
    </Modal>
  );
}

export default PasswordModal;
