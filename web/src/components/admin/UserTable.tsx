import { Button, Select, Space, Table, Tag } from "antd";

import type { Project, User } from "../../types";

type UserTableProps = {
  users: User[];
  projects: Project[];
  loadingUsers: boolean;
  loadingProjects: boolean;
  updatingUserId: number | null;
  updatingProjectsId: number | null;
  onRefresh: () => void;
  onUserRoleChange: (userId: number, role: User["role"]) => void;
  onUserProjectsChange: (userId: number, projectIds: number[]) => void;
  onOpenUserInfo: (user: User) => void;
  onOpenPassword: (user: User) => void;
};

function UserTable({
  users,
  projects,
  loadingUsers,
  loadingProjects,
  updatingUserId,
  updatingProjectsId,
  onRefresh,
  onUserRoleChange,
  onUserProjectsChange,
  onOpenUserInfo,
  onOpenPassword,
}: UserTableProps) {
  return (
    <>
      <Button
        onClick={onRefresh}
        style={{ marginBottom: 12 }}
        loading={loadingUsers}
      >
        Обновить список
      </Button>
      <Table<User>
        dataSource={users}
        rowKey="id"
        pagination={false}
        loading={loadingUsers}
        columns={[
          {
            title: "Email",
            dataIndex: "email",
          },
          {
            title: "Юзернейм",
            dataIndex: "username",
            render: (value: string | undefined) => value || "—",
          },
          {
            title: "Имя",
            dataIndex: "firstName",
            render: (value: string | undefined) => value || "—",
          },
          {
            title: "Фамилия",
            dataIndex: "lastName",
            render: (value: string | undefined) => value || "—",
          },
          {
            title: "Проекты",
            dataIndex: "projectIds",
            render: (_: unknown, record) => (
              <Select
                mode="multiple"
                style={{ minWidth: 240 }}
                value={record.projectIds ?? []}
                onChange={(value) =>
                  onUserProjectsChange(
                    record.id,
                    value.map((entry) => Number(entry)),
                  )
                }
                options={projects.map((p) => ({
                  value: p.id,
                  label: p.name,
                }))}
                loading={loadingProjects}
                disabled={updatingProjectsId === record.id}
              />
            ),
          },
          {
            title: "Роль",
            dataIndex: "role",
            render: (role: User["role"]) => (
              <Tag
                color={
                  role === "admin"
                    ? "green"
                    : role === "blocked"
                      ? "red"
                      : "blue"
                }
              >
                {role}
              </Tag>
            ),
          },
          {
            title: "Действия",
            render: (_, record) => (
              <Space size="small" wrap>
                <Button
                  size="small"
                  type="default"
                  onClick={() => onUserRoleChange(record.id, "admin")}
                  loading={updatingUserId === record.id}
                  disabled={record.role === "admin"}
                >
                  Сделать админом
                </Button>
                <Button
                  size="small"
                  onClick={() => onUserRoleChange(record.id, "user")}
                  loading={updatingUserId === record.id}
                  disabled={record.role === "user"}
                >
                  Сделать пользователем
                </Button>
                <Button
                  size="small"
                  danger={record.role !== "blocked"}
                  onClick={() =>
                    onUserRoleChange(
                      record.id,
                      record.role === "blocked" ? "user" : "blocked",
                    )
                  }
                  loading={updatingUserId === record.id}
                >
                  {record.role === "blocked"
                    ? "Разблокировать"
                    : "Заблокировать"}
                </Button>
                <Button size="small" onClick={() => onOpenUserInfo(record)}>
                  Изменить имя
                </Button>
                <Button size="small" onClick={() => onOpenPassword(record)}>
                  Сменить пароль
                </Button>
              </Space>
            ),
          },
        ]}
      />
    </>
  );
}

export default UserTable;
