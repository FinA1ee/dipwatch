import { Link } from "react-router";
import { ApiError } from "@/services/api";

/** Error box for admin pages: explains a 403 instead of showing a raw message. */
export function AdminError({ error }: { error: Error }) {
  const forbidden = error instanceof ApiError && error.status === 403;
  return (
    <p className="rounded-lg bg-rose-50 p-3 text-rose-800">
      {forbidden ? (
        <>
          Admins only. <Link to="/signin?next=/admin">Sign in</Link> with an admin email.
        </>
      ) : (
        error.message
      )}
    </p>
  );
}
