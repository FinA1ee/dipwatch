import { useQuery } from "@tanstack/react-query";
import { signInMethods } from "@/services/auth";

/** Shared by the page and its modules: one request, cached. */
export function useSignInMethods() {
  const query = useQuery({ queryKey: ["sign-in-methods"], queryFn: signInMethods, staleTime: Infinity });
  return { methods: query.data, loading: query.isPending };
}
