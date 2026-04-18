import type { Metadata } from "next";
import "./globals.css";
import { PeaqProvider } from "@/lib/peaq/PeaqProvider";

export const metadata: Metadata = {
  title: "EXRA — The decentralized network that runs on every device",
  description: "DePIN · PEAQ · Fair Launch",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <PeaqProvider>
          {children}
        </PeaqProvider>
      </body>
    </html>
  );
}
