/** 统一页面标题区 */
export function PageHeader({
  title,
  subtitle,
}: {
  title: string;
  subtitle?: string;
}) {
  return (
    <header className="page-header">
      <h1 className="page-title">{title}</h1>
      {subtitle && <p className="page-subtitle">{subtitle}</p>}
    </header>
  );
}
