import { SettingOutlined } from "@ant-design/icons";
import { Button, Drawer, Space, Tag } from "antd";

import AutoRefreshControl from "../shared/AutoRefreshControl";
import type { AutoRefreshIntervalMs, User } from "../../types";

type MobileNavProps = {
  open: boolean;
  onClose: () => void;
  user: User;
  activePage: "board" | "settings" | "quick";
  autoRefreshIntervalMs: AutoRefreshIntervalMs | null;
  onAutoRefreshChange: (value: AutoRefreshIntervalMs | null) => void;
  onToggleSettings: () => void;
  onOpenProfile: () => void;
  onLogout: () => void;
};

function MobileNav({
  open,
  onClose,
  user,
  activePage,
  autoRefreshIntervalMs,
  onAutoRefreshChange,
  onToggleSettings,
  onOpenProfile,
  onLogout,
}: MobileNavProps) {
  return (
    <Drawer
      title="Меню"
      placement="right"
      width={320}
      open={open}
      onClose={onClose}
    >
      <Space direction="vertical" size="middle" className="mobile-nav">
        <div className="mobile-nav-user">
          <div className="mobile-nav-user__name">
            {user.firstName || user.lastName
              ? `${user.firstName ?? ""} ${user.lastName ?? ""}`.trim()
              : user.username || user.email}
          </div>
          <div className="mobile-nav-user__meta">
            <Tag color={user.role === "admin" ? "green" : "blue"}>
              {user.role === "admin" ? "Админ" : "Пользователь"}
            </Tag>
          </div>
        </div>
        {user.role === "admin" && (
          <Button block type="default" onClick={onToggleSettings}>
            {activePage === "settings" ? "К задачам" : "Настройки"}
          </Button>
        )}
        <Button
          block
          type="default"
          icon={<SettingOutlined />}
          onClick={onOpenProfile}
        >
          Профиль
        </Button>
        <AutoRefreshControl
          value={autoRefreshIntervalMs}
          onChange={onAutoRefreshChange}
        />
        <Button block type="primary" danger onClick={onLogout}>
          Выйти
        </Button>
      </Space>
    </Drawer>
  );
}

export default MobileNav;
