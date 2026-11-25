import { Navigate, useSearchParams } from "react-router";
import { useMember } from "@/hooks/useMember";
import { EmailView } from "./modules/EmailModule/View/EmailView";
import { GitHubView } from "./modules/GitHubModule/View/GitHubView";
import { safeReturnTo } from "./returnTo";

export function SignInPage() {
  const next = useSearchParams()[0].get("next");
  const { member } = useMember();
  if (member) return <Navigate to={safeReturnTo(next)} replace />;
  return (
    <div className="mx-auto max-w-sm space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Sign in</h1>
        <p className="mt-1 text-slate-600">Members get 30 new searches a day instead of 10.</p>
      </div>
      <EmailView next={next} />
      <div className="text-center text-sm text-slate-500">or</div>
      <GitHubView next={next} />
    </div>
  );
}
