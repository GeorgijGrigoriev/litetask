import { FolderAddOutlined } from "@ant-design/icons";
import { Button, Card, Input, Popconfirm, Select, Space } from "antd";

import type { Project } from "../../types";

type ProjectManagerCardProps = {
  newProjectName: string;
  onNewProjectNameChange: (value: string) => void;
  onCreateProject: () => void;
  creatingProject: boolean;
  projects: Project[];
  loadingProjects: boolean;
  selectedProject: number | null;
  onProjectChange: (projectId: number) => void;
  onDeleteProject: () => void;
  deletingProject: boolean;
};

function ProjectManagerCard({
  newProjectName,
  onNewProjectNameChange,
  onCreateProject,
  creatingProject,
  projects,
  loadingProjects,
  selectedProject,
  onProjectChange,
  onDeleteProject,
  deletingProject,
}: ProjectManagerCardProps) {
  return (
    <Card className="project-card" title="Управление проектами">
      <Space direction="vertical" size="middle" className="project-row">
        <Space.Compact>
          <Input
            placeholder="Новый проект"
            value={newProjectName}
            onChange={(e) => onNewProjectNameChange(e.target.value)}
            onPressEnter={onCreateProject}
          />
          <Button
            type="primary"
            icon={<FolderAddOutlined />}
            onClick={onCreateProject}
            loading={creatingProject}
          >
            Создать
          </Button>
        </Space.Compact>
        <Space size="middle" wrap>
          <Select
            placeholder="Выберите проект для удаления"
            loading={loadingProjects}
            value={selectedProject ?? undefined}
            onChange={onProjectChange}
            options={projects.map((p) => ({
              value: p.id,
              label: p.name,
            }))}
            style={{ minWidth: 240 }}
          />
          <Popconfirm
            title="Удалить проект и все его задачи?"
            onConfirm={onDeleteProject}
            disabled={!selectedProject}
          >
            <Button danger loading={deletingProject} disabled={!selectedProject}>
              Удалить проект
            </Button>
          </Popconfirm>
        </Space>
      </Space>
    </Card>
  );
}

export default ProjectManagerCard;
