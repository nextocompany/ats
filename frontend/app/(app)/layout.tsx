import { AppHeader } from "@/components/shell/AppHeader";
import { SideNav } from "@/components/shell/SideNav";
import { MobileBar } from "@/components/shell/MobileBar";

// Shared chrome for all authenticated dashboard routes.
// Desktop ≥1024: persistent navy-blue sidebar + slim context bar.
// Below 1024: top bar + slide-in drawer.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-dvh">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 focus:rounded-md focus:bg-card focus:px-3 focus:py-2 focus:text-sm focus:font-medium focus:shadow-lg focus:ring-2 focus:ring-ring"
      >
        Skip to content
      </a>

      <SideNav />

      <div className="flex min-w-0 flex-1 flex-col">
        <MobileBar />
        <AppHeader />
        <main id="main" className="flex-1 px-5 pb-24 pt-7 sm:px-7 lg:px-8 lg:py-9 lg:pb-24">
          <div className="mx-auto w-full max-w-[1240px]">{children}</div>
        </main>
      </div>
    </div>
  );
}
