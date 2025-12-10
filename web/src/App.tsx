import {
  CheckCircleOutlined,
  DeleteOutlined,
  FolderAddOutlined,
  HighlightOutlined,
  LoadingOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import {
  Button,
  Card,
  Col,
  Empty,
  Input,
  Layout,
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Tag,
  message,
} from 'antd'
import axios from 'axios'
import { type ReactNode, useEffect, useMemo, useState } from 'react'
import './App.css'

type StatusKey = 'new' | 'in_progress' | 'done'

type Task = {
  id: number
  title: string
  status: StatusKey
  comment: string
  projectId: number
  createdAt: string
}

type Project = {
  id: number
  name: string
  createdAt: string
}

const statusOrder: StatusKey[] = ['new', 'in_progress', 'done']

const statusMeta: Record<StatusKey, { label: string; color: string; icon: ReactNode }> = {
  new: { label: 'Новая', color: 'blue', icon: <PlusOutlined /> },
  in_progress: { label: 'В работе', color: 'gold', icon: <PlayCircleOutlined /> },
  done: { label: 'Готова', color: 'green', icon: <CheckCircleOutlined /> },
}

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api',
})

const formatDate = (value: string) =>
  new Date(value).toLocaleString('ru-RU', {
    timeZone: 'Europe/Moscow',
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })

const columnDescriptions: Record<StatusKey, string> = {
  new: 'Все новые задачи появляются здесь',
  in_progress: 'Задачи, над которыми ведется работа',
  done: 'Завершенные задачи',
}

function StatusColumn({
  status,
  tasks,
  onChangeStatus,
  updatingId,
  editingId,
  commentDraft,
  onEditStart,
  onEditChange,
  onEditSave,
  onEditCancel,
  onDeleteTask,
  deletingId,
}: {
  status: StatusKey
  tasks: Task[]
  updatingId: number | null
  onChangeStatus: (taskId: number, status: StatusKey) => Promise<void>
  editingId: number | null
  commentDraft: string
  onEditStart: (task: Task) => void
  onEditChange: (value: string) => void
  onEditSave: (taskId: number) => Promise<void>
  onEditCancel: () => void
  onDeleteTask: (taskId: number) => Promise<void>
  deletingId: number | null
}) {
  return (
    <Card
      title={
        <Space size="small">
          {statusMeta[status].icon}
          <span>{statusMeta[status].label}</span>
          <Tag color={statusMeta[status].color}>{tasks.length}</Tag>
        </Space>
      }
      extra={<span className="column-subtitle">{columnDescriptions[status]}</span>}
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
            extra={<Tag color={statusMeta[task.status].color}>{statusMeta[task.status].label}</Tag>}
            actions={[
              <Select
                key="status"
                size="small"
                value={task.status}
                className="status-select"
                onChange={(value) => onChangeStatus(task.id, value)}
                options={statusOrder.map((s) => ({
                  label: statusMeta[s].label,
                  value: s,
                }))}
                suffixIcon={
                  updatingId === task.id ? <LoadingOutlined spin /> : <HighlightOutlined />
                }
              />,
              <Popconfirm
                key="delete"
                title="Удалить задачу?"
                onConfirm={() => onDeleteTask(task.id)}
              >
                <Button
                  type="text"
                  icon={<DeleteOutlined />}
                  danger
                  loading={deletingId === task.id}
                />
              </Popconfirm>,
            ]}
          >
            <Space direction="vertical" size={4}>
              <div className="meta-row">
                <span className="meta-label">Создана:</span>
                <span className="meta-value">{formatDate(task.createdAt)}</span>
              </div>
              <div className="comment-block">
                <div className="meta-label">Комментарий:</div>
                {editingId === task.id ? (
                  <Space direction="vertical" size="small" className="comment-edit">
                    <Input.TextArea
                      value={commentDraft}
                      onChange={(e) => onEditChange(e.target.value)}
                      autoSize={{ minRows: 2, maxRows: 4 }}
                      placeholder="Добавьте детали по задаче"
                    />
                    <Space size="small">
                      <Button
                        type="primary"
                        size="small"
                        onClick={() => onEditSave(task.id)}
                        loading={updatingId === task.id}
                      >
                        Сохранить
                      </Button>
                      <Button size="small" onClick={onEditCancel}>
                        Отмена
                      </Button>
                    </Space>
                  </Space>
                ) : (
                  <div className="comment-display">
                    <span className="meta-value">
                      {task.comment ? task.comment : 'Комментария нет'}
                    </span>
                    <Button type="link" size="small" onClick={() => onEditStart(task)}>
                      Изменить
                    </Button>
                  </div>
                )}
              </div>
            </Space>
          </Card>
        ))}
      </Space>
    </Card>
  )
}

