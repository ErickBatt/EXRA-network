"use client"

import { useEffect, useState, useRef } from "react"
import WebApp from "@twa-dev/sdk"
import { motion, AnimatePresence } from "framer-motion"
import {
  Smartphone, Monitor, Router, X, ArrowUpRight, RefreshCw, Plus, Zap,
  Check, AlertCircle, Wifi, Tag, PauseCircle, PlayCircle, Trash2, List
} from "lucide-react"
import LavaHero from "@/components/LavaHero"
import "./tma.css"

const TMA_PROXY = "/next-tma"

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers || {})
  if (!headers.has("Content-Type") && init?.body) headers.set("Content-Type", "application/json")
  // credentials:"include" ensures the HttpOnly session cookie (exra_tma_session)
  // is sent to the Next.js proxy which forwards it to Go backend.
  const res = await fetch(`${TMA_PROXY}${path}`, { ...init, headers, cache: "no-store", credentials: "include" })

  if (!res.ok) {
    let msg = `API ${res.status}`
    try {
      const data = await res.json()
      if (data && data.error) msg = data.error
    } catch {}
    throw new Error(msg)
  }
  return res.json() as Promise<T>
}

interface Device {
  device_id: string
  pending_usd: number
  batched_usd: number
  device_type: string
  country: string
  status: string
  identity_tier: string
  rs_mult: number
}

interface Listing {
  id: string
  device_id: string
  price_per_gb: number
  bandwidth_mbps: number
  gear_score: number
  identity_tier: string
  status: string
  pop_sessions: number
  updated_at: string
  node_status: string
}

interface Account {
  telegram_id: number
  first_name: string
  username: string
  devices: Device[]
  pending_usd: number
  withdrawable_usd: number
  total_earned_usd: number
  global_pause: boolean
}

type Toast = { id: number; kind: "success" | "error" | "info"; text: string }

