import { Navbar } from "@/components/navbar";
import { Hero } from "@/components/hero";
import { HowItWorks } from "@/components/how-it-works";
import { Features } from "@/components/features";
import { Earnings } from "@/components/earnings";
import { Tokenomics } from "@/components/tokenomics";
import { FinalCta } from "@/components/final-cta";
import { Footer } from "@/components/footer";

export default function Page() {
  return (
    <>
      <Navbar />
      <main>
        <Hero />
        <HowItWorks />
        <Features />
        <Earnings />
        <Tokenomics />
        <FinalCta />
      </main>
      <Footer />
    </>
  );
}
