import { Button, Card, Empty, Input, Space, Typography } from "antd";

type QuickAddPageProps = {
  title: string;
  description: string;
  creating: boolean;
  hasProject: boolean;
  onTitleChange: (value: string) => void;
  onDescriptionChange: (value: string) => void;
  onSubmit: () => void;
  onGoToBoard: () => void;
};

function QuickAddPage({
  title,
  description,
  creating,
  hasProject,
  onTitleChange,
  onDescriptionChange,
  onSubmit,
  onGoToBoard,
}: QuickAddPageProps) {
  return (
    <Card className="quick-card" title="Быстрый ввод задачи">
      {!hasProject ? (
        <>
          <Empty description="Проект «Входящие» недоступен. Обратитесь к администратору." />
          <div style={{ marginTop: 12 }}>
            <Button type="default" onClick={onGoToBoard}>
              К списку задач
            </Button>
          </div>
        </>
      ) : (
        <Space direction="vertical" size="middle" className="quick-form">
          <Typography.Text type="secondary">
            Проект: <strong>Входящие</strong>
          </Typography.Text>
          <Input
            placeholder="Тема"
            value={title}
            onChange={(e) => onTitleChange(e.target.value)}
            onPressEnter={onSubmit}
            maxLength={140}
          />
          <Input.TextArea
            placeholder="Описание (опционально)"
            value={description}
            onChange={(e) => onDescriptionChange(e.target.value)}
            autoSize={{ minRows: 4, maxRows: 8 }}
          />
          <Space size="middle">
            <Button
              type="primary"
              onClick={onSubmit}
              loading={creating}
              disabled={!title.trim()}
            >
              Добавить задачу
            </Button>
            <Button type="default" onClick={onGoToBoard}>
              К списку задач
            </Button>
          </Space>
        </Space>
      )}
    </Card>
  );
}

export default QuickAddPage;
