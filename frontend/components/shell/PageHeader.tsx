import type { ReactNode } from "react";

interface PageHeaderProps {
  eyebrow: string;
  title: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
}

// Shared editorial masthead — brass eyebrow, display title, supporting meta,
// optional right-aligned controls. Keeps every surface in one typographic system.
export function PageHeader({ eyebrow, title, meta, actions }: PageHeaderProps) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-x-6 gap-y-4 border-b border-hairline pb-5">
      <div className="min-w-0">
        <p className="eyebrow brass-underline inline-block">{eyebrow}</p>
        <h1 className="mt-3 font-heading text-3xl font-semibold tracking-tight text-foreground">
          {title}
        </h1>
        {meta && <p className="mt-1.5 text-sm text-muted-foreground">{meta}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-end gap-2">{actions}</div>}
    </header>
  );
}
