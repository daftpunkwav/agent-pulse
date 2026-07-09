import Link from "next/link";

export default function NotFound() {
  return (
    <div className="card" style={{ textAlign: "center", padding: "2rem" }}>
      <h2 className="text-2xl mb-4">404</h2>
      <p className="text-sm text-gray mb-4">页面不存在</p>
      <Link href="/" className="btn btn-primary">
        返回首页
      </Link>
    </div>
  );
}
