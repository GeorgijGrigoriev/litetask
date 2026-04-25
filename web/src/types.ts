export type StatusKey = "new" | "in_progress" | "done";

export type PriorityKey = "high" | "medium" | "low";

export type TaskComment = {
  id: number;
  taskId: number;
  body: string;
  authorId?: number | null;
  authorEmail?: string | null;
  createdAt: string;
};

export type Task = {
  id: number;
  title: string;
  status: StatusKey;
  priority?: PriorityKey;
  description?: string | null;
  projectId: number;
  createdAt: string;
  createdBy?: number;
  authorEmail?: string;
  authorFirstName?: string;
  authorLastName?: string;
  comments?: TaskComment[];
};

export type Project = {
  id: number;
  name: string;
  isInbox: boolean;
  createdAt: string;
};

export type User = {
  id: number;
  email: string;
  username?: string | null;
  role: "admin" | "user" | "blocked";
  firstName?: string | null;
  lastName?: string | null;
  projectIds?: number[] | null;
  telegram?: string | null;
};

export type AutoRefreshIntervalMs = 5000 | 30000 | 60000 | 300000;
