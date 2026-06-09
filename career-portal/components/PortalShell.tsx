import { Container } from "@/components/Container";
import { SiteHeader } from "@/components/SiteHeader";
import { SiteFooter } from "@/components/SiteFooter";

interface PortalShellProps {
  children: React.ReactNode;
  // showBack renders a back chevron in the header linking to `backHref`.
  backHref?: string;
  // narrow caps the main column for reading/forms (apply, status).
  narrow?: boolean;
  className?: string;
}

// PortalShell is the responsive page chrome shared by inner pages: the site
// header, a width-capped main column (full container, or narrow for forms), and
// the site footer. Works from 320px (LINE in-app browser) to wide desktop.
export function PortalShell({ children, backHref, narrow, className }: PortalShellProps) {
  return (
    <div className="flex min-h-dvh flex-col">
      <SiteHeader backHref={backHref} />
      <main className="flex-1 py-8 sm:py-10">
        <Container narrow={narrow} className={className}>
          {children}
        </Container>
      </main>
      <SiteFooter />
    </div>
  );
}
