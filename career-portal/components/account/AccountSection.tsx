import { cn } from "@/lib/utils";

import { Eyebrow } from "@/components/ds/Eyebrow";
import { Card, CardContent } from "@/components/ui/card";

interface AccountSectionProps {
  // Short uppercase context label above the section title.
  eyebrow: string;
  // The section title (renders as h2 for correct heading order under the page h1).
  title: string;
  // Optional supporting lead line.
  lead?: string;
  // Optional action node placed top-right of the section header (e.g. a count).
  action?: React.ReactNode;
  children: React.ReactNode;
  // tone="danger" hints a compliance/destructive surface with a hairline accent.
  tone?: "default" | "danger";
  // padded wraps children in CardContent padding; set false to control padding.
  padded?: boolean;
  className?: string;
}

// AccountSection is the canonical section frame for the account page: an
// Eyebrow + Anuphan h2 (+ optional lead/action) over a single flat Card. It
// replaces the live page's repeated bare "rounded-xl border bg-card p-6" boxes
// with real design-system primitives and consistent heading structure.
export function AccountSection({
  eyebrow,
  title,
  lead,
  action,
  children,
  tone = "default",
  padded = true,
  className,
}: AccountSectionProps) {
  const headingId = `acct-${title.replace(/\s+/g, "-")}`;

  return (
    <section aria-labelledby={headingId} className={cn("flex flex-col gap-4", className)}>
      <div className="flex items-end justify-between gap-3">
        <div className="flex flex-col gap-1.5">
          <Eyebrow>{eyebrow}</Eyebrow>
          <h2
            id={headingId}
            className="[font-size:var(--text-h3)] font-semibold leading-tight text-foreground"
          >
            {title}
          </h2>
          {lead ? <p className="text-sm text-muted-foreground">{lead}</p> : null}
        </div>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>

      <Card className={cn(tone === "danger" && "border-destructive/30")}>
        {padded ? <CardContent>{children}</CardContent> : children}
      </Card>
    </section>
  );
}
