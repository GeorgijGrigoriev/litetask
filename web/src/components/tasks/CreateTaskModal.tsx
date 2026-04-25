import { Input, Modal, Select, Space } from "antd";

import { priorityMeta, priorityOrder } from "../../constants";
import { useTranslation } from "../../i18n";
import type { PriorityKey } from "../../types";

type CreateTaskModalProps = {
  open: boolean;
  title: string;
  description: string;
  priority: PriorityKey;
  creating: boolean;
  canCreate: boolean;
  onTitleChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onPriorityChange: (value: PriorityKey) => void;
  onCreate: () => void;
  onClose: () => void;
};

function CreateTaskModal({
  open,
  title,
  description,
  priority,
  creating,
  canCreate,
  onTitleChange,
  onDescriptionChange,
  onPriorityChange,
  onCreate,
  onClose,
}: CreateTaskModalProps) {
  const { t } = useTranslation();
  return (
    <Modal
      title={t("task.newTitle")}
      open={open}
      onCancel={onClose}
      onOk={onCreate}
      okButtonProps={{ loading: creating, disabled: !canCreate }}
      cancelText={t("common.cancel")}
      okText={t("common.create")}
    >
      <Space direction="vertical" size="small" className="create-column">
        <Input
          placeholder={t("task.titlePlaceholder")}
          value={title}
          onChange={(e) => onTitleChange(e.target.value)}
          onPressEnter={onCreate}
          maxLength={140}
        />
        <Input.TextArea
          placeholder={t("task.descriptionPlaceholder")}
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          autoSize={{ minRows: 3, maxRows: 5 }}
        />
        <Select
          value={priority}
          onChange={onPriorityChange}
          style={{ width: "100%" }}
          options={priorityOrder.map((p) => ({
            label: t(priorityMeta[p].label),
            value: p,
          }))}
        />
      </Space>
    </Modal>
  );
}

export default CreateTaskModal;
