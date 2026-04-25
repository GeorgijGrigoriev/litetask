import {
  CheckCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { type ReactNode } from "react";

import type { I18nKey } from "./i18n/ru";
import type { PriorityKey, StatusKey } from "./types";

export const statusOrder: StatusKey[] = ["new", "in_progress", "done"];

export const statusMeta: Record<
  StatusKey,
  { label: I18nKey; color: string; icon: ReactNode }
> = {
  new: { label: "status.new", color: "blue", icon: <PlusOutlined /> },
  in_progress: {
    label: "status.in_progress",
    color: "gold",
    icon: <PlayCircleOutlined />,
  },
  done: { label: "status.done", color: "green", icon: <CheckCircleOutlined /> },
};

export const columnDescriptions: Record<StatusKey, I18nKey> = {
  new: "column.new",
  in_progress: "column.in_progress",
  done: "column.done",
};

export const priorityOrder: PriorityKey[] = ["high", "medium", "low"];

export const priorityMeta: Record<PriorityKey, { label: I18nKey; color: string }> = {
  high:   { label: "priority.high",   color: "red" },
  medium: { label: "priority.medium", color: "orange" },
  low:    { label: "priority.low",    color: "default" },
};

export const AUTO_REFRESH_INTERVAL_STORAGE_KEY =
  "litetask:autoRefreshIntervalMs";
