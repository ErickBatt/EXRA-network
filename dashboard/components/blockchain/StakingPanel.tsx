"use client";

import React, { useEffect, useState } from "react";
import { usePeaq } from "@/lib/peaq/PeaqProvider";
import { motion } from "framer-motion";
import { Rocket, Coins, ShieldCheck, Loader2, ArrowRight } from "lucide-react";
import { formatBalance } from "@polkadot/util";

export const StakingPanel = () => {
  const { api, isReady, selectedAccount } = usePeaq();
  const [balance, setBalance] = useState<string>("0");
  const [isLoadingBalance, setIsLoadingBalance] = useState(false);
  const [isStaking, setIsStaking] = useState(false);
  const [txStatus, setTxStatus] = useState<string | null>(null);

  useEffect(() => {
    if (!api || !selectedAccount) return;

    let unsubscribe: any;

    const watchBalance = async () => {
      setIsLoadingBalance(true);
      try {
        // Subscribe to balance changes
        unsubscribe = await api.query.system.account(
          selectedAccount.address,
          ({ data: { free } }: any) => {
            // Peaq typically uses 18 or 12 decimals, we use formatBalance for human readability
            // Assuming 18 decimals as per standard EVM-compatible Substrate chains like Peaq
            setBalance(free.toString());
            setIsLoadingBalance(false);
          }
        );
      } catch (err) {
        console.error("Failed to fetch balance:", err);
        setIsLoadingBalance(false);
      }
    };

    watchBalance();

    return () => {
      if (unsubscribe) unsubscribe();
    };
  }, [api, selectedAccount]);

  const handleStake = async () => {
    if (!api || !selectedAccount) return;

    setIsStaking(true);
    setTxStatus("Preparing transaction...");

    try {
      const injector = selectedAccount.wallet?.signer;
      
      const tx = api.tx.exra.stakeForPeak();
      
      setTxStatus("Please sign in your wallet...");

      const unsub = await tx.signAndSend(
        selectedAccount.address,
        { signer: injector },
        ({ status, events = [], dispatchError }) => {
          if (status.isInBlock) {
            setTxStatus("Transaction in block. Finalizing...");
          } else if (status.isFinalized) {
            setTxStatus("Success! You are now a Peak Node.");
            setIsStaking(false);
            unsub();
          }

          if (dispatchError) {
            if (dispatchError.isModule) {
              const decoded = api.registry.findMetaError(dispatchError.asModule);
              const { docs, name, section } = decoded;
              setTxStatus(`Error: ${section}.${name} - ${docs.join(" ")}`);
            } else {
              setTxStatus(`Error: ${dispatchError.toString()}`);
            }
            setIsStaking(false);
          }
        }
      );
    } catch (err: any) {
      console.error("Staking failed:", err);
      setTxStatus(`Failed: ${err.message || "Unknown error"}`);
      setIsStaking(false);
    }
  };

  if (!selectedAccount) return null;

  const freeTokens = Number(BigInt(balance) / BigInt(10 ** 18));
  const canStake = freeTokens >= 100;

  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
      className="p-6 bg-zinc-900 border border-zinc-800 rounded-2xl shadow-2xl relative overflow-hidden group"
    >
      {/* Decorative background pulse */}
      <div className="absolute -top-24 -right-24 w-48 h-48 bg-indigo-500/10 rounded-full blur-3xl group-hover:bg-indigo-500/20 transition-all duration-700" />

      <div className="relative z-10 space-y-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-3 bg-emerald-500/10 rounded-xl text-emerald-400">
              <ShieldCheck size={24} />
            </div>
            <div>
              <h2 className="text-xl font-black text-zinc-100 uppercase tracking-tighter">Become a Peak Node</h2>
              <p className="text-xs text-zinc-500 font-medium">Earn 3x more rewards & priority access</p>
            </div>
          </div>
          <div className="text-right">
            <p className="text-[10px] font-bold text-zinc-500 uppercase">Your Balance</p>
            <div className="flex items-center gap-1.5 text-indigo-400 font-black">
              <Coins size={14} />
              <span className="text-lg tabular-nums">
                {isLoadingBalance ? "..." : freeTokens.toLocaleString()} EXRA
              </span>
            </div>
          </div>
        </div>

        <div className="p-4 bg-zinc-950/50 rounded-xl border border-zinc-800/50 space-y-3">
          <div className="flex items-center justify-between text-xs font-bold uppercase tracking-wider">
            <span className="text-zinc-500">Staking Requirement</span>
            <span className="text-zinc-300">100.00 EXRA</span>
          </div>
          <div className="w-full bg-zinc-800 h-1 rounded-full overflow-hidden">
            <motion.div 
              initial={{ width: 0 }}
              animate={{ width: `${Math.min((freeTokens / 100) * 100, 100)}%` }}
              className={`h-full ${canStake ? 'bg-emerald-500' : 'bg-indigo-500 animate-pulse'}`}
            />
          </div>
          {!canStake && (
            <p className="text-[10px] text-zinc-500 text-center italic">
              Need {(100 - freeTokens).toLocaleString()} more EXRA to upgrade
            </p>
          )}
        </div>

        <button
          onClick={handleStake}
          disabled={!canStake || isStaking}
          className={`w-full py-4 rounded-xl font-black uppercase tracking-widest flex items-center justify-center gap-3 transition-all ${
            canStake && !isStaking
              ? 'bg-indigo-500 text-white hover:bg-indigo-400 hover:shadow-lg hover:shadow-indigo-500/20 active:scale-[0.98]'
              : 'bg-zinc-800 text-zinc-500 cursor-not-allowed opacity-50'
          }`}
        >
          {isStaking ? (
            <>
              <Loader2 className="animate-spin" size={20} />
              <span>Processing...</span>
            </>
          ) : (
            <>
              <span>Stake for Peak</span>
              <Rocket size={20} className="group-hover:translate-x-1 transition-transform" />
            </>
          )}
        </button>

        {txStatus && (
          <motion.div 
            initial={{ opacity: 0, y: 5 }}
            animate={{ opacity: 1, y: 0 }}
            className={`text-center text-[10px] font-bold p-2 rounded-lg border ${
              txStatus.includes("Error") || txStatus.includes("Failed")
                ? 'bg-red-500/10 border-red-500/20 text-red-500'
                : 'bg-indigo-500/10 border-indigo-500/20 text-indigo-400'
            }`}
          >
            {txStatus}
          </motion.div>
        )}
      </div>
    </motion.div>
  );
};
