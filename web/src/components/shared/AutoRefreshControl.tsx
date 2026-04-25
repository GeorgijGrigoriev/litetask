import { Select } from "antd";

import { useTranslation } from "../../i18n";
import type { AutoRefreshIntervalMs } from "../../types";

type AutoRefreshControlProps = {
  value: AutoRefreshIntervalMs | null;
  onChange: (value: AutoRefreshIntervalMs | null) => void;
};

function AutoRefreshControl({ value, onChange }: AutoRefreshControlProps) {
  const { t } = useTranslation();
  return (
    <div className="auto-refresh-row">
      <div>
        <div>{t("autoRefresh.title")}</div>
        <div className="muted-text">{t("autoRefresh.subtitle")}</div>
      </div>
      <Select
        value={value === null ? "off" : value}
        onChange={(next) => {
          if (next === "off") {
            onChange(null);
          } else {
            onChange(next as AutoRefreshIntervalMs);
          }
        }}
        options={[
          { label: t("autoRefresh.off"), value: "off" },
          { label: t("autoRefresh.5s"), value: 5_000 },
          { label: t("autoRefresh.30s"), value: 30_000 },
          { label: t("autoRefresh.1m"), value: 60_000 },
          { label: t("autoRefresh.5m"), value: 300_000 },
        ]}
        style={{ width: 140 }}
      />
    </div>
  );
}

export default AutoRefreshControl;
