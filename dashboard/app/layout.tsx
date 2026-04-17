import type { Metadata } from "next";
import "./globals.css";
import { PeaqProvider } from "@/lib/peaq/PeaqProvider";

export const metadata: Metadata = {
  title: "Exra — Your device. Your network. Your income.",
  description: "DePIN · TON · Fair Launch",
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
