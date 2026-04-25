"use server";

const VALID_ROLES = new Set(["tester", "investor", "buyer", "ghost"]);
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;

const ROLE_LABELS: Record<string, string> = {
  tester:   "Tester (Beta)",
  investor: "Investor",
  buyer:    "Buyer / Resident IP",
  ghost:    "Ghost Node 👻",
};

export interface WaitlistInput {
  role: string;
  name: string;
  email: string;
  telegram: string;
  deviceType: string;
  country: string;
  useCase: string;
}

export async function submitWaitlist(
  input: WaitlistInput
): Promise<{ ok: boolean; error?: string }> {
  const { role, name, email, telegram, deviceType, country, useCase } = input;

  if (!VALID_ROLES.has(role)) return { ok: false, error: "Invalid role." };
  if (!name.trim() || name.trim().length > 100) return { ok: false, error: "Name is required." };
  if (!EMAIL_RE.test(email.trim())) return { ok: false, error: "Valid email is required." };

  const supabaseUrl = process.env.SUPABASE_URL;
  const supabaseKey = process.env.SUPABASE_SERVICE_ROLE_KEY;
  if (!supabaseUrl || !supabaseKey) {
    console.error("[waitlist] missing SUPABASE_URL or SUPABASE_SERVICE_ROLE_KEY");
    return { ok: false, error: "Server configuration error." };
  }

  // ── Insert into Supabase ────────────────────────────────────────────────
  let res: Response;
  try {
    res = await fetch(`${supabaseUrl}/rest/v1/waitlist`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        apikey: supabaseKey,
        Authorization: `Bearer ${supabaseKey}`,
        Prefer: "return=minimal",
      },
      body: JSON.stringify({
        name: name.trim(),
        email: email.trim().toLowerCase(),
        role,
        telegram: telegram.trim() || null,
        device_type: deviceType || null,
        country: country.trim() || null,
        use_case: useCase || null,
        is_ghost: role === "ghost",
      }),
    });
  } catch (err) {
    console.error("[waitlist] supabase fetch error:", err);
    return { ok: false, error: "Network error. Please try again." };
  }

  if (res.status === 409) return { ok: false, error: "This email is already on the list." };
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    console.error("[waitlist] supabase error:", res.status, body);
    return { ok: false, error: "Failed to join. Please try again." };
  }

  // ── Email notification via Resend ────────────────────────────────────────
  const resendKey = process.env.RESEND_API_KEY;
  if (resendKey) {
    const extras = [
      telegram  && `Telegram: ${telegram}`,
      deviceType && `Device: ${deviceType}`,
      country   && `Country: ${country}`,
      useCase   && `Use case / stage: ${useCase}`,
    ]
      .filter(Boolean)
      .map(l => `<li>${l}</li>`)
      .join("");

    const html = `
      <div style="font-family:monospace;background:#09090b;color:#fafafa;padding:32px;border-radius:12px;max-width:480px">
        <div style="color:#22d3ee;font-size:11px;letter-spacing:0.2em;text-transform:uppercase;margin-bottom:12px">
          exra.space / waitlist
        </div>
        <h2 style="margin:0 0 16px;font-size:18px">New ${ROLE_LABELS[role] ?? role} signup</h2>
        <ul style="padding-left:18px;line-height:1.8;color:#a1a1aa;margin:0">
          <li>Name: <strong style="color:#fafafa">${name.trim()}</strong></li>
          <li>Email: <strong style="color:#fafafa">${email.trim().toLowerCase()}</strong></li>
          <li>Role: <strong style="color:#22d3ee">${ROLE_LABELS[role] ?? role}</strong></li>
          ${extras}
        </ul>
      </div>`;

    fetch("https://api.resend.com/emails", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${resendKey}`,
      },
      body: JSON.stringify({
        from: "EXRA Network <noreply@exra.space>",
        to: ["ilya.khotin@exra.space"],
        subject: `[Waitlist] ${ROLE_LABELS[role] ?? role} — ${name.trim()}`,
        html,
      }),
    }).catch(err => console.error("[waitlist] resend error:", err));
    // fire-and-forget — don't block the response on email delivery
  }

  return { ok: true };
}
