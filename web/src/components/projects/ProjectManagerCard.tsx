import { FolderAddOutlined } from "@ant-design/icons";
import { Button, Card, Input, Popconfirm, Select, Space } from "antd";

import { useTranslation } from "../../i18n";
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
  const { t } = useTranslation();
  return (
    <Card className="project-card" title={t("project.manageTitle")}>
      <Space direction="vertical" size="middle" className="project-row">
        <Space.Compact>
          <Input
            placeholder={t("project.namePlaceholder")}
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
            {t("common.create")}
          </Button>
        </Space.Compact>
        <Space size="middle" wrap>
          <Select
            placeholder={t("project.selectToDelete")}
            loading={loadingProjects}
            value={selectedProject ?? undefined}
            onChange={onProjectChange}
            options={projects.map((p) => ({
              value: p.id,
              label: p.isInbox ? t("common.inbox") : p.name,
            }))}
            style={{ minWidth: 240 }}
          />
          <Popconfirm
            title={t("project.deleteConfirm")}
            onConfirm={onDeleteProject}
            disabled={!selectedProject}
          >
            <Button danger loading={deletingProject} disabled={!selectedProject}>
              {t("project.deleteButton")}
            </Button>
          </Popconfirm>
        </Space>
      </Space>
    </Card>
  );
}

export default ProjectManagerCard;
