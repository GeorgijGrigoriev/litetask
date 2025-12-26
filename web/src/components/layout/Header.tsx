import { MenuOutlined } from "@ant-design/icons";
import { Button, Layout } from "antd";

type HeaderProps = {
  showMenuButton?: boolean;
  onMenuClick?: () => void;
  brandHref?: string;
  onBrandClick?: () => void;
};

function Header({
  showMenuButton,
  onMenuClick,
  brandHref,
  onBrandClick,
}: HeaderProps) {
  const brand = <div className="brand">LiteTask</div>;
  return (
    <Layout.Header className="header">
      {brandHref ? (
        <a href={brandHref}>{brand}</a>
      ) : onBrandClick ? (
        <button type="button" className="brand-btn" onClick={onBrandClick}>
          {brand}
        </button>
      ) : (
        brand
      )}
      {showMenuButton && (
        <Button
          type="text"
          icon={<MenuOutlined />}
          className="header-menu-btn"
          onClick={onMenuClick}
          aria-label="Открыть меню"
        />
      )}
    </Layout.Header>
  );
}

export default Header;
