import { Card, Empty, Layout, Spin, message } from "antd";
import axios from "axios";
import { useCallback, useEffect, useMemo, useState } from "react";
import "./App.css";
import api from "./api";
import { AUTO_REFRESH_INTERVAL_STORAGE_KEY } from "./constants";
import AuthCard from "./components/auth/AuthCard";
import PasswordModal from "./components/admin/PasswordModal";
import UserForm from "./components/admin/UserForm";
import UserInfoModal from "./components/admin/UserInfoModal";
import UserTable from "./components/admin/UserTable";
import Header from "./components/layout/Header";
import MobileNav from "./components/layout/MobileNav";
import ProjectManagerCard from "./components/projects/ProjectManagerCard";
import ProjectTabs from "./components/projects/ProjectTabs";
import ProfileModal from "./components/profile/ProfileModal";
import QuickAddPage from "./components/quick/QuickAddPage";
import Board from "./components/tasks/Board";
import CreateTaskModal from "./components/tasks/CreateTaskModal";
import TaskDetailCard from "./components/tasks/TaskDetailCard";
import type {
  AutoRefreshIntervalMs,
  Project,
  StatusKey,
  Task,
  TaskComment,
  User,
} from "./types";

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
  const [quickTitle, setQuickTitle] = useState("");
  const [quickDescription, setQuickDescription] = useState("");
  const [quickCreating, setQuickCreating] = useState(false);
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
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [activePage, setActivePage] = useState<"board" | "settings" | "quick">(
    "quick",
  );
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
  const [profileModalOpen, setProfileModalOpen] = useState(false);
  const [profilePassword, setProfilePassword] = useState("");
  const [profileTelegram, setProfileTelegram] = useState("");
  const [profileFirstName, setProfileFirstName] = useState("");
  const [profileLastName, setProfileLastName] = useState("");
  const [profileUsername, setProfileUsername] = useState("");
  const [profileSaving, setProfileSaving] = useState(false);
  const [editingUserInfo, setEditingUserInfo] = useState<User | null>(null);
  const [editingUserFirstName, setEditingUserFirstName] = useState("");
  const [editingUserLastName, setEditingUserLastName] = useState("");
  const [savingUserInfo, setSavingUserInfo] = useState(false);
  const [autoRefreshIntervalMs, setAutoRefreshIntervalMs] =
    useState<AutoRefreshIntervalMs | null>(60_000);

  const fetchMe = async () => {
    try {
      const response = await api.get<User>("/auth/me");
      setUser(response.data);
    } catch {
      setUser(null);
    }
  };

  const handleAuth = async (
    email: string,
    password: string,
    mode: "login" | "register",
    username?: string,
    firstName?: string,
    lastName?: string,
  ) => {
    setAuthLoading(true);
    setAuthError("");
    try {
      const response =
        mode === "login"
          ? await api.post<User>("/auth/login", { login: email, password })
          : await api.post<User>("/auth/register", {
              email,
              username: username?.trim() ?? "",
              password,
              firstName: firstName?.trim() ?? "",
              lastName: lastName?.trim() ?? "",
            });
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
    setProfileModalOpen(false);
    setProfilePassword("");
    setProfileTelegram("");
    setProfileUsername("");
    setQuickTitle("");
    setQuickDescription("");
    setQuickCreating(false);
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

  const loadTasks = useCallback(
    async (projectId = selectedProject, options?: { silent?: boolean }) => {
      if (!user || !projectId) {
        if (!options?.silent) {
          setLoading(false);
          setTasks([]);
          setSelectedTaskId(null);
        }
        return;
      }
      if (!options?.silent) {
        setLoading(true);
      }
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
        if (!options?.silent) {
          message.error("Не удалось загрузить задачи");
        }
      } finally {
        if (!options?.silent) {
          setLoading(false);
        }
      }
    },
    [selectedProject, selectedTaskId, user],
  );

  const loadProjects = useCallback(async () => {
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
  }, [selectedProject, user]);

  useEffect(() => {
    void fetchMe();
  }, []);

  useEffect(() => {
    const saved = localStorage.getItem(AUTO_REFRESH_INTERVAL_STORAGE_KEY);
    if (!saved) {
      return;
    }
    if (saved === "off") {
      setAutoRefreshIntervalMs(null);
      return;
    }
    const parsed = Number(saved);
    if (
      parsed === 5_000 ||
      parsed === 30_000 ||
      parsed === 60_000 ||
      parsed === 300_000
    ) {
      setAutoRefreshIntervalMs(parsed);
    }
  }, []);

  useEffect(() => {
    localStorage.setItem(
      AUTO_REFRESH_INTERVAL_STORAGE_KEY,
      autoRefreshIntervalMs === null ? "off" : String(autoRefreshIntervalMs),
    );
  }, [autoRefreshIntervalMs]);

  useEffect(() => {
    void loadProjects();
  }, [loadProjects, user]);

  useEffect(() => {
    void loadTasks();
  }, [loadTasks, selectedProject, user]);

  useEffect(() => {
    if (autoRefreshIntervalMs === null) {
      return;
    }
    if (activePage !== "board" || !user || !selectedProject) {
      return;
    }
    const intervalId = window.setInterval(() => {
      void loadTasks(selectedProject, { silent: true });
    }, autoRefreshIntervalMs);
    return () => window.clearInterval(intervalId);
  }, [activePage, autoRefreshIntervalMs, loadTasks, selectedProject, user]);

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
      if (e.altKey && e.key.toLowerCase() === "n") {
        e.preventDefault();
        setTaskModalOpen(true);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [activePage, selectedProject, user]);

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

  const handleUpdateProfile = async () => {
    if (!user) return;
    const payload: {
      password?: string;
      telegram?: string;
      firstName?: string;
      lastName?: string;
      username?: string;
    } = {
      telegram: profileTelegram,
      firstName: profileFirstName,
      lastName: profileLastName,
    };
    if (!user.username?.trim() && profileUsername.trim()) {
      payload.username = profileUsername.trim();
    }
    if (profilePassword.trim()) {
      payload.password = profilePassword.trim();
      if (payload.password.length < 6) {
        message.error("Пароль должен быть не меньше 6 символов");
        return;
      }
    }
    try {
      setProfileSaving(true);
      const response = await api.patch<User>("/profile", payload);
      setUser(response.data);
      setProfilePassword("");
      setProfileTelegram(response.data.telegram ?? "");
      setProfileFirstName(response.data.firstName ?? "");
      setProfileLastName(response.data.lastName ?? "");
      setProfileUsername(response.data.username ?? "");
      message.success("Профиль обновлен");
      setProfileModalOpen(false);
    } catch (error) {
      console.error(error);
      if (axios.isAxiosError(error) && error.response?.data) {
        const errMsg =
          typeof error.response.data === "string"
            ? error.response.data
            : "Не удалось обновить профиль";
        message.error(errMsg);
      } else {
        message.error("Не удалось обновить профиль");
      }
    } finally {
      setProfileSaving(false);
    }
  };

  const openUserInfoModal = (target: User) => {
    setEditingUserInfo(target);
    setEditingUserFirstName(target.firstName ?? "");
    setEditingUserLastName(target.lastName ?? "");
  };

  const saveUserInfo = async () => {
    if (!editingUserInfo) return;
    setSavingUserInfo(true);
    try {
      const response = await api.patch<User>(`/users/${editingUserInfo.id}`, {
        firstName: editingUserFirstName,
        lastName: editingUserLastName,
      });
      setUsers((prev) =>
        prev.map((u) =>
          u.id === editingUserInfo.id
            ? {
                ...u,
                ...response.data,
                projectIds: response.data.projectIds ?? u.projectIds,
              }
            : u,
        ),
      );
      if (response.data.id === user?.id) {
        setUser({
          ...user,
          ...response.data,
          projectIds: response.data.projectIds ?? user.projectIds,
        });
      }
      message.success("Данные пользователя обновлены");
      setEditingUserInfo(null);
    } catch (error) {
      console.error(error);
      message.error("Не удалось обновить данные пользователя");
    } finally {
      setSavingUserInfo(false);
    }
  };

  const handleProjectChange = (projectId: number) => {
    setSelectedProject(projectId);
    setSelectedTaskId(null);
  };

  const defaultProjectId = selectedProject ?? projects[0]?.id ?? null;
  const defaultProjectName =
    projects.find((project) => project.id === defaultProjectId)?.name ?? null;

  const handleQuickCreate = async () => {
    if (!user) {
      message.warning("Авторизуйтесь, чтобы создавать задачи");
      return;
    }
    if (!defaultProjectId) {
      message.warning("Нет доступных проектов");
      return;
    }
    if (!quickTitle.trim()) {
      message.warning("Введите тему задачи");
      return;
    }
    setQuickCreating(true);
    try {
      const response = await api.post<Task>("/tasks", {
        title: quickTitle,
        description: quickDescription,
        projectId: defaultProjectId,
      });
      if (defaultProjectId === selectedProject) {
        setTasks((prev) => [
          { ...response.data, comments: response.data.comments ?? [] },
          ...prev,
        ]);
      }
      setQuickTitle("");
      setQuickDescription("");
      message.success("Задача создана");
    } catch (error) {
      console.error(error);
      message.error("Не удалось создать задачу");
    } finally {
      setQuickCreating(false);
    }
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

  const loadUsers = useCallback(async () => {
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
  }, [user]);

  useEffect(() => {
    if (activePage === "settings" && user?.role === "admin") {
      void loadUsers();
    }
  }, [activePage, loadUsers, user]);

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
    username?: string;
    password: string;
    role: User["role"];
    firstName?: string;
    lastName?: string;
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
        <Header brandHref="/" />
        <Layout.Content className="content">
          <AuthCard
            authMode={authMode}
            authError={authError}
            authLoading={authLoading}
            onAuthModeChange={setAuthMode}
            onSubmit={(values, mode) =>
              handleAuth(
                values.email,
                values.password,
                mode,
                values.username,
                values.firstName,
                values.lastName,
              )
            }
          />
        </Layout.Content>
      </Layout>
    );
  }

  return (
    <Layout className="layout">
      <Header
        showMenuButton
        onMenuClick={() => setMobileNavOpen(true)}
        onBrandClick={() => setActivePage("board")}
      />
      <MobileNav
        open={mobileNavOpen}
        onClose={() => setMobileNavOpen(false)}
        user={user}
        activePage={activePage}
        autoRefreshIntervalMs={autoRefreshIntervalMs}
        onAutoRefreshChange={setAutoRefreshIntervalMs}
        onToggleSettings={() => {
          setActivePage((prev) => (prev === "settings" ? "board" : "settings"));
          setMobileNavOpen(false);
        }}
        onOpenProfile={() => {
          setProfileTelegram(user.telegram ?? "");
          setProfileFirstName(user.firstName ?? "");
          setProfileLastName(user.lastName ?? "");
          setProfileUsername(user.username ?? "");
          setProfilePassword("");
          setProfileModalOpen(true);
          setMobileNavOpen(false);
        }}
        onLogout={() => {
          setMobileNavOpen(false);
          handleLogout();
        }}
      />
      <Layout.Content className="content">
        {activePage === "quick" ? (
          <QuickAddPage
            title={quickTitle}
            description={quickDescription}
            creating={quickCreating}
            hasProject={!!defaultProjectId && !loadingProjects}
            defaultProjectName={defaultProjectName}
            onTitleChange={setQuickTitle}
            onDescriptionChange={setQuickDescription}
            onSubmit={handleQuickCreate}
            onGoToBoard={() => setActivePage("board")}
          />
        ) : activePage === "settings" && user.role === "admin" ? (
          <>
            <ProjectManagerCard
              newProjectName={newProjectName}
              onNewProjectNameChange={setNewProjectName}
              onCreateProject={handleCreateProject}
              creatingProject={creatingProject}
              projects={projects}
              loadingProjects={loadingProjects}
              selectedProject={selectedProject}
              onProjectChange={handleProjectChange}
              onDeleteProject={handleDeleteProject}
              deletingProject={deletingProject}
            />
            <Card className="create-card" title="Пользователи">
              <UserForm
                creatingUser={creatingUser}
                onCreateUser={handleCreateUser}
              />
              <UserTable
                users={users}
                projects={projects}
                loadingUsers={loadingUsers}
                loadingProjects={loadingProjects}
                updatingUserId={updatingUserId}
                updatingProjectsId={updatingProjectsId}
                onRefresh={() => void loadUsers()}
                onUserRoleChange={handleUserRoleChange}
                onUserProjectsChange={(userId, projectIds) =>
                  void handleUserProjectsChange(userId, projectIds)
                }
                onOpenUserInfo={openUserInfoModal}
                onOpenPassword={openPasswordModal}
              />
            </Card>
          </>
        ) : (
          <>
            <ProjectTabs
              projects={projects}
              loadingProjects={loadingProjects}
              selectedProject={selectedProject}
              projectTaskCounts={projectTaskCounts}
              onSelectProject={handleProjectChange}
              onOpenCreateTask={() => setTaskModalOpen(true)}
            />

            {projects.length === 0 && !loadingProjects ? (
              <Empty description="Создайте проект, чтобы начать" />
            ) : loading ? (
              <div className="loader">
                <Spin tip="Загрузка задач..." size="large" />
              </div>
            ) : selectedTask ? (
              <TaskDetailCard
                task={selectedTask}
                user={user}
                updatingId={updatingId}
                deletingTaskId={deletingTaskId}
                deletingCommentId={deletingCommentId}
                editingId={editingId}
                descriptionDraft={descriptionDraft}
                commentDraft={commentDraft}
                addingComment={addingComment}
                onClose={closeTaskDetails}
                onChangeStatus={changeStatus}
                onDeleteTask={(taskId) => void deleteTask(taskId)}
                onStartEditDescription={startEditDescription}
                onCancelEditDescription={cancelEditDescription}
                onSaveDescription={saveDescription}
                onDescriptionDraftChange={setDescriptionDraft}
                onCommentDraftChange={setCommentDraft}
                onAddComment={() => void addComment()}
                onDeleteComment={(commentId) => void deleteComment(commentId)}
              />
            ) : (
              <Board groupedTasks={grouped} onSelectTask={openTaskDetails} />
            )}
          </>
        )}
      </Layout.Content>
      <ProfileModal
        open={profileModalOpen}
        user={user}
        saving={profileSaving}
        profileFirstName={profileFirstName}
        profileLastName={profileLastName}
        profileUsername={profileUsername}
        profileTelegram={profileTelegram}
        profilePassword={profilePassword}
        onFirstNameChange={setProfileFirstName}
        onLastNameChange={setProfileLastName}
        onUsernameChange={setProfileUsername}
        onTelegramChange={setProfileTelegram}
        onPasswordChange={setProfilePassword}
        onSave={() => void handleUpdateProfile()}
        onClose={() => setProfileModalOpen(false)}
      />
      <UserInfoModal
        user={editingUserInfo}
        firstName={editingUserFirstName}
        lastName={editingUserLastName}
        saving={savingUserInfo}
        onFirstNameChange={setEditingUserFirstName}
        onLastNameChange={setEditingUserLastName}
        onSave={() => void saveUserInfo()}
        onClose={() => setEditingUserInfo(null)}
      />
      <CreateTaskModal
        open={taskModalOpen}
        title={title}
        description={description}
        creating={creating}
        canCreate={!!selectedProject}
        onTitleChange={setTitle}
        onDescriptionChange={setDescription}
        onCreate={handleCreate}
        onClose={() => setTaskModalOpen(false)}
      />
      <PasswordModal
        user={passwordModalUser}
        newPassword={newPassword}
        updatingPassword={updatingPassword}
        onPasswordChange={setNewPassword}
        onSave={() => void handlePasswordChange()}
        onClose={() => setPasswordModalUser(null)}
      />
    </Layout>
  );
}

export default App;
