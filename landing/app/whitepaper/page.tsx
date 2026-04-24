import type { Metadata } from "next";
import { Navbar } from "@/components/navbar";
import { Footer } from "@/components/footer";
import { WhitepaperContent } from "@/components/whitepaper-content";

export const metadata: Metadata = {
  title: "White Paper — Sovereign DePIN Infrastructure",
  description:
    "EXRA v2.5 Sovereign Edition. Техническая документация: двухуровневая сеть, токеномика, юнит-экономика и дорожная карта.",
};

export default function WhitepaperPage() {
  return (
    <>
      <Navbar />
      <main>
        <WhitepaperContent />
      </main>
      <Footer />
    </>
  );
}
