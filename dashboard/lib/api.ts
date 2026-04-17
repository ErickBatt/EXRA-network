// Empty string = relative URL (works via nginx: exra.space/api/* → Go backend).
// Override with NEXT_PUBLIC_API_BASE_URL only in local dev.
const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL || "";

export type BuyerProfile = {
  id: string;
  api_key: string;
  balance_usd: number;
  created_at: string;
};

export async function fetchJson<T>(path: string, token?: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers || {});
  if (token) headers.set("X-Exra-Token", token);
  if (!headers.has("Content-Type") && init?.body) headers.set("Content-Type", "application/json");

  const res = await fetch(`${baseUrl}${path}`, { ...init, headers, cache: "no-store" });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API ${res.status}: ${text}`);
  }
  return res.json() as Promise<T>;
}

export async function registerBuyer(adminSecret: string): Promise<BuyerProfile> {
  return fetchJson<BuyerProfile>("/api/buyer/register", adminSecret, { method: "POST" });
}

export async function settlePayment(adminSecret: string, buyerId: string, inputCurrency: "USDT" | "EXRA", inputAmount: number) {
  return fetchJson("/api/tokenomics/payments/settle", adminSecret, {
    method: "POST",
    body: JSON.stringify({ buyer_id: buyerId, input_currency: inputCurrency, input_amount: inputAmount })
  });
}
