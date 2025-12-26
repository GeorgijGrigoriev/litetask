import { Card, Empty, Space, Tag } from "antd";
import type { MouseEvent } from "react";

import { columnDescriptions, statusMeta } from "../../constants";
import type { StatusKey, Task } from "../../types";
import { formatAuthor, formatDate } from "../../utils/formatters";

type StatusColumnProps = {
  status: StatusKey;
  tasks: Task[];
  onSelectTask: (taskId: number) => void;
};

function StatusColumn({ status, tasks, onSelectTask }: StatusColumnProps) {
  const handleCardClick = (event: MouseEvent<HTMLElement>, taskId: number) => {
    const target = event.target as HTMLElement | null;
    if (target && target.closest(".card-interactive")) {
      return;
    }
    onSelectTask(taskId);
  };

  return (
    <Card
      title={
        <Space size="small">
          {statusMeta[status].icon}
          <span>{statusMeta[status].label}</span>
          <Tag color={statusMeta[status].color}>{tasks.length}</Tag>
        </Space>
      }
      extra={
        <span className="column-subtitle">{columnDescriptions[status]}</span>
      }
      className="status-column"
    >
      {tasks.length === 0 && (
        <Empty description="Нет задач" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      )}
      <Space direction="vertical" size="middle" className="task-list">
        {tasks.map((task) => (
          <Card
            key={task.id}
            size="small"
            title={task.title}
            className="task-card"
            hoverable
            onClick={(e) => handleCardClick(e, task.id)}
            extra={
              <Tag color={statusMeta[task.status].color}>
                {statusMeta[task.status].label}
              </Tag>
            }
          >
            <Space direction="vertical" size={4}>
              <div className="meta-row">
                <span className="meta-label">Создана:</span>
                <span className="meta-value">{formatDate(task.createdAt)}</span>
              </div>
              <div className="meta-row">
                <span className="meta-label">Автор:</span>
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
