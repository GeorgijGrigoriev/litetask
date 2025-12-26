import { Button, Col, Form, Input, Row, Select, Space } from "antd";

import type { User } from "../../types";

type UserFormValues = {
  email: string;
  username?: string;
  password: string;
  role: User["role"];
  firstName?: string;
  lastName?: string;
};

type UserFormProps = {
  creatingUser: boolean;
  onCreateUser: (values: UserFormValues) => void;
};

function UserForm({ creatingUser, onCreateUser }: UserFormProps) {
  return (
    <Form
      layout="vertical"
      onFinish={onCreateUser}
      className="user-form"
      style={{ marginBottom: 12 }}
      initialValues={{ role: "user" }}
    >
      <Row gutter={[16, 8]}>
        <Col xs={24} md={8}>
          <Form.Item name="firstName" label="Имя">
            <Input placeholder="Имя" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="lastName" label="Фамилия">
            <Input placeholder="Фамилия" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item
            name="username"
            label="Юзернейм"
            rules={[
              { required: true, message: "Введите юзернейм" },
              { min: 3, max: 32, message: "От 3 до 32 символов" },
              {
                pattern: /^[A-Za-z0-9_.-]+$/,
                message: "Допустимы a-z, 0-9, _, ., -",
              },
            ]}
          >
            <Input placeholder="юзернейм" autoCapitalize="none" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item
            name="email"
            label="Email"
            rules={[
              { required: true, type: "email", message: "Введите email" },
            ]}
          >
            <Input placeholder="email@example.com" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item
            name="password"
            label="Пароль"
            rules={[{ required: true, min: 6, message: "Минимум 6 символов" }]}
          >
            <Input.Password placeholder="Пароль" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="role" label="Роль" rules={[{ required: true }]}>
            <Select
              options={[
                { label: "Пользователь", value: "user" },
                { label: "Админ", value: "admin" },
                { label: "Заблокирован", value: "blocked" },
              ]}
            />
          </Form.Item>
        </Col>
      </Row>
      <Space className="user-form-actions" size="middle">
        <Button type="primary" htmlType="submit" loading={creatingUser}>
          Добавить пользователя
        </Button>
      </Space>
    </Form>
  );
}

export default UserForm;
