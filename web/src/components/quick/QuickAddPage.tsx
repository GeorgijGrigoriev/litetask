import { Button, Card, Empty, Input, Space, Typography } from "antd";

import { useTranslation } from "../../i18n";

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
  const { t } = useTranslation();
  return (
    <Card className="quick-card" title={t("quick.title")}>
      {!hasProject ? (
        <>
          <Empty description={t("quick.inboxUnavailableLong")} />
          <div style={{ marginTop: 12 }}>
            <Button type="default" onClick={onGoToBoard}>
              {t("task.backToList")}
            </Button>
          </div>
        </>
      ) : (
        <Space direction="vertical" size="middle" className="quick-form">
          <Typography.Text type="secondary">
            {t("quick.project")} <strong>{t("common.inbox")}</strong>
          </Typography.Text>
          <Input
            placeholder={t("quick.titlePlaceholder")}
            value={title}
            onChange={(e) => onTitleChange(e.target.value)}
            onPressEnter={onSubmit}
            maxLength={140}
          />
          <Input.TextArea
            placeholder={t("quick.descriptionPlaceholder")}
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
              {t("task.add")}
            </Button>
            <Button type="default" onClick={onGoToBoard}>
              {t("task.backToList")}
            </Button>
          </Space>
        </Space>
      )}
    </Card>
  );
}

export default QuickAddPage;
