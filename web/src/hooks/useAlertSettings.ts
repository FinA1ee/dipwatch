import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getSettings, saveSettings, sendTestMessage } from "@/services/tracking";

const KEY = ["alert-settings"];

export function useAlertSettings() {
  const client = useQueryClient();
  const query = useQuery({ queryKey: KEY, queryFn: getSettings, retry: false });
  const save = useMutation({
    mutationFn: (v: { feishu: string; email: boolean }) => saveSettings(v.feishu, v.email),
    onSuccess: (data) => client.setQueryData(KEY, data),
  });
  const test = useMutation({ mutationFn: sendTestMessage });
  return { settings: query.data, loading: query.isPending, error: query.error, save, test };
}
