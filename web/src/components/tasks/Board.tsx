import { Col, Row } from "antd";

import { statusOrder } from "../../constants";
import type { PriorityKey, StatusKey, Task } from "../../types";
import StatusColumn from "./StatusColumn";

const priorityWeight: Record<PriorityKey, number> = { high: 0, medium: 1, low: 2 };

type BoardProps = {
  groupedTasks: Record<StatusKey, Task[]>;
  onSelectTask: (taskId: number) => void;
  prioritySort: "asc" | "desc" | null;
};

function Board({ groupedTasks, onSelectTask, prioritySort }: BoardProps) {
  return (
    <Row gutter={[16, 16]} className="board">
      {statusOrder.map((status) => {
        const tasks = groupedTasks[status];
        const sorted = prioritySort
          ? [...tasks].sort((a, b) => {
              const wa = priorityWeight[a.priority ?? "medium"];
              const wb = priorityWeight[b.priority ?? "medium"];
              return prioritySort === "asc" ? wa - wb : wb - wa;
            })
          : tasks;
        return (
          <Col key={status} xs={24} md={8}>
            <StatusColumn
              status={status}
              tasks={sorted}
              onSelectTask={onSelectTask}
            />
          </Col>
        );
      })}
    </Row>
  );
}

export default Board;
