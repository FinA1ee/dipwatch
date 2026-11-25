import {
  api,
  type DoneEvent,
  fail,
  type OffersEvent,
  type ProductPage,
  type Schemas,
  type SearchResult,
} from "./api";

export type Store = Schemas["Store"];

/** Stores a search can use, and how many one search may pick. */
export async function getStores(): Promise<{ stores: Store[]; perSearch: number }> {
  const { data, error, response } = await api.GET("/stores");
  if (!data) fail(response, error);
  return { stores: data.stores, perSearch: data.per_search };
}

export async function search(query: string, currency: string, stores: string[]): Promise<SearchResult> {
  const { data, error, response } = await api.POST("/search", { body: { query, currency, stores } });
  if (!data) fail(response, error);
  return data;
}

export async function getProduct(productId: number, currency: string): Promise<ProductPage> {
  const { data, error, response } = await api.GET("/products/{productId}", {
    params: { path: { productId }, query: { currency } },
  });
  if (!data) fail(response, error);
  return data;
}

export type StreamHandlers = {
  onOffers: (event: OffersEvent) => void;
  onDone: (event: DoneEvent) => void;
};

/**
 * Follows a refresh: each `offers` event carries the full ranked list, so the
 * caller replaces its list. Returns a function that closes the stream.
 */
export function streamOffers(
  productId: number,
  currency: string,
  stores: string[],
  handlers: StreamHandlers,
): () => void {
  const params = new URLSearchParams({ currency });
  for (const s of stores) params.append("stores", s);
  const source = new EventSource(`/api/search/${productId}/events?${params}`);
  source.addEventListener("offers", (e) => handlers.onOffers(JSON.parse((e as MessageEvent<string>).data)));
  source.addEventListener("done", (e) => {
    source.close(); // otherwise EventSource reconnects when the server ends the stream
    handlers.onDone(JSON.parse((e as MessageEvent<string>).data));
  });
  source.onerror = () => {
    if (source.readyState === EventSource.CLOSED) handlers.onDone({ timed_out: true });
  };
  return () => source.close();
}
