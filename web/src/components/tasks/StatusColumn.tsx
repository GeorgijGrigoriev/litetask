import { Card, Empty, Space, Tag } from "antd";
import type { MouseEvent } from "react";

import { columnDescriptions, priorityMeta, statusMeta } from "../../constants";
import { useTranslation } from "../../i18n";
import type { StatusKey, Task } from "../../types";
import { formatAuthor, formatDate } from "../../utils/formatters";

type StatusColumnProps = {
  status: StatusKey;
  tasks: Task[];
  onSelectTask: (taskId: number) => void;
};

function StatusColumn({ status, tasks, onSelectTask }: StatusColumnProps) {
  const { t } = useTranslation();

  const handleCardClick = (event: MouseEvent<HTMLElement>, taskId: number) => {
    const target = event.target as HTMLElement | null;
    if (target && target.closest(".card-interactive")) {
      return;
    }
    onSelectTask(taskId);
  };

  return (
    <Card
      data-status={status}
      title={
        <Space size="small">
          {statusMeta[status].icon}
          <span>{t(statusMeta[status].label)}</span>
          <Tag color={statusMeta[status].color}>{tasks.length}</Tag>
        </Space>
      }
      extra={
        <span className="column-subtitle">{t(columnDescriptions[status])}</span>
      }
      className="status-column"
    >
      {tasks.length === 0 && (
        <Empty description={t("task.empty")} image={Empty.PRESENTED_IMAGE_SIMPLE} />
      )}
      <Space direction="vertical" size="middle" className="task-list">
        {tasks.map((task) => (
          <Card
            key={task.id}
            data-status={task.status}
            size="small"
            title={task.title}
            className="task-card"
            hoverable
            onClick={(e) => handleCardClick(e, task.id)}
          >
            <Space direction="vertical" size={2}>
              <Tag color={priorityMeta[task.priority ?? "medium"].color} style={{ marginBottom: 2 }}>
                {t(priorityMeta[task.priority ?? "medium"].label)}
              </Tag>
              <div className="meta-row">
                <span className="meta-label">created</span>
                <span className="meta-value">{formatDate(task.createdAt)}</span>
              </div>
              <div className="meta-row">
                <span className="meta-label">by</span>
                <span className="meta-value">{formatAuthor(task)}</span>
              </div>
            </Space>
          </Card>
        ))}
      </Space>
    </Card>
  );
}

export default StatusColumn;
