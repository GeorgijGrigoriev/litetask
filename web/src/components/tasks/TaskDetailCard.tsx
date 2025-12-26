import {
  ArrowLeftOutlined,
  DeleteOutlined,
  HighlightOutlined,
  LoadingOutlined,
} from "@ant-design/icons";
import { Button, Card, Input, Popconfirm, Select, Space, Tag } from "antd";

import { statusMeta, statusOrder } from "../../constants";
import type { StatusKey, Task, User } from "../../types";
import { formatAuthor, formatDate } from "../../utils/formatters";

type TaskDetailCardProps = {
  task: Task;
  user: User;
  updatingId: number | null;
  deletingTaskId: number | null;
  deletingCommentId: number | null;
  editingId: number | null;
  descriptionDraft: string;
  commentDraft: string;
  addingComment: boolean;
  onClose: () => void;
  onChangeStatus: (taskId: number, status: StatusKey) => void;
  onDeleteTask: (taskId: number) => void;
  onStartEditDescription: (task: Task) => void;
  onCancelEditDescription: () => void;
  onSaveDescription: (taskId: number) => void;
  onDescriptionDraftChange: (value: string) => void;
  onCommentDraftChange: (value: string) => void;
  onAddComment: () => void;
  onDeleteComment: (commentId: number) => void;
};

function TaskDetailCard({
  task,
  user,
  updatingId,
  deletingTaskId,
  deletingCommentId,
  editingId,
  descriptionDraft,
  commentDraft,
  addingComment,
  onClose,
  onChangeStatus,
  onDeleteTask,
  onStartEditDescription,
  onCancelEditDescription,
  onSaveDescription,
  onDescriptionDraftChange,
  onCommentDraftChange,
  onAddComment,
  onDeleteComment,
}: TaskDetailCardProps) {
  return (
    <Card
      className="task-details-card"
      title={
        <Space size="middle">
          <Button type="link" icon={<ArrowLeftOutlined />} onClick={onClose}>
            К списку задач
          </Button>
          <span className="detail-title">{task.title}</span>
        </Space>
      }
      extra={
        <Space size="small" wrap>
          <Tag color={statusMeta[task.status].color}>
            {statusMeta[task.status].label}
          </Tag>
          <Select
            size="small"
            value={task.status}
            onChange={(value) => onChangeStatus(task.id, value as StatusKey)}
            dropdownMatchSelectWidth={false}
            options={statusOrder.map((status) => ({
              label: statusMeta[status].label,
              value: status,
            }))}
            suffixIcon={
              updatingId === task.id ? (
                <LoadingOutlined spin />
              ) : (
                <HighlightOutlined />
              )
            }
            style={{ minWidth: 180 }}
          />
          <Popconfirm
            title="Удалить задачу?"
            onConfirm={() => onDeleteTask(task.id)}
          >
            <Button
              icon={<DeleteOutlined />}
              danger
              loading={deletingTaskId === task.id}
            >
              Удалить
            </Button>
          </Popconfirm>
        </Space>
      }
    >
      <Space direction="vertical" size="large" className="detail-content">
        <Space size="large" className="detail-meta" wrap>
          <div className="meta-row">
            <span className="meta-label">Создана:</span>
            <span className="meta-value">{formatDate(task.createdAt)}</span>
          </div>
          <div className="meta-row">
            <span className="meta-label">Автор:</span>
            <span className="meta-value">{formatAuthor(task)}</span>
          </div>
        </Space>

        <div className="detail-section">
          <div className="section-header">
            <span className="meta-label">Описание</span>
          </div>
          {editingId === task.id ? (
            <Space
              direction="vertical"
              size="small"
              className="description-edit"
            >
              <Input.TextArea
                value={descriptionDraft}
                onChange={(e) => onDescriptionDraftChange(e.target.value)}
                autoSize={{ minRows: 3, maxRows: 6 }}
                placeholder="Опишите детали задачи"
              />
              <Space size="small">
                <Button
                  type="primary"
                  onClick={() => onSaveDescription(task.id)}
                  loading={updatingId === task.id}
                >
                  Сохранить
                </Button>
                <Button onClick={onCancelEditDescription}>Отмена</Button>
              </Space>
            </Space>
          ) : (
            <div className="description-display">
              <p className={task.description ? "meta-value" : "muted-text"}>
                {task.description || "Описание отсутствует"}
              </p>
              <Button type="link" onClick={() => onStartEditDescription(task)}>
                Изменить
              </Button>
            </div>
          )}
        </div>

        <div className="detail-section">
          <div className="comments-header">
            <span className="meta-label">Комментарии</span>
            <Tag color="default">{task.comments?.length ?? 0}</Tag>
          </div>
          {task.comments && task.comments.length > 0 ? (
            <Space direction="vertical" size="small" className="comments-list">
              {task.comments.map((comment) => (
                <div key={comment.id} className="comment-item">
                  <div className="comment-meta">
                    <span>{formatDate(comment.createdAt)}</span>
                    <Space size="small">
                      <span>{comment.authorEmail || "Не указан"}</span>
                      {comment.authorId === user.id && (
                        <Popconfirm
                          title="Удалить комментарий?"
                          onConfirm={() => onDeleteComment(comment.id)}
                        >
                          <Button
                            type="link"
                            size="small"
                            danger
                            loading={deletingCommentId === comment.id}
                          >
                            Удалить
                          </Button>
                        </Popconfirm>
                      )}
                    </Space>
                  </div>
                  <div className="comment-body">{comment.body}</div>
                </div>
              ))}
            </Space>
          ) : (
            <div className="muted-text">Комментариев нет</div>
          )}
          <div className="comment-form">
            <Input.TextArea
              value={commentDraft}
              onChange={(e) => onCommentDraftChange(e.target.value)}
              autoSize={{ minRows: 3, maxRows: 4 }}
              placeholder="Новый комментарий"
            />
            <Button
              type="primary"
              onClick={onAddComment}
              loading={addingComment}
              disabled={!commentDraft.trim()}
            >
              Добавить комментарий
            </Button>
          </div>
        </div>
      </Space>
    </Card>
  );
}

export default TaskDetailCard;
