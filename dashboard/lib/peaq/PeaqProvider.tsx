"use client";

import React, { createContext, useContext, useEffect, useState, ReactNode } from "react";
import { ApiPromise, WsProvider } from "@polkadot/api";
import { Wallet, getWallets } from "@talismn/connect-wallets";

interface PeaqContextType {
  api: ApiPromise | null;
  isReady: boolean;
  wallets: Wallet[];
  selectedAccount: any | null;
  setSelectedAccount: (account: any) => void;
  error: string | null;
}

const PeaqContext = createContext<PeaqContextType>({
  api: null,
  isReady: false,
  wallets: [],
  selectedAccount: null,
  setSelectedAccount: () => {},
  error: null,
});

export const PeaqProvider = ({ children }: { children: ReactNode }) => {
  const [api, setApi] = useState<ApiPromise | null>(null);
  const [isReady, setIsReady] = useState(false);
  const [wallets, setWallets] = useState<Wallet[]>([]);
  const [selectedAccount, setSelectedAccount] = useState<any | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const rpcUrl = process.env.NEXT_PUBLIC_PEAQ_RPC || "ws://127.0.0.1:9944";
    const appName = process.env.NEXT_PUBLIC_APP_NAME || "EXRA Dashboard";

    const initApi = async () => {
      try {
        const provider = new WsProvider(rpcUrl);
        const newApi = await ApiPromise.create({ provider });
        await newApi.isReady;
        setApi(newApi);
        setIsReady(true);
        console.log(`[PEAQ] Connected to ${rpcUrl}`);

        // Fetch available wallets
        const availableWallets = getWallets();
        setWallets(availableWallets);
      } catch (err: any) {
        console.error("[PEAQ] Connection error:", err);
        setError(err.message || "Failed to connect to Peaq network");
      }
    };

    initApi();

    return () => {
      if (api) api.disconnect();
    };
  }, []);

  return (
    <PeaqContext.Provider
      value={{
        api,
        isReady,
        wallets,
        selectedAccount,
        setSelectedAccount,
        error,
      }}
    >
      {children}
    </PeaqContext.Provider>
  );
};

export const usePeaq = () => useContext(PeaqContext);