function App() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProject, setSelectedProject] = useState<number | null>(null)
  const [newProjectName, setNewProjectName] = useState('')
  const [title, setTitle] = useState('')
  const [comment, setComment] = useState('')
  const [loading, setLoading] = useState(true)
  const [loadingProjects, setLoadingProjects] = useState(true)
  const [creating, setCreating] = useState(false)
  const [creatingProject, setCreatingProject] = useState(false)
  const [updatingId, setUpdatingId] = useState<number | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [commentDraft, setCommentDraft] = useState('')
  const [deletingTaskId, setDeletingTaskId] = useState<number | null>(null)
  const [deletingProject, setDeletingProject] = useState(false)

  const grouped = useMemo(() => {
    const bucket: Record<StatusKey, Task[]> = {
      new: [],
      in_progress: [],
      done: [],
    }
    tasks.forEach((task) => bucket[task.status].push(task))
    return bucket
  }, [tasks])

  const loadTasks = async (projectId = selectedProject) => {
    if (!projectId) {
      setLoading(false)
      setTasks([])
      return
    }
    setLoading(true)
    try {
      const response = await api.get<Task[]>('/tasks', { params: { projectId } })
      setTasks(response.data)
    } catch (error) {
      console.error(error)
      message.error('Не удалось загрузить задачи')
    } finally {
      setLoading(false)
    }
  }

  const loadProjects = async () => {
    setLoadingProjects(true)
    try {
      const response = await api.get<Project[]>('/projects')
      setProjects(response.data)
      if (selectedProject && response.data.some((p) => p.id === selectedProject)) {
        // keep current
      } else if (response.data.length > 0) {
        setSelectedProject(response.data[0].id)
      } else {
        setSelectedProject(null)
      }
    } catch (error) {
      console.error(error)
      message.error('Не удалось загрузить проекты')
    } finally {
      setLoadingProjects(false)
    }
  }

  useEffect(() => {
    loadProjects()
  }, [])

  useEffect(() => {
    loadTasks()
  }, [selectedProject])

  const handleCreate = async () => {
    if (!selectedProject) {
      message.warning('Сначала выберите проект')
      return
    }
    if (!title.trim()) {
      message.warning('Введите название задачи')
      return
    }
    setCreating(true)
    try {
      const response = await api.post<Task>('/tasks', { title, comment, projectId: selectedProject })
      setTasks((prev) => [response.data, ...prev])
      setTitle('')
      setComment('')
      message.success('Задача создана')
    } catch (error) {
      console.error(error)
      message.error('Не удалось создать задачу')
    } finally {
      setCreating(false)
    }
  }

  const changeStatus = async (taskId: number, status: StatusKey) => {
    setUpdatingId(taskId)
    try {
      const response = await api.patch<Task>(`/tasks/${taskId}/status`, { status })
      setTasks((prev) => prev.map((task) => (task.id === taskId ? response.data : task)))
      message.success('Статус обновлен')
    } catch (error) {
      console.error(error)
      message.error('Не удалось обновить статус')
    } finally {
      setUpdatingId(null)
    }
  }

  const startEditComment = (task: Task) => {
    setEditingId(task.id)
    setCommentDraft(task.comment || '')
  }

  const cancelEditComment = () => {
    setEditingId(null)
    setCommentDraft('')
  }

  const saveComment = async (taskId: number) => {
    setUpdatingId(taskId)
    try {
      const response = await api.patch<Task>(`/tasks/${taskId}`, { comment: commentDraft })
      setTasks((prev) => prev.map((task) => (task.id === taskId ? response.data : task)))
      setEditingId(null)
      setCommentDraft('')
      message.success('Комментарий обновлен')
    } catch (error) {
      console.error(error)
      message.error('Не удалось обновить комментарий')
    } finally {
      setUpdatingId(null)
    }
  }

  const deleteTask = async (taskId: number) => {
    setDeletingTaskId(taskId)
    try {
      await api.delete(`/tasks/${taskId}`)
      setTasks((prev) => prev.filter((t) => t.id !== taskId))
      message.success('Задача удалена')
    } catch (error) {
      console.error(error)
      message.error('Не удалось удалить задачу')
    } finally {
      setDeletingTaskId(null)
    }
  }

  const handleProjectChange = (projectId: number) => {
    setSelectedProject(projectId)
  }

  const handleCreateProject = async () => {
    const name = newProjectName.trim()
    if (!name) {
      message.warning('Введите название проекта')
      return
    }
    setCreatingProject(true)
    try {
      const response = await api.post<Project>('/projects', { name })
      setProjects((prev) => [response.data, ...prev])
      setSelectedProject(response.data.id)
      setNewProjectName('')
      message.success('Проект создан')
    } catch (error) {
      console.error(error)
      if (axios.isAxiosError(error) && error.response?.status === 400) {
        message.error('Проект с таким названием уже существует')
      } else {
        message.error('Не удалось создать проект')
      }
    } finally {
      setCreatingProject(false)
    }
  }

  const handleDeleteProject = async () => {
    if (!selectedProject) {
      return
    }
    setDeletingProject(true)
    try {
      await api.delete(`/projects/${selectedProject}`)
      const next = projects.filter((p) => p.id !== selectedProject)
      setProjects(next)
      if (next.length > 0) {
        setSelectedProject(next[0].id)
      } else {
        setSelectedProject(null)
        setTasks([])
      }
      message.success('Проект удален')
    } catch (error) {
      console.error(error)
      if (axios.isAxiosError(error) && error.response?.status === 400) {
        message.error('Нельзя удалить проект по умолчанию')
      } else {
        message.error('Не удалось удалить проект')
      }
    } finally {
      setDeletingProject(false)
    }
  }

  return (
    <Layout className="layout">
      <Layout.Header className="header">
        <div className="brand">LiteTask</div>
        <Button type="default" icon={<ReloadOutlined />} onClick={() => loadTasks()}>
          Обновить
        </Button>
      </Layout.Header>
      <Layout.Content className="content">
        <Card className="project-card" title="Проекты">
          <Space className="project-row" wrap>
            <Select
              placeholder="Выберите проект"
              loading={loadingProjects}
              value={selectedProject ?? undefined}
              onChange={handleProjectChange}
              options={projects.map((p) => ({ value: p.id, label: p.name }))}
              style={{ minWidth: 220 }}
            />
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
            <Popconfirm
              title="Удалить проект и все его задачи?"
              onConfirm={handleDeleteProject}
              disabled={!selectedProject}
            >
              <Button danger loading={deletingProject} disabled={!selectedProject}>
                Удалить проект
              </Button>
            </Popconfirm>
          </Space>
        </Card>

        <Card className="create-card" title="Добавить задачу">
          <Space direction="vertical" size="small" className="create-column">
            <Space.Compact className="create-row">
              <Input
                placeholder="Новая задача..."
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                onPressEnter={handleCreate}
                maxLength={140}
              />
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={handleCreate}
                loading={creating}
              >
                Добавить
              </Button>
            </Space.Compact>
            <Input.TextArea
              placeholder="Комментарий (необязательно)"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              autoSize={{ minRows: 2, maxRows: 4 }}
            />
          </Space>
        </Card>

        {projects.length === 0 && !loadingProjects ? (
          <Empty description="Создайте проект, чтобы начать" />
        ) : loading ? (
          <div className="loader">
            <Spin tip="Загрузка задач..." size="large" />
          </div>
        ) : (
          <Row gutter={[16, 16]} className="board">
            {statusOrder.map((status) => (
              <Col key={status} xs={24} md={8}>
                <StatusColumn
                  status={status}
                  tasks={grouped[status]}
                  onChangeStatus={changeStatus}
                  updatingId={updatingId}
                  editingId={editingId}
                  commentDraft={commentDraft}
                  onEditStart={startEditComment}
                  onEditChange={setCommentDraft}
                  onEditSave={saveComment}
                  onEditCancel={cancelEditComment}
                  onDeleteTask={deleteTask}
                  deletingId={deletingTaskId}
                />
              </Col>
            ))}
          </Row>
        )}
      </Layout.Content>
    </Layout>
  )
}

export default App
