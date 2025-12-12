import {
  CheckCircleOutlined,
  DeleteOutlined,
  FolderAddOutlined,
  HighlightOutlined,
  LoadingOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  ArrowLeftOutlined,
} from "@ant-design/icons";
import {
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  Layout,
  Popconfirm,
  Modal,
  Row,
  Select,
  Space,
  Badge,
  Spin,
  Table,
  Tag,
  Tabs,
  message,
} from "antd";
import axios from "axios";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import "./App.css";

type StatusKey = "new" | "in_progress" | "done";

type TaskComment = {
  id: number;
  taskId: number;
  body: string;
  authorId?: number;
  authorEmail: string;
  createdAt: string;
};

type Task = {
  id: number;
  title: string;
  status: StatusKey;
  description: string;
  projectId: number;
  createdAt: string;
  createdBy: number;
  authorEmail: string;
  comments: TaskComment[];
};

type Project = {
  id: number;
  name: string;
  createdAt: string;
};

type User = {
  id: number;
  email: string;
  role: "admin" | "user" | "blocked";
  projectIds?: number[];
};

const statusOrder: StatusKey[] = ["new", "in_progress", "done"];

const statusMeta: Record<
  StatusKey,
  { label: string; color: string; icon: ReactNode }
> = {
  new: { label: "Новая", color: "blue", icon: <PlusOutlined /> },
  in_progress: {
    label: "В работе",
    color: "gold",
    icon: <PlayCircleOutlined />,
  },
  done: { label: "Готова", color: "green", icon: <CheckCircleOutlined /> },
};

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || "/api",
  withCredentials: true,
});

const formatDate = (value: string) =>
  new Date(value).toLocaleString("ru-RU", {
    timeZone: "Europe/Moscow",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

const columnDescriptions: Record<StatusKey, string> = {
  new: "Все новые задачи появляются здесь",
  in_progress: "Задачи, над которыми ведется работа",
  done: "Завершенные задачи",
};

function StatusColumn({
  status,
  tasks,
  onSelectTask,
}: {
  status: StatusKey;
  tasks: Task[];
  onSelectTask: (taskId: number) => void;
}) {
  const handleCardClick = (
    event: React.MouseEvent<HTMLElement>,
    taskId: number,
  ) => {
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
                <span className="meta-value">
                  {task.authorEmail || "Не указан"}
                </span>
              </div>
            </Space>
          </Card>
        ))}
      </Space>
    </Card>
  );
}

