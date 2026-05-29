import { AppHeader } from "@/components/shell/AppHeader";

// Shared chrome for all authenticated dashboard routes.
export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-50 focus:rounded focus:bg-background focus:px-3 focus:py-2 focus:shadow">
        Skip to content
      </a>
      <AppHeader />
      <main id="main" className="mx-auto w-full max-w-[1400px] flex-1 px-4 py-6">
        {children}
      </main>
    </>
  );
}
