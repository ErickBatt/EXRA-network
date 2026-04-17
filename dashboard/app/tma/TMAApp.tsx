"use client"

import { useEffect, useState } from "react"
import WebApp from "@twa-dev/sdk"
import { motion } from "framer-motion"
import { Smartphone, Monitor, Zap, X, ArrowUpRight, RefreshCw, Plus } from "lucide-react"
import LavaHero from "@/components/LavaHero"
import "./tma.css"

// TMA calls go through Next.js server-side proxy at /next-tma/*.
// Path avoids /api/ prefix so nginx routes it to Next.js (port 3000),
// not directly to Go backend (port 8080). NODE_SECRET stays server-side only.
const TMA_PROXY = "/next-tma"

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers || {})
  if (!headers.has("Content-Type") && init?.body) headers.set("Content-Type", "application/json")
  const res = await fetch(`${TMA_PROXY}${path}`, { ...init, headers, cache: "no-store" })
  
  if (!res.ok) {
    let msg = `API ${res.status}`
    try {
      const data = await res.json()
      if (data && data.error) msg = data.error
    } catch (e) {
      // Not JSON or no error field
    }
    throw new Error(msg)
  }
  
  return res.json() as Promise<T>
}

interface Device {
  device_id: string
  balance_usd: number
  exra_earned: number
  device_type: string
  country: string
  status: string
}

interface Account {
  telegram_id: number
  first_name: string
  username: string
  devices: Device[]
  total_usd: number
  total_exra: number
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
  const [linkRequestId, setLinkRequestId] = useState("")

  useEffect(() => {
    if (typeof window === "undefined") return
    WebApp.ready()
    WebApp.expand()
    WebApp.setHeaderColor("#111110")
    authenticate()
  }, [])

