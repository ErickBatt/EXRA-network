"use client";

import React, { useState } from "react";
import { usePeaq } from "@/lib/peaq/PeaqProvider";
import { motion, AnimatePresence } from "framer-motion";
import { Wallet, WalletAccount } from "@talismn/connect-wallets";
import { Wallet as WalletIcon, User, ChevronRight, CheckCircle2, AlertCircle } from "lucide-react";

export const WalletSelector = () => {
  const { isReady, wallets, selectedAccount, setSelectedAccount, error } = usePeaq();
  const [activeWallet, setActiveWallet] = useState<Wallet | null>(null);
  const [accounts, setAccounts] = useState<WalletAccount[]>([]);
  const [isConnecting, setIsConnecting] = useState(false);

  const handleConnectWallet = async (wallet: Wallet) => {
    setIsConnecting(true);
    try {
      await wallet.enable("EXRA Dashboard");
      const walletAccounts = await wallet.getAccounts();
      setAccounts(walletAccounts);
      setActiveWallet(wallet);
    } catch (err) {
      console.error("Failed to connect wallet:", err);
    } finally {
      setIsConnecting(false);
    }
  };

  const handleSelectAccount = (account: WalletAccount) => {
    setSelectedAccount(account);
    setActiveWallet(null);
  };

  if (!isReady) {
    return (
      <div className="flex items-center gap-2 p-3 bg-zinc-900/50 rounded-xl border border-zinc-800 text-zinc-400">
        <div className="w-2 h-2 bg-zinc-600 rounded-full animate-pulse" />
        <span className="text-sm font-medium">Connecting to Peaq Node...</span>
      </div>
    );
  }

  if (selectedAccount) {
    return (
      <div className="flex items-center justify-between p-4 bg-indigo-500/5 backdrop-blur-xl rounded-2xl border border-indigo-500/20 shadow-lg shadow-indigo-500/5 transition-all">
        <div className="flex items-center gap-4">
          <div className="p-2.5 bg-gradient-to-br from-indigo-500 to-violet-600 rounded-xl shadow-inner">
            <User size={20} className="text-white" />
          </div>
          <div>
            <p className="text-[10px] font-bold uppercase tracking-[0.1em] text-indigo-400 mb-0.5 opacity-80">Authenticated</p>
            <p className="text-sm font-black text-zinc-100 truncate max-w-[160px] tracking-tight">
              {selectedAccount.name || `${selectedAccount.address.slice(0, 10)}...`}
            </p>
          </div>
        </div>
        <div className="relative">
          <CheckCircle2 size={18} className="text-emerald-400" />
          <motion.div 
            initial={{ scale: 0.8, opacity: 0 }}
            animate={{ scale: 1.5, opacity: 0 }}
            transition={{ repeat: Infinity, duration: 2 }}
            className="absolute inset-0 bg-emerald-400 rounded-full"
          />
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <AnimatePresence mode="wait">
        {!activeWallet ? (
          <motion.div
            key="wallet-list"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            className="grid gap-2"
          >
            <h3 className="text-sm font-bold text-zinc-400 mb-2 flex items-center gap-2">
              <WalletIcon size={16} /> Select Substrate Wallet
            </h3>
            {wallets.length === 0 ? (
              <div className="p-5 bg-amber-500/5 border border-amber-500/20 rounded-2xl flex items-start gap-4 text-amber-500 backdrop-blur-sm">
                <AlertCircle size={22} className="shrink-0" />
                <p className="text-xs leading-relaxed font-medium">
                  Substrate wallet extension not detected. To participate in the Exra Network, please install <a href="https://talisman.xyz" target="_blank" className="font-bold underline text-amber-400 hover:text-amber-300">Talisman</a> or Polkadot.js.
                </p>
              </div>
            ) : (
              wallets.map((wallet) => (
                <button
                  key={wallet.extensionName}
                  onClick={() => handleConnectWallet(wallet)}
                  disabled={isConnecting}
                  className="relative overflow-hidden flex items-center justify-between p-4 bg-zinc-900/40 border border-zinc-800 rounded-2xl hover:border-indigo-500/50 hover:bg-zinc-800/40 transition-all duration-300 group"
                >
                  <div className="absolute inset-0 bg-gradient-to-r from-indigo-500/0 via-indigo-500/5 to-indigo-500/0 opacity-0 group-hover:opacity-100 translate-x-[-100%] group-hover:translate-x-[100%] transition-all duration-1000" />
                  <div className="flex items-center gap-4 relative z-10">
                    <div className="w-10 h-10 p-1.5 bg-zinc-800 rounded-xl group-hover:bg-zinc-700 transition-colors shadow-inner">
                      <img src={wallet.logo.src} alt={wallet.title} className="w-full h-full object-contain" />
                    </div>
                    <span className="font-black text-zinc-100 tracking-tight group-hover:text-white transition-colors">{wallet.title}</span>
                  </div>
                  <ChevronRight size={18} className="text-zinc-600 group-hover:text-indigo-400 group-hover:translate-x-1.5 transition-all duration-300 relative z-10" />
                </button>
              ))
            )}
          </motion.div>
        ) : (
          <motion.div
            key="account-list"
            initial={{ opacity: 0, x: 20 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -20 }}
            className="grid gap-2"
          >
            <div className="flex items-center justify-between mb-2">
              <h3 className="text-sm font-bold text-zinc-400 flex items-center gap-2">
                <User size={16} /> Choose Account
              </h3>
              <button
                onClick={() => setActiveWallet(null)}
                className="text-xs text-indigo-400 hover:text-indigo-300 transition-colors"
                aria-label="Back to Wallets"
              >
                Back
              </button>
            </div>
            {accounts.map((acc) => (
              <button
                key={acc.address}
                onClick={() => handleSelectAccount(acc)}
                className="flex flex-col p-3 bg-zinc-900 border border-zinc-800 rounded-xl hover:border-emerald-500/50 hover:bg-zinc-800/80 transition-all text-left"
              >
                <span className="font-bold text-zinc-200 text-sm truncate w-full">{acc.name || "Unnamed Account"}</span>
                <span className="text-[10px] text-zinc-500 font-mono mt-1 opacity-60">{acc.address.slice(0, 8)}...{acc.address.slice(-8)}</span>
              </button>
            ))}
          </motion.div>
        )}
      </AnimatePresence>
      {error && (
        <p className="text-[10px] text-red-500 bg-red-500/10 p-2 rounded-lg border border-red-500/20">
          Error: {error}
        </p>
      )}
    </div>
  );
};
