import { Button, Card, Form, Input, Tabs } from "antd";

import { useTranslation } from "../../i18n";

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
  const { t } = useTranslation();
  return (
    <Card className="auth-card" title={t("auth.cardTitle")}>
      <Tabs
        activeKey={authMode}
        onChange={(key) => onAuthModeChange(key as AuthMode)}
        items={[
          { key: "login", label: t("auth.loginTab") },
          { key: "register", label: t("auth.registerTab") },
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
              <Input
                placeholder={t("auth.usernamePlaceholder")}
                autoCapitalize="none"
                autoCorrect="off"
              />
            </Form.Item>
            <Form.Item name="firstName" label={t("auth.firstName")}>
              <Input placeholder={t("auth.firstNameOptional")} />
            </Form.Item>
            <Form.Item name="lastName" label={t("auth.lastName")}>
              <Input placeholder={t("auth.lastNameOptional")} />
            </Form.Item>
          </>
        )}
        <Form.Item
          name="email"
          label={authMode === "login" ? t("auth.emailOrUsername") : t("auth.email")}
          rules={[
            {
              required: true,
              message:
                authMode === "login"
                  ? t("auth.emailOrUsernameRequired")
                  : t("auth.emailRequired"),
            },
            ...(authMode === "register"
              ? [{ type: "email" as const, message: t("auth.emailRequired") }]
              : []),
          ]}
        >
          <Input
            placeholder={
              authMode === "login"
                ? t("auth.emailOrUsernamePlaceholder")
                : "you@example.com"
            }
            autoCapitalize="none"
            autoCorrect="off"
          />
        </Form.Item>
        <Form.Item
          name="password"
          label={t("auth.password")}
          rules={[
            { required: true },
            ...(authMode === "register"
              ? [{ min: 6, message: t("auth.passwordMinLength") }]
              : []),
          ]}
        >
          <Input.Password placeholder="••••••" />
        </Form.Item>
        {authError && <div className="auth-error">{authError}</div>}
        <Button type="primary" htmlType="submit" block loading={authLoading}>
          {authMode === "login" ? t("auth.loginButton") : t("auth.registerButton")}
        </Button>
      </Form>
    </Card>
  );
}

export default AuthCard;
