import { Button, Col, Form, Input, Row, Select, Space } from "antd";

import { useTranslation } from "../../i18n";
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
  const { t } = useTranslation();
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
          <Form.Item name="firstName" label={t("auth.firstName")}>
            <Input placeholder={t("auth.firstName")} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="lastName" label={t("auth.lastName")}>
            <Input placeholder={t("auth.lastName")} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item
            name="username"
            label={t("auth.username")}
            rules={[
              { required: true, message: t("auth.usernameRequired") },
              { min: 3, max: 32, message: t("auth.usernameLength") },
              {
                pattern: /^[A-Za-z0-9_.-]+$/,
                message: t("auth.usernamePattern"),
              },
            ]}
          >
            <Input placeholder={t("auth.usernamePlaceholder")} autoCapitalize="none" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item
            name="email"
            label="Email"
            rules={[
              { required: true, type: "email", message: t("auth.emailRequired") },
            ]}
          >
            <Input placeholder="email@example.com" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item
            name="password"
            label={t("auth.password")}
            rules={[{ required: true, min: 6, message: t("auth.passwordMinLength") }]}
          >
            <Input.Password placeholder={t("auth.password")} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="role" label={t("admin.role")} rules={[{ required: true }]}>
            <Select
              options={[
                { label: t("admin.roleUser"), value: "user" },
                { label: t("admin.roleAdmin"), value: "admin" },
                { label: t("admin.roleBlocked"), value: "blocked" },
              ]}
            />
          </Form.Item>
        </Col>
      </Row>
      <Space className="user-form-actions" size="middle">
        <Button type="primary" htmlType="submit" loading={creatingUser}>
          {t("admin.addUser")}
        </Button>
      </Space>
    </Form>
  );
}

export default UserForm;
