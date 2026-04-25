import { Navbar } from "@/components/navbar";
import { Hero } from "@/components/hero";
import { HowItWorks } from "@/components/how-it-works";
import { ForBuyers } from "@/components/for-buyers";
import { Features } from "@/components/features";
import { DownloadSection } from "@/components/download-section";
import { Earnings } from "@/components/earnings";
import { Tokenomics } from "@/components/tokenomics";
import { WaitlistSection } from "@/components/waitlist-section";
import { FinalCta } from "@/components/final-cta";
import { Footer } from "@/components/footer";

export default function Page() {
  return (
    <>
      <Navbar />
      <main>
        <Hero />
        <HowItWorks />
        <ForBuyers />
        <Features />
        <DownloadSection />
        <Earnings />
        <Tokenomics />
        <WaitlistSection />
        <FinalCta />
      </main>
      <Footer />
    </>
  );
}
