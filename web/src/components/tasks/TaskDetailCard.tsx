import {
  ArrowLeftOutlined,
  DeleteOutlined,
  FolderOpenOutlined,
  HighlightOutlined,
  LoadingOutlined,
} from "@ant-design/icons";
import { Button, Card, Input, Popconfirm, Select, Space, Tag } from "antd";
import { useState } from "react";

import { priorityMeta, priorityOrder, statusMeta, statusOrder } from "../../constants";
import { useTranslation } from "../../i18n";
import type { PriorityKey, Project, StatusKey, Task, User } from "../../types";
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
  projects: Project[];
  movingTaskId: number | null;
  onMoveTask: (taskId: number, targetProjectId: number) => void;
  onChangePriority: (taskId: number, priority: PriorityKey) => void;
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
  projects,
  movingTaskId,
  onMoveTask,
  onChangePriority,
}: TaskDetailCardProps) {
  const { t } = useTranslation();
  const [pendingProjectId, setPendingProjectId] = useState<number | null>(null);
  const pendingProject = projects.find((p) => p.id === pendingProjectId);
  const pendingProjectName = pendingProject?.isInbox
    ? t("common.inbox")
    : pendingProject?.name;
  return (
    <Card
      className="task-details-card"
      title={
        <Space size="middle">
          <Button type="link" icon={<ArrowLeftOutlined />} onClick={onClose}>
            {t("task.backToList")}
          </Button>
          <span className="detail-title">{task.title}</span>
        </Space>
      }
      extra={
        <Space size="small" wrap>
          <Tag color={statusMeta[task.status].color}>
            {t(statusMeta[task.status].label)}
          </Tag>
          <Select
            size="small"
            value={task.status}
            onChange={(value) => onChangeStatus(task.id, value as StatusKey)}
            dropdownMatchSelectWidth={false}
            options={statusOrder.map((status) => ({
              label: t(statusMeta[status].label),
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
          <Tag color={priorityMeta[task.priority ?? "medium"].color}>
            {t(priorityMeta[task.priority ?? "medium"].label)}
          </Tag>
          <Select
            size="small"
            value={task.priority ?? "medium"}
            onChange={(value) => onChangePriority(task.id, value as PriorityKey)}
            dropdownMatchSelectWidth={false}
            options={priorityOrder.map((p) => ({
              label: t(priorityMeta[p].label),
              value: p,
            }))}
            style={{ minWidth: 130 }}
          />
          <Popconfirm
            title={t("task.moveConfirm", { name: pendingProjectName ?? "" })}
            open={pendingProjectId !== null}
            onConfirm={() => {
              if (pendingProjectId !== null) onMoveTask(task.id, pendingProjectId);
              setPendingProjectId(null);
            }}
            onCancel={() => setPendingProjectId(null)}
          >
            <Select
              size="small"
              placeholder={t("task.moveTo")}
              value={null}
              onChange={(value: number) => setPendingProjectId(value)}
              loading={movingTaskId === task.id}
              disabled={
                movingTaskId === task.id ||
                projects.filter((p) => p.id !== task.projectId).length === 0
              }
              dropdownMatchSelectWidth={false}
              style={{ minWidth: 160 }}
              suffixIcon={<FolderOpenOutlined />}
              options={projects
                .filter((p) => p.id !== task.projectId)
                .map((p) => ({
                  label: p.isInbox ? t("common.inbox") : p.name,
                  value: p.id,
                }))}
            />
          </Popconfirm>
          <Popconfirm
            title={t("task.deleteConfirm")}
            onConfirm={() => onDeleteTask(task.id)}
          >
            <Button
              icon={<DeleteOutlined />}
              danger
              loading={deletingTaskId === task.id}
            >
              {t("common.delete")}
            </Button>
          </Popconfirm>
        </Space>
      }
    >
      <Space direction="vertical" size="large" className="detail-content">
        <Space size="large" className="detail-meta" wrap>
          <div className="meta-row">
            <span className="meta-label">{t("task.createdAt")}</span>
            <span className="meta-value">{formatDate(task.createdAt)}</span>
          </div>
          <div className="meta-row">
            <span className="meta-label">{t("task.author")}</span>
            <span className="meta-value">{formatAuthor(task)}</span>
          </div>
        </Space>

        <div className="detail-section">
          <div className="section-header">
            <span className="meta-label">{t("task.description")}</span>
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
                placeholder={t("task.descriptionEditPlaceholder")}
              />
              <Space size="small">
                <Button
                  type="primary"
                  onClick={() => onSaveDescription(task.id)}
                  loading={updatingId === task.id}
                >
                  {t("common.save")}
                </Button>
                <Button onClick={onCancelEditDescription}>{t("common.cancel")}</Button>
              </Space>
            </Space>
          ) : (
            <div className="description-display">
              <p className={task.description ? "meta-value" : "muted-text"}>
                {task.description || t("task.noDescription")}
              </p>
              <Button type="link" onClick={() => onStartEditDescription(task)}>
                {t("common.edit")}
              </Button>
            </div>
          )}
        </div>

        <div className="detail-section">
          <div className="comments-header">
            <span className="meta-label">{t("task.comments")}</span>
            <Tag color="default">{task.comments?.length ?? 0}</Tag>
          </div>
          {task.comments && task.comments.length > 0 ? (
            <Space direction="vertical" size="small" className="comments-list">
              {task.comments.map((comment) => (
                <div key={comment.id} className="comment-item">
                  <div className="comment-meta">
                    <span>{formatDate(comment.createdAt)}</span>
                    <Space size="small">
                      <span>{comment.authorEmail || t("task.authorUnknown")}</span>
                      {comment.authorId === user.id && (
                        <Popconfirm
                          title={t("task.deleteCommentConfirm")}
                          onConfirm={() => onDeleteComment(comment.id)}
                        >
                          <Button
                            type="link"
                            size="small"
                            danger
                            loading={deletingCommentId === comment.id}
                          >
                            {t("common.delete")}
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
            <div className="muted-text">{t("task.noComments")}</div>
          )}
          <div className="comment-form">
            <Input.TextArea
              value={commentDraft}
              onChange={(e) => onCommentDraftChange(e.target.value)}
              autoSize={{ minRows: 3, maxRows: 4 }}
              placeholder={t("task.commentPlaceholder")}
            />
            <Button
              type="primary"
              onClick={onAddComment}
              loading={addingComment}
              disabled={!commentDraft.trim()}
            >
              {t("task.addComment")}
            </Button>
          </div>
        </div>
      </Space>
    </Card>
  );
}

export default TaskDetailCard;
