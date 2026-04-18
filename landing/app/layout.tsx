import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  subsets: ["latin"],
  variable: "--font-geist-sans",
  display: "swap",
});

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-geist-mono",
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL("https://exra.space"),
  title: {
    default: "EXRA — The decentralized network that runs on every device",
    template: "%s · EXRA",
  },
  description:
    "EXRA turns idle bandwidth and compute into verifiable income. Phones, PCs, routers — earn $EXRA tokens on peaq L1. Buyers tap a global mesh of residential IPs in 60+ countries.",
  keywords: [
    "DePIN",
    "peaq",
    "decentralized network",
    "bandwidth sharing",
    "passive income",
    "EXRA",
    "crypto",
    "Web3 infrastructure",
  ],
  authors: [{ name: "EXRA Network" }],
  creator: "EXRA",
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://exra.space",
    title: "EXRA — The decentralized network that runs on every device",
    description:
      "Turn idle bandwidth into verifiable income. Built on peaq L1.",
    siteName: "EXRA",
  },
  twitter: {
    card: "summary_large_image",
    title: "EXRA — The decentralized network that runs on every device",
    description:
      "Turn idle bandwidth into verifiable income. Built on peaq L1.",
    creator: "@exranetwork",
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-image-preview": "large",
      "max-snippet": -1,
    },
  },
};

export const viewport: Viewport = {
  themeColor: "#09090b",
  colorScheme: "dark",
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    // suppressHydrationWarning on <html>/<body> swallows attribute injects
    // from browser extensions (Bybit Wallet, accessibility plugins that add
    // data-heading-tag, password managers, etc). It does NOT suppress real
    // hydration mismatches inside child trees.
    <html lang="en" className="dark" suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable}`}
        suppressHydrationWarning
      >
        {children}
      </body>
    </html>
  );
}
