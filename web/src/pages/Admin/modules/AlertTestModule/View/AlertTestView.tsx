import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { simulateDrop } from "@/services/admin";

/**
 * Lowers the cheapest stored offer of a product and runs the alert check, to
 * test delivery end to end (M6.3). It changes stored prices: use a test product.
 */
export function AlertTestView() {
  const [productId, setProductId] = useState("");
  const [percent, setPercent] = useState("20");
  const run = useMutation({ mutationFn: () => simulateDrop(Number(productId), Number(percent)) });
  return (
    <form
      className="flex flex-wrap items-end gap-3 text-sm"
      onSubmit={(e) => {
        e.preventDefault();
        run.mutate();
      }}
    >
      <label className="space-y-1">
        <span className="block font-medium">Product id</span>
        <input
          inputMode="numeric"
          value={productId}
          onChange={(e) => setProductId(e.target.value)}
          placeholder="from /p/<id>"
          className="w-32 rounded border border-rule bg-white px-2 py-1"
        />
      </label>
      <label className="space-y-1">
        <span className="block font-medium">Drop %</span>
        <input
          inputMode="numeric"
          value={percent}
          onChange={(e) => setPercent(e.target.value)}
          className="w-20 rounded border border-rule bg-white px-2 py-1"
        />
      </label>
      <button
        type="submit"
        disabled={run.isPending || !Number(productId)}
        className="rounded-lg border border-rule bg-white px-3 py-1.5 font-medium hover:border-muted disabled:opacity-50"
      >
        Simulate a price drop
      </button>
      {run.isSuccess && (
        <span className="text-drop">Queued. A matching tracker gets its alert within a minute.</span>
      )}
      {run.isError && <span className="text-rose-700">{run.error.message}</span>}
    </form>
  );
}
