import {
  CheckCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { type ReactNode } from "react";

import type { StatusKey } from "./types";

export const statusOrder: StatusKey[] = ["new", "in_progress", "done"];

export const statusMeta: Record<
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

export const columnDescriptions: Record<StatusKey, string> = {
  new: "Все новые задачи появляются здесь",
  in_progress: "Задачи, над которыми ведется работа",
  done: "Завершенные задачи",
};

export const AUTO_REFRESH_INTERVAL_STORAGE_KEY =
  "litetask:autoRefreshIntervalMs";
