import { Select } from "antd";

import type { AutoRefreshIntervalMs } from "../../types";

type AutoRefreshControlProps = {
  value: AutoRefreshIntervalMs | null;
  onChange: (value: AutoRefreshIntervalMs | null) => void;
};

function AutoRefreshControl({ value, onChange }: AutoRefreshControlProps) {
  return (
    <div className="auto-refresh-row">
      <div>
        <div>Автообновление</div>
        <div className="muted-text">Обновление задач</div>
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
          { label: "Выкл", value: "off" },
          { label: "5 секунд", value: 5_000 },
          { label: "30 секунд", value: 30_000 },
          { label: "1 минута", value: 60_000 },
          { label: "5 минут", value: 300_000 },
        ]}
        style={{ width: 140 }}
      />
    </div>
  );
}

export default AutoRefreshControl;
