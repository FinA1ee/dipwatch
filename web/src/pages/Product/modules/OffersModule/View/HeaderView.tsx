import { Link } from "react-router";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { useProductPage } from "../Model/useProductPage";

/** Where the page sits, the product name, and when its stores were checked. */
export function HeaderView() {
  const { page, loading, error } = useProductPage();
  if (loading) return <p className="text-muted">Loading…</p>;
  if (error || !page)
    return <p className="rounded-lg bg-rose-50 p-3 text-rose-800">{error?.message ?? "Not found"}</p>;
  const search = `/?q=${encodeURIComponent(page.product.name)}`;
  return (
    <div className="space-y-2">
      <Breadcrumbs
        items={[
          { label: "Search", to: "/" },
          { label: `Results for ${page.product.name}`, to: search },
          { label: page.product.name },
        ]}
      />
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-[34px] font-semibold leading-tight">{page.product.name}</h1>
        <p className="text-sm text-muted">
          {page.refreshing
            ? "Stores are being searched now."
            : page.last_refreshed
              ? `Checked ${new Date(page.last_refreshed).toLocaleString()}`
              : "Not checked yet."}{" "}
          <Link to={search} className="hover:underline">
            Search again
          </Link>
        </p>
      </div>
    </div>
  );
}
