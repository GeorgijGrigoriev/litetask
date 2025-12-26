import { Input, Modal, Space } from "antd";

type CreateTaskModalProps = {
  open: boolean;
  title: string;
  description: string;
  creating: boolean;
  canCreate: boolean;
  onTitleChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onCreate: () => void;
  onClose: () => void;
};

function CreateTaskModal({
  open,
  title,
  description,
  creating,
  canCreate,
  onTitleChange,
  onDescriptionChange,
  onCreate,
  onClose,
}: CreateTaskModalProps) {
  return (
    <Modal
      title="Новая задача"
      open={open}
      onCancel={onClose}
      onOk={onCreate}
      okButtonProps={{ loading: creating, disabled: !canCreate }}
      cancelText="Отмена"
      okText="Создать"
    >
      <Space direction="vertical" size="small" className="create-column">
        <Input
          placeholder="Новая задача"
          value={title}
          onChange={(e) => onTitleChange(e.target.value)}
          onPressEnter={onCreate}
          maxLength={140}
        />
        <Input.TextArea
          placeholder="Описание задачи (необязательно)"
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          autoSize={{ minRows: 3, maxRows: 5 }}
        />
      </Space>
    </Modal>
  );
}

export default CreateTaskModal;