  useEffect(() => {
    if (!isWaitingApproval) return
    const interval = setInterval(() => {
      // Re-fetch account. If the device appears in the list, it means it's linked.
      silentAuthenticate()
    }, 3000)
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
      // Check if the device we're waiting for is now in the list
      if (acc.devices.some((d: any) => d.device_id === linkDeviceId)) {
        setIsWaitingApproval(false)
        setShowLinkDevice(false)
        setLinkDeviceId("")
        WebApp.HapticFeedback.notificationOccurred("success")
        WebApp.showAlert("Device linked successfully!")
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
        apiFetch<any>("/auth", {
          method: "POST",
          body: JSON.stringify({ init_data: initData }),
        }),
        apiFetch<any>("/epoch"),
      ])
      setAccount(acc)
      setEpoch(ep)
    } catch (err: any) {
      setError("Failed to load account")
    } finally {
      setLoading(false)
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
        WebApp.HapticFeedback.notificationOccurred("success")
        authenticate()
      }
    } catch (err: any) {
      setLinkError(err.message)
    } finally {
      setLinkLoading(false)
    }
  }

  const handleWithdraw = async () => {
    if (!account || !withdrawWallet) return
    const device = account.devices[0]
    if (!device) return alert("No device linked")
    setWithdrawLoading(true)
    try {
      // /api/tma/withdraw is nodeAuth-protected — correct TMA withdrawal endpoint
      await apiFetch("/withdraw", {
        method: "POST",
        body: JSON.stringify({
          device_id: device.device_id,
          amount_usd: parseFloat(withdrawAmount),
          recipient_wallet: withdrawWallet,
        }),
      })
      setShowWithdraw(false)
      WebApp.showAlert("Withdrawal submitted! Usually processed within 24h.")
      WebApp.HapticFeedback.notificationOccurred("success")
      authenticate()
    } catch (err: any) {
      alert("Withdrawal failed: " + err.message)
    } finally {
      setWithdrawLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="tma-root" style={{ alignItems: "center", justifyContent: "center" }}>
        <div style={{ textAlign: "center" }}>
          <div style={{ fontSize: "40px", marginBottom: "16px" }}>⚡</div>
          <div style={{ fontSize: "13px", color: "var(--ink-dim)" }}>Loading...</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="tma-root" style={{ alignItems: "center", justifyContent: "center", padding: "32px" }}>
        <div style={{ textAlign: "center" }}>
          <div style={{ fontSize: "40px", marginBottom: "16px" }}>⚠️</div>
          <div style={{ fontSize: "14px", color: "#ff4444", marginBottom: "16px" }}>{error}</div>
          <button className="primary-btn" onClick={authenticate}>retry</button>
        </div>
      </div>
    )
  }

  if (!account) return null

  return (
    <div className="tma-root">
      {/* HEADER */}
      <header className="tma-header">
        <div className="tma-avatar">
          {account.first_name?.charAt(0) || "E"}
        </div>
        <div className="tma-title-group">
          <div className="tma-title">{account.first_name || "EXRA"}</div>
          <div className="tma-subtitle">
            {account.devices.length} device{account.devices.length !== 1 ? "s" : ""} connected
          </div>
        </div>
        <button className="tma-back-btn" onClick={() => WebApp.close()}>
          <X size={18} color="var(--ink-dim)" />
        </button>
      </header>

      {/* WITHDRAW MODAL */}
      {showWithdraw && (
        <div className="tma-modal">
          <div className="tma-modal-card">
            <div className="modal-header">
              <span className="modal-title">Withdraw</span>
              <button className="modal-close" onClick={() => setShowWithdraw(false)}>✕</button>
            </div>
            <div className="modal-body">
              <div className="api-label">TON WALLET ADDRESS</div>
              <div className="input-wrap">
                <input type="text" placeholder="UQ..." value={withdrawWallet} onChange={e => setWithdrawWallet(e.target.value)} />
              </div>
              <div className="api-label" style={{ marginTop: "12px" }}>AMOUNT (USD)</div>
              <div className="input-wrap">
                <input type="number" value={withdrawAmount} onChange={e => setWithdrawAmount(e.target.value)} />
              </div>
              <button className="primary-btn" style={{ marginTop: "20px", width: "100%" }} onClick={handleWithdraw} disabled={withdrawLoading}>
                {withdrawLoading ? "processing..." : "confirm withdrawal →"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* LINK DEVICE MODAL */}
      {showLinkDevice && (
        <div className="tma-modal">
          <div className="tma-modal-card">
            <div className="modal-header">
              <span className="modal-title">Add Device</span>
              <button className="modal-close" onClick={() => { setShowLinkDevice(false); setLinkError("") }}>✕</button>
            </div>
            <div className="modal-body">
              {isWaitingApproval ? (
                <div style={{ textAlign: "center", padding: "20px 0" }}>
                  <div style={{ animation: "spin 2s linear infinite", fontSize: "32px", marginBottom: "16px" }}>⏳</div>
                  <div style={{ fontSize: "14px", fontWeight: 600, marginBottom: "8px" }}>Waiting for approval</div>
                  <div style={{ fontSize: "12px", color: "var(--ink-dim)", lineHeight: "1.5" }}>
                    A request has been sent to your device.<br/>
                    Please open the EXRA app and tap <b>Approve</b>.
                  </div>
                  <button className="primary-btn" style={{ marginTop: "24px", width: "100%", background: "none", border: "1px solid var(--border)", color: "var(--ink)" }} onClick={() => setIsWaitingApproval(false)}>
                    cancel
                  </button>
                </div>
              ) : (
                <>
                  <div style={{ fontSize: "12px", color: "var(--ink-dim)", marginBottom: "16px", lineHeight: "1.5" }}>
                    Open the EXRA app on your device → Settings → copy Device ID
                  </div>
                  <div className="api-label">DEVICE ID</div>
                  <div className="input-wrap">
                    <input type="text" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" value={linkDeviceId} onChange={e => setLinkDeviceId(e.target.value)} />
                  </div>
                  {linkError && <div style={{ color: "#ff4444", fontSize: "12px", marginTop: "8px" }}>{linkError}</div>}
                  <button className="primary-btn" style={{ marginTop: "16px", width: "100%" }} onClick={handleLinkDevice} disabled={linkLoading}>
                    {linkLoading ? "linking..." : "link device →"}
                  </button>
                </>
              )}
            </div>
          </div>
        </div>
      )}

      <div className="scroll-area">
        {/* LAVA HERO */}
        <LavaHero totalEarned={account.total_usd.toFixed(2)} nodesOnline={account.devices.filter(d => d.status === "online").length} />

        {/* BALANCE */}
        <div className="balance-split">
          <motion.div initial={{ opacity: 0, x: -20 }} animate={{ opacity: 1, x: 0 }} className="bal-card">
            <div className="bal-label">$EXRA EARNED</div>
            <div className="bal-val accent">{account.total_exra.toFixed(2)}</div>
            <div className="bal-sub">≈ ${account.total_usd.toFixed(2)} USD</div>
          </motion.div>
          <motion.div initial={{ opacity: 0, x: 20 }} animate={{ opacity: 1, x: 0 }} transition={{ delay: 0.1 }} className="bal-card">
            <div className="bal-label">WITHDRAWABLE</div>
            <div className="bal-val">${account.total_usd.toFixed(2)}</div>
            <div className="bal-sub">ready to withdraw</div>
          </motion.div>
        </div>

        {/* EPOCH BAR */}
        {epoch && (
          <div style={{ margin: "0 16px 4px", padding: "12px 16px", background: "var(--surface)", borderRadius: "14px", border: "1px solid var(--border)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "8px" }}>
              <span style={{ fontSize: "11px", color: "var(--ink-muted)", textTransform: "uppercase", letterSpacing: "0.08em" }}>{epoch.epoch_name}</span>
              <span style={{ fontSize: "11px", color: "var(--exra)", fontFamily: "monospace" }}>
                {epoch.days_remaining > 0 ? `${epoch.days_remaining}d left` : "∞"}
              </span>
            </div>
            <div style={{ height: "3px", background: "#1a1a16", borderRadius: "2px" }}>
              <div style={{ height: "100%", width: `${Math.min(epoch.progress_pct || 0, 100)}%`, background: "#c8f03c", borderRadius: "2px", transition: "width 0.5s" }} />
            </div>
          </div>
        )}

        {/* QUICK ACTIONS */}
        <div className="quick-actions">
          <motion.div whileTap={{ scale: 0.95 }} className="qa-card" onClick={authenticate}>
            <div className="qa-icon" style={{ background: "rgba(200,240,60,0.1)", border: "1px solid rgba(200,240,60,0.2)" }}>
              <RefreshCw size={18} color="#c8f03c" />
            </div>
            <div className="qa-label">Refresh</div>
          </motion.div>
          <motion.div whileTap={{ scale: 0.95 }} className="qa-card" onClick={() => { WebApp.HapticFeedback.impactOccurred("medium"); setShowWithdraw(true) }}>
            <div className="qa-icon" style={{ background: "rgba(168,154,255,0.1)", border: "1px solid rgba(168,154,255,0.2)" }}>
              <ArrowUpRight size={18} color="#a89aff" />
            </div>
            <div className="qa-label">Withdraw</div>
          </motion.div>
          <motion.div whileTap={{ scale: 0.95 }} className="qa-card" onClick={() => setShowLinkDevice(true)}>
            <div className="qa-icon" style={{ background: "rgba(74,222,128,0.1)", border: "1px solid rgba(74,222,128,0.2)" }}>
              <Plus size={18} color="#4ade80" />
            </div>
            <div className="qa-label">Add Device</div>
          </motion.div>
        </div>

        {/* DEVICES */}
        <section className="section">
          <div className="section-header">
            <span className="section-title">MY DEVICES</span>
            <span className="section-action" onClick={authenticate} style={{ fontSize: "11px", color: "var(--ink-muted)", cursor: "pointer" }}>refresh</span>
          </div>
          {account.devices.length === 0 && (
            <div style={{ textAlign: "center", padding: "32px 16px", color: "var(--ink-muted)", fontSize: "13px" }}>
              No devices connected yet.{"\n"}
              <button onClick={() => setShowLinkDevice(true)} style={{ marginTop: "12px", background: "none", border: "1px solid var(--border)", color: "var(--exra)", padding: "8px 20px", borderRadius: "10px", cursor: "pointer", fontSize: "12px" }}>
                + Add your first device
              </button>
            </div>
          )}
          {account.devices.map(device => (
            <div key={device.device_id} className="device-row">
              <div className="device-icon">
                {device.device_type?.toLowerCase().includes("phone") ? (
                  <Smartphone size={18} color="#c8f03c" />
                ) : (
                  <Monitor size={18} color="#a89aff" />
                )}
              </div>
              <div className="device-info">
                <div className="device-name">{device.device_id.substring(0, 12)}… · {device.country || "??"}</div>
                <div className="device-meta">{device.device_type || "unknown"}</div>
              </div>
              <div className="device-right" style={{ textAlign: "right" }}>
                <div className="device-status" style={{ background: device.status === "online" ? "#c8f03c" : "#3a3a30", marginLeft: "auto", marginBottom: "4px" }} />
                <div style={{ color: "#c8f03c", fontSize: "13px", fontWeight: 500 }}>${device.balance_usd.toFixed(2)}</div>
                <div style={{ fontSize: "10px", color: "var(--ink-muted)" }}>{device.exra_earned.toFixed(1)} EXRA</div>
              </div>
            </div>
          ))}
        </section>
      </div>
    </div>
  )
}
