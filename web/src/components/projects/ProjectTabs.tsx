import { Badge, Card, Empty, Space, Tabs, Button } from "antd";
import { ArrowDownOutlined, ArrowUpOutlined, PlusOutlined, SwapOutlined } from "@ant-design/icons";

import { useTranslation } from "../../i18n";
import type { Project } from "../../types";

type ProjectTabsProps = {
  projects: Project[];
  loadingProjects: boolean;
  selectedProject: number | null;
  projectTaskCounts: Record<number, number>;
  onSelectProject: (projectId: number) => void;
  onOpenCreateTask: () => void;
  onGoToQuick: () => void;
  prioritySort: "asc" | "desc" | null;
  onTogglePrioritySort: () => void;
};

function ProjectTabs({
  projects,
  loadingProjects,
  selectedProject,
  projectTaskCounts,
  onSelectProject,
  onOpenCreateTask,
  onGoToQuick,
  prioritySort,
  onTogglePrioritySort,
}: ProjectTabsProps) {
  const { t } = useTranslation();
  return (
    <Card className="project-card project-card--compact">
      <div className="project-card-header">
        <Space size="small">
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={onOpenCreateTask}
            disabled={!selectedProject}
            title="Option+N"
          >
            {t("task.add")}
          </Button>
          <Button onClick={onGoToQuick}>{t("nav.quickInput")}</Button>
          <Button
            icon={
              prioritySort === "asc"
                ? <ArrowDownOutlined />
                : prioritySort === "desc"
                ? <ArrowUpOutlined />
                : <SwapOutlined rotate={90} />
            }
            onClick={onTogglePrioritySort}
            type={prioritySort !== null ? "primary" : "default"}
            title={t("task.priority")}
          >
            {prioritySort === "asc"
              ? t("task.sortHighToLow")
              : prioritySort === "desc"
              ? t("task.sortLowToHigh")
              : t("task.priority")}
          </Button>
        </Space>
        <div className="project-card-title">{t("project.title")}</div>
      </div>
      {projects.length === 0 && !loadingProjects ? (
        <Empty description={t("project.noProjects")} />
      ) : (
        <Tabs
          activeKey={selectedProject ? String(selectedProject) : undefined}
          onChange={(key) => onSelectProject(Number(key))}
          items={projects.map((p) => ({
            key: String(p.id),
            label: (
              <Space size={6}>
                <span>{p.isInbox ? t("common.inbox") : p.name}</span>
                <Badge
                  count={
                    projectTaskCounts[p.id] !== undefined
                      ? projectTaskCounts[p.id]
                      : "?"
                  }
                  overflowCount={99}
                  size="small"
                />
              </Space>
            ),
          }))}
        />
      )}
    </Card>
  );
}

export default ProjectTabs;
