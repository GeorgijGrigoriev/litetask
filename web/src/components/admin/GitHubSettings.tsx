import { DeleteOutlined, GithubOutlined, SyncOutlined } from "@ant-design/icons";
import { Button, Card, Select, Space, Table, Tag, message } from "antd";
import { useCallback, useEffect, useState } from "react";
import api from "../../api";
import type { GitHubIntegration, Project } from "../../types";

interface Props {
  projects: Project[];
}

interface SyncResult {
  created: number;
  updated: number;
  skipped: number;
  errors: string[];
}

export default function GitHubSettings({ projects }: Props) {
  const [integrations, setIntegrations] = useState<GitHubIntegration[]>([]);
  const [repos, setRepos] = useState<string[]>([]);
  const [loadingIntegrations, setLoadingIntegrations] = useState(false);
  const [hasToken, setHasToken] = useState(false);
  const [syncing, setSyncing] = useState<number | null>(null);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [connectRepo, setConnectRepo] = useState<string | undefined>(undefined);
  const [connectProject, setConnectProject] = useState<number | undefined>(undefined);
  const [connecting, setConnecting] = useState(false);

  const loadIntegrations = useCallback(async () => {
    setLoadingIntegrations(true);
    try {
      const resp = await api.get<GitHubIntegration[]>("/github/integrations");
      setIntegrations(resp.data ?? []);
    } catch {
      // ignore
    } finally {
      setLoadingIntegrations(false);
    }
  }, []);

  const loadRepos = useCallback(async () => {
    try {
      const resp = await api.get<string[]>("/github/repos");
      setRepos(resp.data ?? []);
      setHasToken(true);
    } catch {
      setHasToken(false);
      setRepos([]);
    }
  }, []);

  useEffect(() => {
    void loadIntegrations();
    void loadRepos();
    const params = new URLSearchParams(window.location.search);
    if (params.get("github") === "connected") {
      message.success("GitHub подключен");
      window.history.replaceState({}, "", window.location.pathname);
    }
  }, [loadIntegrations, loadRepos]);

  const handleConnect = async () => {
    if (!connectRepo || !connectProject) {
      message.warning("Выберите репозиторий и проект");
      return;
    }
    setConnecting(true);
    try {
      const resp = await api.post<GitHubIntegration>("/github/connect", {
        repoFullName: connectRepo,
        projectId: connectProject,
      });
      setIntegrations((prev) => [...prev, resp.data]);
      setConnectRepo(undefined);
      setConnectProject(undefined);
      message.success("Интеграция создана");
    } catch {
      message.error("Не удалось создать интеграцию");
    } finally {
      setConnecting(false);
    }
  };

  const handleSync = async (id: number) => {
    setSyncing(id);
    try {
      const resp = await api.post<SyncResult>(`/github/sync/${id}`);
      const { created, updated, skipped, errors } = resp.data;
      message.success(`Синхронизировано: создано ${created}, обновлено ${updated}, пропущено ${skipped}`);
      if (errors && errors.length > 0) {
        message.warning(`Ошибки (${errors.length}): ${errors.slice(0, 2).join("; ")}`);
      }
      void loadIntegrations();
    } catch {
      message.error("Ошибка синхронизации");
    } finally {
      setSyncing(null);
    }
  };

  const handleDelete = async (id: number) => {
    setDeleting(id);
    try {
      await api.delete(`/github/integrations/${id}`);
      setIntegrations((prev) => prev.filter((ig) => ig.id !== id));
      message.success("Интеграция удалена");
    } catch {
      message.error("Не удалось удалить интеграцию");
    } finally {
      setDeleting(null);
    }
  };

  const nonInboxProjects = projects.filter((p) => !p.isInbox);

  const columns = [
    {
      title: "Репозиторий",
      dataIndex: "repoFullName",
      key: "repo",
      render: (name: string) => (
        <Space>
          <GithubOutlined />
          {name}
        </Space>
      ),
    },
    {
      title: "Проект",
      dataIndex: "projectId",
      key: "project",
      render: (id: number) => projects.find((p) => p.id === id)?.name ?? String(id),
    },
    {
      title: "Последняя синхронизация",
      dataIndex: "lastSyncedAt",
      key: "lastSyncedAt",
      render: (dt: string | null) =>
        dt ? new Date(dt).toLocaleString() : <Tag>Никогда</Tag>,
    },
    {
      title: "",
      key: "actions",
      render: (_: unknown, record: GitHubIntegration) => (
        <Space>
          <Button
            size="small"
            icon={<SyncOutlined />}
            loading={syncing === record.id}
            onClick={() => void handleSync(record.id)}
          >
            Синхронизировать
          </Button>
          <Button
            size="small"
            danger
            icon={<DeleteOutlined />}
            loading={deleting === record.id}
            onClick={() => void handleDelete(record.id)}
          >
            Удалить
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <Card title="Интеграция с GitHub" className="create-card">
      {!hasToken ? (
        <Space direction="vertical">
          <span>Подключите GitHub-аккаунт для импорта задач из Issues.</span>
          <Button type="primary" icon={<GithubOutlined />} href="/api/github/auth">
            Подключить GitHub
          </Button>
        </Space>
      ) : (
        <>
          <Space style={{ marginBottom: 16 }} wrap>
            <Select
              placeholder="Репозиторий"
              value={connectRepo}
              onChange={setConnectRepo}
              style={{ minWidth: 240 }}
              showSearch
              options={repos.map((r) => ({ label: r, value: r }))}
            />
            <Select
              placeholder="Проект"
              value={connectProject}
              onChange={setConnectProject}
              style={{ minWidth: 160 }}
              options={nonInboxProjects.map((p) => ({ label: p.name, value: p.id }))}
            />
            <Button type="primary" loading={connecting} onClick={() => void handleConnect()}>
              Добавить интеграцию
            </Button>
            <Button href="/api/github/auth">Переподключить</Button>
          </Space>
          <Table
            dataSource={integrations}
            columns={columns}
            rowKey="id"
            loading={loadingIntegrations}
            size="small"
            pagination={false}
            locale={{ emptyText: "Нет интеграций" }}
          />
        </>
      )}
    </Card>
  );
}