function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [selectedProject, setSelectedProject] = useState<number | null>(null);
  const [newProjectName, setNewProjectName] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadingProjects, setLoadingProjects] = useState(true);
  const [creating, setCreating] = useState(false);
  const [creatingProject, setCreatingProject] = useState(false);
  const [updatingId, setUpdatingId] = useState<number | null>(null);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [descriptionDraft, setDescriptionDraft] = useState("");
  const [commentDraft, setCommentDraft] = useState("");
  const [addingComment, setAddingComment] = useState(false);
  const [deletingTaskId, setDeletingTaskId] = useState<number | null>(null);
  const [deletingCommentId, setDeletingCommentId] = useState<number | null>(
    null,
  );
  const [deletingProject, setDeletingProject] = useState(false);
  const [user, setUser] = useState<User | null>(null);
  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [authLoading, setAuthLoading] = useState(false);
  const [authError, setAuthError] = useState("");
  const [activePage, setActivePage] = useState<"board" | "settings">("board");
  const [users, setUsers] = useState<User[]>([]);
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [updatingUserId, setUpdatingUserId] = useState<number | null>(null);
  const [passwordModalUser, setPasswordModalUser] = useState<User | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const [updatingPassword, setUpdatingPassword] = useState(false);
  const [updatingProjectsId, setUpdatingProjectsId] = useState<number | null>(
    null,
  );
  const [creatingUser, setCreatingUser] = useState(false);
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const [taskModalOpen, setTaskModalOpen] = useState(false);
  const [projectTaskCounts, setProjectTaskCounts] = useState<
    Record<number, number>
  >({});

  const fetchMe = async () => {
    try {
      const response = await api.get<User>("/auth/me");
      setUser(response.data);
    } catch (error) {
      setUser(null);
    }
  };

  const handleAuth = async (
    email: string,
    password: string,
    mode: "login" | "register",
  ) => {
    setAuthLoading(true);
    setAuthError("");
    try {
      const response =
        mode === "login"
          ? await api.post<User>("/auth/login", { email, password })
          : await api.post<User>("/auth/register", { email, password });
      setUser(response.data);
      message.success(
        mode === "login" ? "Вход выполнен" : "Регистрация успешна",
      );
      void loadProjects();
      void loadTasks();
    } catch (error) {
      console.error(error);
      if (axios.isAxiosError(error)) {
        if (error.response?.status === 403 && mode === "register") {
          setAuthError("Регистрация отключена");
        } else if (error.response?.status === 401) {
          setAuthError("Неверные учетные данные");
        } else if (error.response?.data) {
          setAuthError(
            typeof error.response.data === "string"
              ? error.response.data
              : "Ошибка",
          );
        } else {
          setAuthError("Ошибка запроса");
        }
      } else {
        setAuthError("Ошибка запроса");
      }
    } finally {
      setAuthLoading(false);
    }
  };

  const handleLogout = async () => {
    try {
      await api.post("/auth/logout");
    } catch (error) {
      console.error(error);
    }
    setUser(null);
    setTasks([]);
    setProjects([]);
    setSelectedProject(null);
    setDescription("");
    setDescriptionDraft("");
    setCommentDraft("");
    setAddingComment(false);
    setEditingId(null);
    setSelectedTaskId(null);
  };

  const grouped = useMemo(() => {
    const bucket: Record<StatusKey, Task[]> = {
      new: [],
      in_progress: [],
      done: [],
    };
    tasks.forEach((task) => bucket[task.status].push(task));
    return bucket;
  }, [tasks]);

  const selectedTask = useMemo(
    () => tasks.find((task) => task.id === selectedTaskId) ?? null,
    [tasks, selectedTaskId],
  );

  const loadTasks = async (projectId = selectedProject) => {
    if (!user || !projectId) {
      setLoading(false);
      setTasks([]);
      setSelectedTaskId(null);
      return;
    }
    setLoading(true);
    try {
      const response = await api.get<Task[]>("/tasks", {
        params: { projectId },
      });
      const normalized = response.data.map((task) => ({
        ...task,
        comments: task.comments ?? [],
      }));
      setTasks(normalized);
      if (projectId) {
        setProjectTaskCounts((prev) => ({
          ...prev,
          [projectId]: normalized.length,
        }));
      }
      if (
        selectedTaskId &&
        !normalized.some((task) => task.id === selectedTaskId)
      ) {
        setSelectedTaskId(null);
      }
    } catch (error) {
      console.error(error);
      message.error("Не удалось загрузить задачи");
    } finally {
      setLoading(false);
    }
  };

  const loadProjects = async () => {
    if (!user) {
      setProjects([]);
      setSelectedProject(null);
      setLoadingProjects(false);
      return;
    }
    setLoadingProjects(true);
    try {
      const response = await api.get<Project[]>("/projects");
      setProjects(response.data);
      if (
        selectedProject &&
        response.data.some((p) => p.id === selectedProject)
      ) {
        // keep current
      } else if (response.data.length > 0) {
        setSelectedProject(response.data[0].id);
      } else {
        setSelectedProject(null);
      }
    } catch (error) {
      console.error(error);
      message.error("Не удалось загрузить проекты");
    } finally {
      setLoadingProjects(false);
    }
  };

  useEffect(() => {
    void fetchMe();
  }, []);

  useEffect(() => {
    void loadProjects();
  }, [user]);

  useEffect(() => {
    void loadTasks();
  }, [selectedProject, user]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (activePage !== "board" || !selectedProject || !user) {
        return;
      }
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || target?.isContentEditable) {
        return;
      }
      if ((e.altKey || e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "n") {
        e.preventDefault();
        setTaskModalOpen(true);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [activePage, selectedProject, user]);

  useEffect(() => {
    if (activePage === "settings" && user?.role === "admin") {
      void loadUsers();
    }
  }, [activePage, user]);

  const handleCreate = async () => {
    if (!user) {
      message.warning("Авторизуйтесь, чтобы создавать задачи");
      return;
    }
    if (!selectedProject) {
      message.warning("Сначала выберите проект");
      return;
    }
    if (!title.trim()) {
      message.warning("Введите название задачи");
      return;
    }
    setCreating(true);
    try {
      const response = await api.post<Task>("/tasks", {
        title,
        description,
        projectId: selectedProject,
      });
      setTasks((prev) => [
        { ...response.data, comments: response.data.comments ?? [] },
        ...prev,
      ]);
      setTitle("");
      setDescription("");
      setTaskModalOpen(false);
      message.success("Задача создана");
    } catch (error) {
      console.error(error);
      message.error("Не удалось создать задачу");
    } finally {
      setCreating(false);
    }
  };

  const changeStatus = async (taskId: number, status: StatusKey) => {
    setUpdatingId(taskId);
    try {
      const response = await api.patch<Task>(`/tasks/${taskId}/status`, {
        status,
      });
      const updatedTask = {
        ...response.data,
        comments: response.data.comments ?? [],
      };
      setTasks((prev) =>
        prev.map((task) => (task.id === taskId ? updatedTask : task)),
      );
      message.success("Статус обновлен");
    } catch (error) {
      console.error(error);
      message.error("Не удалось обновить статус");
    } finally {
      setUpdatingId(null);
    }
  };

  const startEditDescription = (task: Task) => {
    setEditingId(task.id);
    setDescriptionDraft(task.description || "");
  };

  const cancelEditDescription = () => {
    setEditingId(null);
    setDescriptionDraft(selectedTask?.description ?? "");
  };

  const saveDescription = async (taskId: number) => {
    setUpdatingId(taskId);
    try {
      const response = await api.patch<Task>(`/tasks/${taskId}`, {
        description: descriptionDraft,
      });
      setTasks((prev) =>
        prev.map((task) =>
          task.id === taskId
            ? { ...response.data, comments: response.data.comments ?? [] }
            : task,
        ),
      );
      setEditingId(null);
      setDescriptionDraft("");
      message.success("Описание обновлено");
    } catch (error) {
      console.error(error);
      message.error("Не удалось обновить описание");
    } finally {
      setUpdatingId(null);
    }
  };

  const loadTaskComments = async (taskId: number) => {
    try {
      const response = await api.get<TaskComment[]>(
        `/tasks/${taskId}/comments`,
      );
      setTasks((prev) =>
        prev.map((task) =>
          task.id === taskId
            ? { ...task, comments: response.data ?? [] }
            : task,
        ),
      );
    } catch (error) {
      console.error(error);
      message.error("Не удалось загрузить комментарии");
    }
  };

  const openTaskDetails = (taskId: number) => {
    setSelectedTaskId(taskId);
    const currentTask = tasks.find((task) => task.id === taskId);
    setDescriptionDraft(currentTask?.description ?? "");
    setCommentDraft("");
    setEditingId(null);
    void loadTaskComments(taskId);
  };

  const closeTaskDetails = () => {
    setSelectedTaskId(null);
    setCommentDraft("");
    setEditingId(null);
    setDescriptionDraft("");
  };

  const addComment = async () => {
    if (!selectedTask) {
      return;
    }
    const text = commentDraft.trim();
    if (!text) {
      message.warning("Введите текст комментария");
      return;
    }
    setAddingComment(true);
    try {
      const response = await api.post<TaskComment>(
        `/tasks/${selectedTask.id}/comments`,
        { body: text },
      );
      setTasks((prev) =>
        prev.map((task) =>
          task.id === selectedTask.id
            ? {
                ...task,
                comments: [...(task.comments ?? []), response.data],
              }
            : task,
        ),
      );
      setCommentDraft("");
      message.success("Комментарий добавлен");
    } catch (error) {
      console.error(error);
      message.error("Не удалось добавить комментарий");
    } finally {
      setAddingComment(false);
    }
  };

  const deleteTask = async (taskId: number) => {
    setDeletingTaskId(taskId);
    try {
      await api.delete(`/tasks/${taskId}`);
      setTasks((prev) => prev.filter((t) => t.id !== taskId));
      if (selectedTaskId === taskId) {
        closeTaskDetails();
      }
      message.success("Задача удалена");
    } catch (error) {
      console.error(error);
      message.error("Не удалось удалить задачу");
    } finally {
      setDeletingTaskId(null);
    }
  };

  const deleteComment = async (commentId: number) => {
    if (!selectedTask) {
      return;
    }
    setDeletingCommentId(commentId);
    try {
      await api.delete(`/tasks/${selectedTask.id}/comments/${commentId}`);
      setTasks((prev) =>
        prev.map((task) =>
          task.id === selectedTask.id
            ? {
                ...task,
                comments: (task.comments ?? []).filter(
                  (comment) => comment.id !== commentId,
                ),
              }
            : task,
        ),
      );
      message.success("Комментарий удален");
    } catch (error) {
      console.error(error);
      message.error("Не удалось удалить комментарий");
    } finally {
      setDeletingCommentId(null);
    }
  };

  const handleProjectChange = (projectId: number) => {
    setSelectedProject(projectId);
    setSelectedTaskId(null);
  };

  const handleCreateProject = async () => {
    if (!user) {
      message.warning("Авторизуйтесь, чтобы создавать проекты");
      return;
    }
    const name = newProjectName.trim();
    if (!name) {
      message.warning("Введите название проекта");
      return;
    }
    setCreatingProject(true);
    try {
      const response = await api.post<Project>("/projects", { name });
      setProjects((prev) => [response.data, ...prev]);
      setSelectedProject(response.data.id);
      setNewProjectName("");
      message.success("Проект создан");
    } catch (error) {
      console.error(error);
      if (axios.isAxiosError(error) && error.response?.status === 400) {
        message.error("Проект с таким названием уже существует");
      } else {
        message.error("Не удалось создать проект");
      }
    } finally {
      setCreatingProject(false);
    }
  };

  const handleDeleteProject = async () => {
    if (!selectedProject) {
      return;
    }
    if (user?.role !== "admin") {
      message.error("Удаление проектов доступно только администратору");
      return;
    }
    setDeletingProject(true);
    try {
      await api.delete(`/projects/${selectedProject}`);
      const next = projects.filter((p) => p.id !== selectedProject);
      setProjects(next);
      if (next.length > 0) {
        setSelectedProject(next[0].id);
      } else {
        setSelectedProject(null);
        setTasks([]);
      }
      message.success("Проект удален");
    } catch (error) {
      console.error(error);
      if (axios.isAxiosError(error) && error.response?.status === 400) {
        message.error("Нельзя удалить проект по умолчанию");
      } else {
        message.error("Не удалось удалить проект");
      }
    } finally {
      setDeletingProject(false);
    }
  };

  const loadUsers = async () => {
    if (user?.role !== "admin") {
      setUsers([]);
      return;
    }
    setLoadingUsers(true);
    try {
      const response = await api.get<User[]>("/users");
      setUsers(response.data);
    } catch (error) {
      console.error(error);
      message.error("Не удалось загрузить пользователей");
    } finally {
      setLoadingUsers(false);
    }
  };

  const handleUserRoleChange = async (userId: number, role: User["role"]) => {
    setUpdatingUserId(userId);
    try {
      const response = await api.patch<User>(`/users/${userId}`, { role });
      setUsers((prev) =>
        prev.map((u) =>
          u.id === userId
            ? {
                ...response.data,
                projectIds: response.data.projectIds ?? u.projectIds,
              }
            : u,
        ),
      );
      message.success("Роль обновлена");
      if (response.data.id === user?.id) {
        setUser({
          ...user,
          role: response.data.role,
          projectIds: response.data.projectIds ?? user.projectIds,
        });
        if (response.data.role !== "admin") {
          setActivePage("board");
        }
      }
      if (response.data.id === user?.id && response.data.role === "blocked") {
        await handleLogout();
      }
    } catch (error) {
      console.error(error);
      if (axios.isAxiosError(error) && error.response?.data) {
        const errMsg =
          typeof error.response.data === "string"
            ? error.response.data
            : "Не удалось обновить роль";
        message.error(errMsg);
      } else {
        message.error("Не удалось обновить роль");
      }
    } finally {
      setUpdatingUserId(null);
    }
  };

  const handleUserProjectsChange = async (
    userId: number,
    projectIds: number[],
  ) => {
    setUpdatingProjectsId(userId);
    try {
      const response = await api.patch<User>(`/users/${userId}`, {
        projectIds,
      });
      setUsers((prev) =>
        prev.map((u) =>
          u.id === userId
            ? { ...u, projectIds: response.data.projectIds ?? [] }
            : u,
        ),
      );
      message.success("Доступы обновлены");
      if (response.data.id === user?.id) {
        setUser({ ...user, projectIds: response.data.projectIds ?? [] });
        void loadProjects();
      }
    } catch (error) {
      console.error(error);
      message.error("Не удалось обновить доступы к проектам");
    } finally {
      setUpdatingProjectsId(null);
    }
  };

  const handleCreateUser = async (values: {
    email: string;
    password: string;
    role: User["role"];
  }) => {
    setCreatingUser(true);
    try {
      const response = await api.post<User>("/users", values);
      setUsers((prev) => [response.data, ...prev]);
      message.success("Пользователь создан");
    } catch (error) {
      console.error(error);
      if (axios.isAxiosError(error) && error.response?.data) {
        const errMsg =
          typeof error.response.data === "string"
            ? error.response.data
            : "Не удалось создать пользователя";
        message.error(errMsg);
      } else {
        message.error("Не удалось создать пользователя");
      }
    } finally {
      setCreatingUser(false);
    }
  };

  const openPasswordModal = (user: User) => {
    setPasswordModalUser(user);
    setNewPassword("");
  };

  const handlePasswordChange = async () => {
    if (!passwordModalUser) return;
    if (newPassword.trim().length < 6) {
      message.error("Пароль должен быть не меньше 6 символов");
      return;
    }
    setUpdatingPassword(true);
    try {
      await api.patch<User>(`/users/${passwordModalUser.id}`, {
        password: newPassword.trim(),
      });
      message.success("Пароль обновлен");
      setPasswordModalUser(null);
      setNewPassword("");
    } catch (error) {
      console.error(error);
      message.error("Не удалось обновить пароль");
    } finally {
      setUpdatingPassword(false);
    }
  };

  if (!user) {
    return (
      <Layout className="layout">
        <Layout.Header className="header">
          <div className="brand">LiteTask</div>
        </Layout.Header>
        <Layout.Content className="content">
          <Card className="auth-card" title="Войдите или зарегистрируйтесь">
            <Tabs
              activeKey={authMode}
              onChange={(key) => setAuthMode(key as "login" | "register")}
              items={[
                { key: "login", label: "Вход" },
                { key: "register", label: "Регистрация" },
              ]}
            />
            <Form
              layout="vertical"
              onFinish={(values) =>
                handleAuth(values.email, values.password, authMode)
              }
            >
              <Form.Item
                name="email"
                label="Email"
                rules={[{ required: true, type: "email" }]}
              >
                <Input placeholder="you@example.com" />
              </Form.Item>
              <Form.Item
                name="password"
                label="Пароль"
                rules={[
                  { required: true },
                  ...(authMode === "register"
                    ? [{ min: 6, message: "Минимум 6 символов" }]
                    : []),
                ]}
              >
                <Input.Password placeholder="••••••" />
              </Form.Item>
              {authError && <div className="auth-error">{authError}</div>}
              <Button
                type="primary"
                htmlType="submit"
                block
                loading={authLoading}
              >
                {authMode === "login" ? "Войти" : "Зарегистрироваться"}
              </Button>
            </Form>
          </Card>
        </Layout.Content>
      </Layout>
    );
  }

  return (
    <Layout className="layout">
      <Layout.Header className="header">
        <div className="brand">LiteTask</div>
        <Space>
          {user.role === "admin" && (
            <Button
              type="default"
              onClick={() =>
                setActivePage((prev) =>
                  prev === "board" ? "settings" : "board",
                )
              }
            >
              {activePage === "settings" ? "К задачам" : "Настройки"}
            </Button>
          )}
          <Tag color={user.role === "admin" ? "green" : "blue"}>
            {user.role === "admin" ? "Админ" : "Пользователь"}
          </Tag>
          <span className="user-email">{user.email}</span>
          <Button
            type="default"
            icon={<ReloadOutlined />}
            onClick={() => loadTasks()}
          >
            Обновить
          </Button>
          <Button onClick={handleLogout}>Выйти</Button>
        </Space>
      </Layout.Header>
      <Layout.Content className="content">
        {activePage === "settings" && user.role === "admin" ? (
          <>
            <Card className="project-card" title="Управление проектами">
              <Space direction="vertical" size="middle" className="project-row">
                <Space.Compact>
                  <Input
                    placeholder="Новый проект"
                    value={newProjectName}
                    onChange={(e) => setNewProjectName(e.target.value)}
                    onPressEnter={handleCreateProject}
                  />
                  <Button
                    type="primary"
                    icon={<FolderAddOutlined />}
                    onClick={handleCreateProject}
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
                    onChange={handleProjectChange}
                    options={projects.map((p) => ({
                      value: p.id,
                      label: p.name,
                    }))}
                    style={{ minWidth: 240 }}
                  />
                  <Popconfirm
                    title="Удалить проект и все его задачи?"
                    onConfirm={handleDeleteProject}
                    disabled={!selectedProject}
                  >
                    <Button
                      danger
                      loading={deletingProject}
                      disabled={!selectedProject}
                    >
                      Удалить проект
                    </Button>
                  </Popconfirm>
                </Space>
              </Space>
            </Card>
            <Card className="create-card" title="Пользователи">
              <Form
                layout="inline"
                onFinish={handleCreateUser}
                style={{ marginBottom: 12 }}
                initialValues={{ role: "user" }}
              >
                <Form.Item
                  name="email"
                  rules={[
                    { required: true, type: "email", message: "Введите email" },
                  ]}
                >
                  <Input placeholder="email@example.com" />
                </Form.Item>
                <Form.Item
                  name="password"
                  rules={[
                    { required: true, min: 6, message: "Минимум 6 символов" },
                  ]}
                >
                  <Input.Password placeholder="Пароль" />
                </Form.Item>
                <Form.Item name="role" rules={[{ required: true }]}>
                  <Select
                    style={{ width: 140 }}
                    options={[
                      { label: "Пользователь", value: "user" },
                      { label: "Админ", value: "admin" },
                      { label: "Заблокирован", value: "blocked" },
                    ]}
                  />
                </Form.Item>
                <Form.Item>
                  <Button
                    type="primary"
                    htmlType="submit"
                    loading={creatingUser}
                  >
                    Добавить пользователя
                  </Button>
                </Form.Item>
              </Form>
              <Button
                onClick={() => void loadUsers()}
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
                    title: "Проекты",
                    dataIndex: "projectIds",
                    render: (_: unknown, record) => (
                      <Select
                        mode="multiple"
                        style={{ minWidth: 240 }}
                        value={record.projectIds ?? []}
                        onChange={(value) =>
                          void handleUserProjectsChange(
                            record.id,
                            value.map((v) => Number(v)),
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
                          onClick={() =>
                            handleUserRoleChange(record.id, "admin")
                          }
                          loading={updatingUserId === record.id}
                          disabled={record.role === "admin"}
                        >
                          Сделать админом
                        </Button>
                        <Button
                          size="small"
                          onClick={() =>
                            handleUserRoleChange(record.id, "user")
                          }
                          loading={updatingUserId === record.id}
                          disabled={record.role === "user"}
                        >
                          Сделать пользователем
                        </Button>
                        <Button
                          size="small"
                          danger={record.role !== "blocked"}
                          onClick={() =>
                            handleUserRoleChange(
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
                        <Button
                          size="small"
                          onClick={() => openPasswordModal(record)}
                        >
                          Сменить пароль
                        </Button>
                      </Space>
                    ),
                  },
                ]}
              />
            </Card>
          </>
        ) : (
          <>
            <Card className="project-card project-card--compact">
              <div className="project-card-header">
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={() => setTaskModalOpen(true)}
                  disabled={!selectedProject}
                  title="Alt+N / ⌘+N"
                >
                  Добавить задачу
                </Button>
                <div className="project-card-title">Проекты</div>
              </div>
              {projects.length === 0 && !loadingProjects ? (
                <Empty description="Нет проектов. Создайте их в настройках" />
              ) : (
                <Tabs
                  activeKey={
                    selectedProject ? String(selectedProject) : undefined
                  }
                  onChange={(key) => handleProjectChange(Number(key))}
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

            {projects.length === 0 && !loadingProjects ? (
              <Empty description="Создайте проект, чтобы начать" />
            ) : loading ? (
              <div className="loader">
                <Spin tip="Загрузка задач..." size="large" />
              </div>
            ) : selectedTask ? (
              <Card
                className="task-details-card"
                title={
                  <Space size="middle">
                    <Button
                      type="link"
                      icon={<ArrowLeftOutlined />}
                      onClick={closeTaskDetails}
                    >
                      К списку задач
                    </Button>
                    <span className="detail-title">{selectedTask.title}</span>
                  </Space>
                }
                extra={
                  <Space size="small" wrap>
                    <Tag color={statusMeta[selectedTask.status].color}>
                      {statusMeta[selectedTask.status].label}
                    </Tag>
                    <Select
                      size="small"
                      value={selectedTask.status}
                      onChange={(value) =>
                        changeStatus(selectedTask.id, value as StatusKey)
                      }
                      dropdownMatchSelectWidth={false}
                      options={statusOrder.map((s) => ({
                        label: statusMeta[s].label,
                        value: s,
                      }))}
                      suffixIcon={
                        updatingId === selectedTask.id ? (
                          <LoadingOutlined spin />
                        ) : (
                          <HighlightOutlined />
                        )
                      }
                      style={{ minWidth: 180 }}
                    />
                    <Popconfirm
                      title="Удалить задачу?"
                      onConfirm={() => void deleteTask(selectedTask.id)}
                    >
                      <Button
                        icon={<DeleteOutlined />}
                        danger
                        loading={deletingTaskId === selectedTask.id}
                      >
                        Удалить
                      </Button>
                    </Popconfirm>
                  </Space>
                }
              >
                <Space
                  direction="vertical"
                  size="large"
                  className="detail-content"
                >
                  <Space size="large" className="detail-meta" wrap>
                    <div className="meta-row">
                      <span className="meta-label">Создана:</span>
                      <span className="meta-value">
                        {formatDate(selectedTask.createdAt)}
                      </span>
                    </div>
                    <div className="meta-row">
                      <span className="meta-label">Автор:</span>
                      <span className="meta-value">
                        {selectedTask.authorEmail || "Не указан"}
                      </span>
                    </div>
                  </Space>

                  <div className="detail-section">
                    <div className="section-header">
                      <span className="meta-label">Описание</span>
                    </div>
                    {editingId === selectedTask.id ? (
                      <Space
                        direction="vertical"
                        size="small"
                        className="description-edit"
                      >
                        <Input.TextArea
                          value={descriptionDraft}
                          onChange={(e) => setDescriptionDraft(e.target.value)}
                          autoSize={{ minRows: 3, maxRows: 6 }}
                          placeholder="Опишите детали задачи"
                        />
                        <Space size="small">
                          <Button
                            type="primary"
                            onClick={() => saveDescription(selectedTask.id)}
                            loading={updatingId === selectedTask.id}
                          >
                            Сохранить
                          </Button>
                          <Button onClick={cancelEditDescription}>
                            Отмена
                          </Button>
                        </Space>
                      </Space>
                    ) : (
                      <div className="description-display">
                        <p
                          className={
                            selectedTask.description
                              ? "meta-value"
                              : "muted-text"
                          }
                        >
                          {selectedTask.description || "Описание отсутствует"}
                        </p>
                        <Button
                          type="link"
                          onClick={() => startEditDescription(selectedTask)}
                        >
                          Изменить
                        </Button>
                      </div>
                    )}
                  </div>

                  <div className="detail-section">
                    <div className="comments-header">
                      <span className="meta-label">Комментарии</span>
                      <Tag color="default">
                        {selectedTask.comments?.length ?? 0}
                      </Tag>
                    </div>
                    {selectedTask.comments &&
                    selectedTask.comments.length > 0 ? (
                      <Space
                        direction="vertical"
                        size="small"
                        className="comments-list"
                      >
                        {selectedTask.comments.map((comment) => (
                          <div key={comment.id} className="comment-item">
                            <div className="comment-meta">
                              <span>{formatDate(comment.createdAt)}</span>
                              <Space size="small">
                                <span>
                                  {comment.authorEmail || "Не указан"}
                                </span>
                                {user &&
                                  (user.role === "admin" ||
                                    comment.authorId === user.id) && (
                                    <Popconfirm
                                      title="Удалить комментарий?"
                                      onConfirm={() =>
                                        void deleteComment(comment.id)
                                      }
                                    >
                                      <Button
                                        type="link"
                                        size="small"
                                        danger
                                        loading={
                                          deletingCommentId === comment.id
                                        }
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
                        onChange={(e) => setCommentDraft(e.target.value)}
                        autoSize={{ minRows: 3, maxRows: 4 }}
                        placeholder="Новый комментарий"
                      />
                      <Button
                        type="primary"
                        onClick={() => void addComment()}
                        loading={addingComment}
                        disabled={!commentDraft.trim()}
                      >
                        Добавить комментарий
                      </Button>
                    </div>
                  </div>
                </Space>
              </Card>
            ) : (
              <Row gutter={[16, 16]} className="board">
                {statusOrder.map((status) => (
                  <Col key={status} xs={24} md={8}>
                    <StatusColumn
                      status={status}
                      tasks={grouped[status]}
                      onSelectTask={openTaskDetails}
                    />
                  </Col>
                ))}
              </Row>
            )}
          </>
        )}
      </Layout.Content>
      <Modal
        title="Новая задача"
        open={taskModalOpen}
        onCancel={() => setTaskModalOpen(false)}
        onOk={handleCreate}
        okButtonProps={{ loading: creating, disabled: !selectedProject }}
        cancelText="Отмена"
        okText="Создать"
      >
        <Space direction="vertical" size="small" className="create-column">
          <Input
            placeholder="Новая задача"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            onPressEnter={handleCreate}
            maxLength={140}
          />
          <Input.TextArea
            placeholder="Описание задачи (необязательно)"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            autoSize={{ minRows: 3, maxRows: 5 }}
          />
        </Space>
      </Modal>

      <Modal
        title={
          passwordModalUser
            ? `Смена пароля: ${passwordModalUser.email}`
            : "Смена пароля"
        }
        open={!!passwordModalUser}
        okText="Обновить пароль"
        cancelText="Отмена"
        onOk={() => void handlePasswordChange()}
        onCancel={() => setPasswordModalUser(null)}
        confirmLoading={updatingPassword}
      >
        <Input.Password
          placeholder="Новый пароль"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          onPressEnter={() => void handlePasswordChange()}
        />
        <div className="auth-error" style={{ marginTop: 8 }}>
          Минимум 6 символов
        </div>
      </Modal>
    </Layout>
  );
}

export default App;
