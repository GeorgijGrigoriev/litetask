import { Button, Card, Form, Input, Tabs } from "antd";

type AuthMode = "login" | "register";

type AuthFormValues = {
  email: string;
  password: string;
  username?: string;
  firstName?: string;
  lastName?: string;
};

type AuthCardProps = {
  authMode: AuthMode;
  authError: string;
  authLoading: boolean;
  onAuthModeChange: (mode: AuthMode) => void;
  onSubmit: (values: AuthFormValues, mode: AuthMode) => void;
};

function AuthCard({
  authMode,
  authError,
  authLoading,
  onAuthModeChange,
  onSubmit,
}: AuthCardProps) {
  return (
    <Card className="auth-card" title="Войдите или зарегистрируйтесь">
      <Tabs
        activeKey={authMode}
        onChange={(key) => onAuthModeChange(key as AuthMode)}
        items={[
          { key: "login", label: "Вход" },
          { key: "register", label: "Регистрация" },
        ]}
      />
      <Form
        layout="vertical"
        onFinish={(values) => onSubmit(values, authMode)}
      >
        {authMode === "register" && (
          <>
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
              <Input
                placeholder="юзернейм"
                autoCapitalize="none"
                autoCorrect="off"
              />
            </Form.Item>
            <Form.Item name="firstName" label="Имя">
              <Input placeholder="Имя (опционально)" />
            </Form.Item>
            <Form.Item name="lastName" label="Фамилия">
              <Input placeholder="Фамилия (опционально)" />
            </Form.Item>
          </>
        )}
        <Form.Item
          name="email"
          label={authMode === "login" ? "Email или юзернейм" : "Email"}
          rules={[
            {
              required: true,
              message:
                authMode === "login"
                  ? "Введите email или юзернейм"
                  : "Введите email",
            },
            ...(authMode === "register"
              ? [{ type: "email" as const, message: "Введите email" }]
              : []),
          ]}
        >
          <Input
            placeholder={
              authMode === "login"
                ? "you@example.com или юзернейм"
                : "you@example.com"
            }
            autoCapitalize="none"
            autoCorrect="off"
          />
        </Form.Item>
        <Form.Item
          name="password"
          label="Пароль"
          rules={[
            { required: true },
            ...(authMode === "register"
              ? [{ min: 6, message: "Минимум 6 символов" }]
              : []),
          ]}
        >
          <Input.Password placeholder="••••••" />
        </Form.Item>
        {authError && <div className="auth-error">{authError}</div>}
        <Button type="primary" htmlType="submit" block loading={authLoading}>
          {authMode === "login" ? "Войти" : "Зарегистрироваться"}
        </Button>
      </Form>
    </Card>
  );
}

export default AuthCard;
