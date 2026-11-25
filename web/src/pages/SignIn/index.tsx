import { Navigate, useSearchParams } from "react-router";
import { useMember } from "@/hooks/useMember";
import { EmailView } from "./modules/EmailModule/View/EmailView";
import { GitHubView } from "./modules/GitHubModule/View/GitHubView";
import { useSignInMethods } from "./modules/MethodsModule/Model/useSignInMethods";
import { safeReturnTo } from "./returnTo";

export function SignInPage() {
  const next = useSearchParams()[0].get("next");
  const { member } = useMember();
  const { methods, loading } = useSignInMethods();
  if (member) return <Navigate to={safeReturnTo(next)} replace />;
  const none = !loading && !methods?.email && !methods?.github;
  return (
    <div className="mx-auto max-w-sm space-y-6">
      <div>
        <h1 className="text-2xl font-semibold">Sign in</h1>
        <p className="mt-1 text-slate-600">Members get 30 new searches a day instead of 10.</p>
      </div>
      {none && <p className="text-slate-600">Sign-in is not available yet.</p>}
      {methods?.email && <EmailView next={next} />}
      {methods?.email && methods.github && <div className="text-center text-sm text-slate-500">or</div>}
      {methods?.github && <GitHubView next={next} />}
    </div>
  );
}
