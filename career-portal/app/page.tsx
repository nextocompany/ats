import { SiteFooter } from "@/components/SiteFooter";
import { SiteHeader } from "@/components/SiteHeader";
import { Hero } from "@/components/landing/Hero";
import { LandingSections } from "@/components/landing/LandingSections";

// The portal landing page — a proper marketing-grade home (CP Axtra brand). The
// jobs list lives at /jobs (also the PWA start_url).
export default function Home() {
  return (
    <div className="flex min-h-dvh flex-col">
      <SiteHeader />
      <main className="flex-1">
        <Hero />
        <LandingSections />
      </main>
      <SiteFooter />
    </div>
  );
}
