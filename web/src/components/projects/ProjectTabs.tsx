import { Badge, Card, Empty, Space, Tabs, Button } from "antd";
import { PlusOutlined } from "@ant-design/icons";

import type { Project } from "../../types";

type ProjectTabsProps = {
  projects: Project[];
  loadingProjects: boolean;
  selectedProject: number | null;
  projectTaskCounts: Record<number, number>;
  onSelectProject: (projectId: number) => void;
  onOpenCreateTask: () => void;
};

function ProjectTabs({
  projects,
  loadingProjects,
  selectedProject,
  projectTaskCounts,
  onSelectProject,
  onOpenCreateTask,
}: ProjectTabsProps) {
  return (
    <Card className="project-card project-card--compact">
      <div className="project-card-header">
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={onOpenCreateTask}
          disabled={!selectedProject}
          title="Option+N"
        >
          Добавить задачу
        </Button>
        <div className="project-card-title">Проекты</div>
      </div>
      {projects.length === 0 && !loadingProjects ? (
        <Empty description="Нет проектов. Создайте их в настройках" />
      ) : (
        <Tabs
          activeKey={selectedProject ? String(selectedProject) : undefined}
          onChange={(key) => onSelectProject(Number(key))}
          items={projects.map((p) => ({
            key: String(p.id),
            label: (
              <Space size={6}>
                <span>{p.name}</span>
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