/** Seeded sparkline — deterministic per device so UI doesn't jitter on re-render. */
function Sparkline({ seed, color = "#67e8f9", width = 44, height = 22, points = 16 }: {
  seed: string; color?: string; width?: number; height?: number; points?: number
}) {
  // Cheap deterministic PRNG from seed hash
  let h = 2166136261
  for (let i = 0; i < seed.length; i++) { h ^= seed.charCodeAt(i); h = Math.imul(h, 16777619) }
  const rand = () => { h = Math.imul(h ^ (h >>> 15), h | 1); h ^= h + Math.imul(h ^ (h >>> 7), h | 61); return ((h ^ (h >>> 14)) >>> 0) / 4294967296 }

  const values: number[] = []
  let v = 0.5
  for (let i = 0; i < points; i++) {
    v += (rand() - 0.45) * 0.22
    v = Math.max(0.05, Math.min(0.95, v))
    values.push(v)
  }
  const step = width / (points - 1)
  const path = values.map((val, i) => `${i === 0 ? "M" : "L"}${(i * step).toFixed(1)},${(height - val * height).toFixed(1)}`).join(" ")
  const area = `${path} L${width},${height} L0,${height} Z`
  const id = `sp-${seed.replace(/[^a-z0-9]/gi, "").slice(0, 8)}`

  return (
    <svg className="device-sparkline" width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden>
      <defs>
        <linearGradient id={id} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.35" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${id})`} />
      <path d={path} fill="none" stroke={color} strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function deviceIconFor(type?: string) {
  const t = (type || "").toLowerCase()
  if (t.includes("phone")) return { Icon: Smartphone, variant: "cyan" as const }
  if (t.includes("router") || t.includes("pi")) return { Icon: Router, variant: "green" as const }
  return { Icon: Monitor, variant: "violet" as const }
}

export default function TMAApp() {
  const [account, setAccount] = useState<Account | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showLinkDevice, setShowLinkDevice] = useState(false)
  const [linkDeviceId, setLinkDeviceId] = useState("")
  const [linkLoading, setLinkLoading] = useState(false)
  const [linkError, setLinkError] = useState("")
  const [showWithdraw, setShowWithdraw] = useState(false)
  const [withdrawWallet, setWithdrawWallet] = useState("")
  const [withdrawAmount, setWithdrawAmount] = useState("5.00")
  const [withdrawLoading, setWithdrawLoading] = useState(false)
  const [epoch, setEpoch] = useState<any>(null)
  const [isWaitingApproval, setIsWaitingApproval] = useState(false)
  const [, setLinkRequestId] = useState("")
  const [toasts, setToasts] = useState<Toast[]>([])
  const toastIdRef = useRef(0)

  // Marketplace listings state
  const [listings, setListings] = useState<Listing[]>([])
  const [showCreateLot, setShowCreateLot] = useState(false)
  const [createLotDeviceId, setCreateLotDeviceId] = useState("")
  const [createLotPrice, setCreateLotPrice] = useState("0.05")
  const [createLotBw, setCreateLotBw] = useState("100")
  const [lotLoading, setLotLoading] = useState(false)

  const pushToast = (kind: Toast["kind"], text: string, ttl = 3200) => {
    const id = ++toastIdRef.current
    setToasts(t => [...t, { id, kind, text }])
    setTimeout(() => setToasts(t => t.filter(x => x.id !== id)), ttl)
  }

  const haptic = (type: "light" | "medium" | "heavy" | "success" | "error" = "light") => {
    try {
      if (type === "success") WebApp.HapticFeedback.notificationOccurred("success")
      else if (type === "error") WebApp.HapticFeedback.notificationOccurred("error")
      else WebApp.HapticFeedback.impactOccurred(type)
    } catch {}
  }

  useEffect(() => {
    if (typeof window === "undefined") return
    WebApp.ready()
    WebApp.expand()
    WebApp.setHeaderColor("#09090b")
    authenticate().then(loadListings)
  }, [])

  useEffect(() => {
    if (!isWaitingApproval) return
    const interval = setInterval(silentAuthenticate, 3000)
    return () => clearInterval(interval)
  }, [isWaitingApproval])

  const silentAuthenticate = async () => {
    try {
      const initData = WebApp.initData
      if (!initData) return
      const acc = await apiFetch<any>("/auth", {
        method: "POST",
        body: JSON.stringify({ init_data: initData }),
      })
      setAccount(acc)
      if (acc.devices.some((d: any) => d.device_id === linkDeviceId)) {
        setIsWaitingApproval(false)
        setShowLinkDevice(false)
        setLinkDeviceId("")
        haptic("success")
        pushToast("success", "Device linked")
      }
    } catch (e) {
      console.error("Polling error", e)
    }
  }

  const authenticate = async () => {
    setLoading(true)
    setError(null)
    try {
      const initData = WebApp.initData
      if (!initData) {
        setError("Open this app from Telegram")
        setLoading(false)
        return
      }
      const [acc, ep] = await Promise.all([
        apiFetch<any>("/auth", { method: "POST", body: JSON.stringify({ init_data: initData }) }),
        apiFetch<any>("/epoch"),
      ])
      setAccount(acc)
      setEpoch(ep)
    } catch {
      setError("Failed to load account")
    } finally {
      setLoading(false)
    }
  }

  const handleRefresh = () => {
    haptic("light")
    authenticate()
    loadListings()
  }

  const loadListings = async () => {
    try {
      const data = await apiFetch<{ listings: Listing[] }>("/lots")
      setListings(data.listings || [])
    } catch {
      // silent — listings are secondary to device dashboard
    }
  }

  const handleCreateLot = async () => {
    if (!createLotDeviceId) return
    setLotLoading(true)
    try {
      await apiFetch("/lots/create", {
        method: "POST",
        body: JSON.stringify({
          device_id: createLotDeviceId,
          price_per_gb: parseFloat(createLotPrice),
          bandwidth_mbps: parseInt(createLotBw, 10),
        }),
      })
      setShowCreateLot(false)
      haptic("success")
      pushToast("success", "Listing created")
      loadListings()
    } catch (err: any) {
      haptic("error")
      pushToast("error", err.message)
    } finally {
      setLotLoading(false)
    }
  }

  const handleLotAction = async (id: string, action: "pause" | "resume" | "delete") => {
    haptic("light")
    try {
      if (action === "delete") {
        await apiFetch(`/lots/${id}`, { method: "DELETE" })
      } else {
        await apiFetch(`/lots/${id}/${action}`, { method: "POST" })
      }
      haptic("success")
      pushToast("success", `Listing ${action}d`)
      loadListings()
    } catch (err: any) {
      haptic("error")
      pushToast("error", err.message)
    }
  }

  const handleLinkDevice = async () => {
    if (!account || !linkDeviceId.trim()) return
    setLinkLoading(true)
    setLinkError("")
    try {
      const initData = WebApp.initData
      if (!initData) {
        setLinkError("Please reopen the mini app — no Telegram session")
        setLinkLoading(false)
        return
      }
      // We send the signed Telegram initData — the server derives telegram_id
      // and display name from it, so the caller cannot spoof another user.
      const res = await apiFetch<any>("/link-device", {
        method: "POST",
        body: JSON.stringify({
          init_data: initData,
          device_id: linkDeviceId.trim(),
        }),
      })
      if (res.status === "pending") {
        setIsWaitingApproval(true)
        setLinkRequestId(res.request_id)
      } else {
        setShowLinkDevice(false)
        setLinkDeviceId("")
        haptic("success")
        pushToast("success", "Device linked")
        authenticate()
      }
    } catch (err: any) {
      setLinkError(err.message)
      haptic("error")
    } finally {
      setLinkLoading(false)
    }
  }

  const handleWithdraw = async () => {
    if (!account || !withdrawWallet) return
    const device = account.devices[0]
    if (!device) { pushToast("error", "No device linked"); return }
    setWithdrawLoading(true)
    try {
      await apiFetch("/withdraw", {
        method: "POST",
        body: JSON.stringify({
          device_id: device.device_id,
          amount_usd: parseFloat(withdrawAmount),
          recipient_wallet: withdrawWallet,
        }),
      })
      setShowWithdraw(false)
      haptic("success")
      pushToast("success", "Withdrawal submitted — usually within 24h")
      authenticate()
    } catch (err: any) {
      haptic("error")
      pushToast("error", "Withdrawal failed: " + err.message)
    } finally {
      setWithdrawLoading(false)
    }
  }

  // ===== LOADING =====
  if (loading) {
    return (
      <div className="tma-root">
        <div className="tma-splash">
          <div className="splash-logo">
            <Zap size={28} strokeWidth={2.4} />
          </div>
          <div className="splash-label">Syncing with peaq</div>
        </div>
      </div>
    )
  }

  // ===== ERROR =====
  if (error) {
    return (
      <div className="tma-root">
        <div className="tma-splash">
          <div className="splash-logo" style={{ background: "rgba(239,68,68,0.12)", boxShadow: "0 0 20px rgba(239,68,68,0.3)", color: "#ef4444" }}>
            <AlertCircle size={28} strokeWidth={2.4} />
          </div>
          <div style={{ fontSize: 13.5, color: "var(--ink-muted)", marginBottom: 12, lineHeight: 1.5 }}>{error}</div>
          <button className="btn-primary" onClick={authenticate}>
            <RefreshCw size={14} />
            Retry
          </button>
        </div>
      </div>
    )
  }

  if (!account) return null

  const onlineCount = account.devices.filter(d => d.status === "online").length

  return (
    <div className="tma-root">
      {/* HEADER */}
      <header className="tma-header">
        <div className="tma-avatar">{account.first_name?.charAt(0).toUpperCase() || "E"}</div>
        <div className="tma-title-group">
          <div className="tma-title">{account.first_name || "EXRA"}</div>
          <div className="tma-subtitle">
            <Wifi size={10} strokeWidth={2.2} style={{ color: onlineCount > 0 ? "var(--success)" : "var(--ink-ghost)" }} />
            {account.devices.length} {account.devices.length === 1 ? "node" : "nodes"} · {onlineCount} online
          </div>
        </div>
        <button className="tma-back-btn" aria-label="Close" onClick={() => WebApp.close()}>
          <X size={16} />
        </button>
      </header>

      {/* WITHDRAW MODAL */}
      <AnimatePresence>
        {showWithdraw && (
          <motion.div
            key="modal-withdraw"
            className="tma-modal"
            onClick={(e) => e.target === e.currentTarget && setShowWithdraw(false)}
            initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
          >
            <motion.div
              className="tma-modal-card"
              initial={{ y: 40 }} animate={{ y: 0 }} exit={{ y: 40 }}
              transition={{ type: "spring", damping: 28, stiffness: 300 }}
            >
              <div className="modal-header">
                <span className="modal-title">Withdraw</span>
                <button className="modal-close" onClick={() => setShowWithdraw(false)}>✕</button>
              </div>
              <div className="modal-body">
                <div className="api-label">TON wallet address</div>
                <div className="input-wrap">
                  <input type="text" placeholder="UQ…" value={withdrawWallet} onChange={e => setWithdrawWallet(e.target.value)} />
                </div>
                <div className="api-label mt-md">Amount (USD)</div>
                <div className="input-wrap">
                  <input type="number" value={withdrawAmount} onChange={e => setWithdrawAmount(e.target.value)} />
                </div>
                <button className="btn-primary w-full mt-lg" onClick={handleWithdraw} disabled={withdrawLoading}>
                  {withdrawLoading ? (<><div className="spinner" /> Processing…</>) : (<>Confirm withdrawal <ArrowUpRight size={15} /></>)}
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}

        {/* CREATE LOT MODAL */}
        {showCreateLot && (
          <motion.div
            key="modal-create-lot"
            className="tma-modal"
            onClick={(e) => e.target === e.currentTarget && setShowCreateLot(false)}
            initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
          >
            <motion.div
              className="tma-modal-card"
              initial={{ y: 40 }} animate={{ y: 0 }} exit={{ y: 40 }}
              transition={{ type: "spring", damping: 28, stiffness: 300 }}
            >
              <div className="modal-header">
                <span className="modal-title">List capacity</span>
                <button className="modal-close" onClick={() => setShowCreateLot(false)}>✕</button>
              </div>
              <div className="modal-body">
                <div className="api-label">Device</div>
                <div className="input-wrap">
                  <select
                    value={createLotDeviceId}
                    onChange={e => setCreateLotDeviceId(e.target.value)}
                    style={{ width: "100%", background: "var(--surface)", color: "var(--ink)", border: "1px solid var(--border)", borderRadius: 8, padding: "8px 12px" }}
                  >
                    <option value="">Select device…</option>
                    {account?.devices.map(d => (
                      <option key={d.device_id} value={d.device_id}>
                        {d.device_id.substring(0, 12)}… · {d.device_type || "unknown"} · {d.status}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="api-label mt-md">Price ($/GB)</div>
                <div className="input-wrap">
                  <input type="number" step="0.01" min="0.001" max="100"
                    value={createLotPrice} onChange={e => setCreateLotPrice(e.target.value)} />
                </div>
                <div className="api-label mt-md">Bandwidth (Mbps)</div>
                <div className="input-wrap">
                  <input type="number" min="1" max="10000"
                    value={createLotBw} onChange={e => setCreateLotBw(e.target.value)} />
                </div>
                <div style={{ fontSize: 11.5, color: "var(--ink-ghost)", marginTop: 10, lineHeight: 1.5 }}>
                  Requires ≥3 completed PoP sessions. Listing is visible to buyers immediately.
                </div>
                <button className="btn-primary w-full mt-lg" onClick={handleCreateLot}
                  disabled={lotLoading || !createLotDeviceId}>
                  {lotLoading ? (<><div className="spinner" /> Creating…</>) : (<><Tag size={14} /> Publish listing</>)}
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}

        {/* LINK DEVICE MODAL */}
        {showLinkDevice && (
          <motion.div
            key="modal-link"
            className="tma-modal"
            onClick={(e) => { if (e.target === e.currentTarget) { setShowLinkDevice(false); setLinkError("") } }}
            initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
          >
            <motion.div
              className="tma-modal-card"
              initial={{ y: 40 }} animate={{ y: 0 }} exit={{ y: 40 }}
              transition={{ type: "spring", damping: 28, stiffness: 300 }}
            >
              <div className="modal-header">
                <span className="modal-title">Add device</span>
                <button className="modal-close" onClick={() => { setShowLinkDevice(false); setLinkError("") }}>✕</button>
              </div>
              <div className="modal-body">
                {isWaitingApproval ? (
                  <div className="waiting">
                    <div className="waiting-logo"><Zap size={26} strokeWidth={2.4} /></div>
                    <div className="waiting-title">Waiting for approval</div>
                    <div className="waiting-sub">
                      A request has been sent to your device. Open the EXRA app and tap{" "}
                      <strong style={{ color: "var(--neon-bright)" }}>Approve</strong>.
                    </div>
                    <button className="btn-secondary mt-md" onClick={() => setIsWaitingApproval(false)}>Cancel</button>
                  </div>
                ) : (
                  <>
                    <div style={{ fontSize: 12.5, color: "var(--ink-muted)", marginBottom: 14, lineHeight: 1.55 }}>
                      Open the EXRA app on your device → Settings → copy Device ID.
                    </div>
                    <div className="api-label">Device ID</div>
                    <div className="input-wrap">
                      <input type="text" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" value={linkDeviceId} onChange={e => setLinkDeviceId(e.target.value)} />
                    </div>
                    {linkError && <div className="error-text">{linkError}</div>}
                    <button className="btn-primary w-full mt-md" onClick={handleLinkDevice} disabled={linkLoading}>
                      {linkLoading ? (<><div className="spinner" /> Linking…</>) : (<>Link device <ArrowUpRight size={15} /></>)}
                    </button>
                  </>
                )}
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="scroll-area">
        {/* PEAQ BADGE — network trust indicator */}
        <div className="peaq-strip">
          <div className="peaq-badge">
            <span className="peaq-badge-dot" />
            Live on peaq · Mainnet
          </div>
        </div>

        {/* HERO with bento-ticker */}
        <LavaHero
          totalEarned={account.total_earned_usd.toFixed(2)}
          nodesOnline={onlineCount}
          exraPrice={1.5}
          dailyRate={account.total_earned_usd > 0 ? account.total_earned_usd / 30 : 0}
          rank={account.telegram_id % 10000}
        />

        {/* PRIMARY CTA ROW — Withdraw is the star, Refresh/Add are satellites */}
        <div className="primary-cta-row">
          <button
            className="cta-primary"
            onClick={() => { haptic("medium"); setShowWithdraw(true) }}
            disabled={account.withdrawable_usd < 1}
          >
            <ArrowUpRight size={17} strokeWidth={2.4} />
            Withdraw ${account.withdrawable_usd.toFixed(2)}
          </button>
          <button className="cta-icon-btn" onClick={handleRefresh} aria-label="Refresh">
            <RefreshCw size={16} strokeWidth={2.2} />
          </button>
          <button className="cta-icon-btn" onClick={() => { haptic("light"); setShowLinkDevice(true) }} aria-label="Add device">
            <Plus size={18} strokeWidth={2.2} />
          </button>
        </div>

        {/* BENTO BALANCE */}
        <div className="bento">
          <motion.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, delay: 0.1 }}
            className="bal-card accent"
          >
            <div className="bal-head">
              <span className="bal-label">$EXRA earned</span>
            </div>
            <div className="bal-val neon tnum">{account.total_earned_usd.toFixed(2)}</div>
            <div className="bal-sub">all-time</div>
            <Sparkline seed={`earned-${account.telegram_id}`} color="#67e8f9" />
          </motion.div>

          <motion.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, delay: 0.18 }}
            className="bal-card accent-violet"
          >
            <div className="bal-head">
              <span className="bal-label">Withdrawable</span>
            </div>
            <div className="bal-val violet tnum">${account.withdrawable_usd.toFixed(2)}</div>
            <div className="bal-sub">ready to claim</div>
            <Sparkline seed={`usd-${account.telegram_id}`} color="#a78bfa" />
          </motion.div>
        </div>

        {/* EPOCH */}
        {epoch && (
          <div className="epoch-bar">
            <div className="epoch-head">
              <span className="epoch-name">{epoch.epoch_name}</span>
              <span className="epoch-remaining tnum">
                {epoch.days_remaining > 0 ? `${epoch.days_remaining}d left` : "∞"}
              </span>
            </div>
            <div className="epoch-track">
              <div className="epoch-fill" style={{ width: `${Math.min(epoch.progress_pct || 0, 100)}%` }} />
            </div>
          </div>
        )}

        {/* DEVICES */}
        <section className="section">
          <div className="section-head">
            <span className="section-title">My devices</span>
            <span className="section-action" onClick={handleRefresh}>refresh</span>
          </div>

          {account.devices.length === 0 && (
            <div className="empty-state">
              <div className="empty-state-icon"><Plus size={22} strokeWidth={2} /></div>
              <div>No devices connected yet.</div>
              <div style={{ marginTop: 14 }}>
                <button className="btn-secondary" onClick={() => { haptic("light"); setShowLinkDevice(true) }}>
                  <Plus size={14} /> Add your first device
                </button>
              </div>
            </div>
          )}

          {account.devices.map((device, i) => {
            const { Icon, variant } = deviceIconFor(device.device_type)
            const sparkColor = variant === "cyan" ? "#67e8f9" : variant === "violet" ? "#a78bfa" : "#10b981"
            const online = device.status === "online"
            return (
              <motion.div
                key={device.device_id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.35, delay: 0.05 * i }}
                className="device-row"
              >
                <div className={`device-icon ${variant}`}>
                  <Icon size={17} strokeWidth={2.1} />
                </div>
                <div className="device-info">
                  <div className="device-name">{device.device_id.substring(0, 10)}…</div>
                  <div className="device-meta">
                    <span className={`device-status ${online ? "online" : "offline"}`} />
                    {device.device_type || "unknown"} · {device.country || "??"}
                  </div>
                </div>
                <Sparkline seed={device.device_id} color={sparkColor} />
                <div className="device-right">
                  <div className="device-balance tnum">${device.pending_usd.toFixed(2)}</div>
                  <div className="device-earned tnum">{device.batched_usd.toFixed(1)} credits</div>
                </div>
              </motion.div>
            )
          })}
        </section>
        {/* MARKETPLACE LISTINGS */}
        <section className="section">
          <div className="section-head">
            <span className="section-title"><List size={13} style={{ display: "inline", verticalAlign: "middle", marginRight: 5 }} />My listings</span>
            <span className="section-action" onClick={() => { haptic("light"); setShowCreateLot(true) }}>
              <Plus size={11} style={{ display: "inline" }} /> list capacity
            </span>
          </div>

          {listings.filter(l => l.status !== "deleted").length === 0 && (
            <div className="empty-state">
              <div className="empty-state-icon"><Tag size={20} strokeWidth={2} /></div>
              <div>No active listings. List a device to earn from marketplace buyers.</div>
              <div style={{ marginTop: 14 }}>
                <button className="btn-secondary" onClick={() => { haptic("light"); setShowCreateLot(true) }}>
                  <Tag size={14} /> Create listing
                </button>
              </div>
            </div>
          )}

          {listings.filter(l => l.status !== "deleted").map((lot, i) => (
            <motion.div
              key={lot.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, delay: 0.04 * i }}
              className="device-row"
              style={{ alignItems: "flex-start", flexDirection: "column", gap: 8 }}
            >
              <div style={{ display: "flex", width: "100%", alignItems: "center", gap: 10 }}>
                <div className={`device-icon ${lot.identity_tier === "peak" ? "violet" : lot.identity_tier === "basic" ? "green" : "cyan"}`}>
                  <Tag size={15} strokeWidth={2.1} />
                </div>
                <div className="device-info" style={{ flex: 1 }}>
                  <div className="device-name">{lot.device_id.substring(0, 12)}…</div>
                  <div className="device-meta">
                    <span className={`device-status ${lot.node_status === "online" ? "online" : "offline"}`} />
                    ${lot.price_per_gb.toFixed(3)}/GB · {lot.bandwidth_mbps} Mbps · {lot.identity_tier}
                  </div>
                </div>
                <div style={{ display: "flex", gap: 6 }}>
                  {lot.status === "active" ? (
                    <button
                      onClick={() => handleLotAction(lot.id, "pause")}
                      style={{ background: "rgba(251,191,36,0.12)", border: "none", borderRadius: 6, padding: "4px 8px", cursor: "pointer", color: "#fbbf24" }}
                      title="Pause"
                    ><PauseCircle size={14} /></button>
                  ) : (
                    <button
                      onClick={() => handleLotAction(lot.id, "resume")}
                      style={{ background: "rgba(16,185,129,0.12)", border: "none", borderRadius: 6, padding: "4px 8px", cursor: "pointer", color: "#10b981" }}
                      title="Resume"
                    ><PlayCircle size={14} /></button>
                  )}
                  <button
                    onClick={() => handleLotAction(lot.id, "delete")}
                    style={{ background: "rgba(239,68,68,0.1)", border: "none", borderRadius: 6, padding: "4px 8px", cursor: "pointer", color: "#ef4444" }}
                    title="Delete"
                  ><Trash2 size={14} /></button>
                </div>
              </div>
              <div style={{ fontSize: 11, color: "var(--ink-ghost)", paddingLeft: 42 }}>
                {lot.pop_sessions} PoP sessions · GearScore {lot.gear_score.toFixed(2)} ·
                {" "}<span style={{ color: lot.status === "active" ? "#10b981" : "#fbbf24" }}>{lot.status}</span>
              </div>
            </motion.div>
          ))}
        </section>
      </div>

      {/* TOASTS */}
      <div className="toast-wrap">
        <AnimatePresence>
          {toasts.map(t => (
            <motion.div
              key={t.id}
              className={`toast ${t.kind}`}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 10, transition: { duration: 0.2 } }}
              transition={{ type: "spring", damping: 28, stiffness: 400 }}
            >
              <div className="toast-icon">
                {t.kind === "success" && <Check size={14} strokeWidth={2.6} />}
                {t.kind === "error" && <AlertCircle size={14} strokeWidth={2.4} />}
                {t.kind === "info" && <Zap size={14} strokeWidth={2.4} />}
              </div>
              {t.text}
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </div>
  )
}
