import { Link } from "react-router";
import { Breadcrumbs } from "@/components/Breadcrumbs";
import { useMember } from "@/hooks/useMember";
import { SettingsView } from "./modules/SettingsModule/View/SettingsView";
import { TrackedView } from "./modules/TrackedModule/View/TrackedView";

export function MyProductsPage() {
  const { member, loading } = useMember();
  if (loading) return null;
  if (!member) {
    return (
      <p className="text-muted">
        <Link to="/signin?next=%2Fme" className="font-medium hover:underline">
          Sign in
        </Link>{" "}
        to see the products you track.
      </p>
    );
  }
  return (
    <div className="space-y-8">
      <section className="space-y-3">
        <Breadcrumbs items={[{ label: "Search", to: "/" }, { label: "My products" }]} />
        <h1 className="text-2xl font-semibold">My products</h1>
        <TrackedView />
      </section>
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Where alerts go</h2>
        <SettingsView />
      </section>
    </div>
  );
}
