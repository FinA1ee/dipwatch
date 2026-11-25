import { useQuery, useQueryClient } from "@tanstack/react-query";
import { currentMember, type Member, signOut } from "@/services/auth";

const KEY = ["member"];

/** The signed-in member, or null for a visitor. */
export function useMember() {
  const query = useQuery({ queryKey: KEY, queryFn: currentMember, staleTime: 60_000, retry: false });
  const client = useQueryClient();
  return {
    member: query.data ?? null,
    loading: query.isPending,
    signOut: async () => {
      await signOut();
      client.setQueryData<Member | null>(KEY, null);
    },
  };
}
