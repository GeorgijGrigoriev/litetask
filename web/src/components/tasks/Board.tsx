import { Col, Row } from "antd";

import { statusOrder } from "../../constants";
import type { StatusKey, Task } from "../../types";
import StatusColumn from "./StatusColumn";

type BoardProps = {
  groupedTasks: Record<StatusKey, Task[]>;
  onSelectTask: (taskId: number) => void;
};

function Board({ groupedTasks, onSelectTask }: BoardProps) {
  return (
    <Row gutter={[16, 16]} className="board">
      {statusOrder.map((status) => (
        <Col key={status} xs={24} md={8}>
          <StatusColumn
            status={status}
            tasks={groupedTasks[status]}
            onSelectTask={onSelectTask}
          />
        </Col>
      ))}
    </Row>
  );
}

export default Board;
