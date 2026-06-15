import { cn } from "@/lib/utils";

interface EyebrowProps {
  children: React.ReactNode;
  className?: string;
  // tone controls the label color. "muted" (default) is the institutional norm;
  // "accent" reserves the single blue for the rare emphasized eyebrow.
  tone?: "muted" | "accent" | "invert";
}

// Eyebrow is a plain uppercase tracked label that sits above a heading. No dots,
// no pill, no decoration — just a quiet typographic signal of section context.
export function Eyebrow({ children, className, tone = "muted" }: EyebrowProps) {
  return (
    <p
      className={cn(
        "text-xs font-medium uppercase tracking-[0.18em]",
        tone === "muted" && "text-muted-foreground",
        tone === "accent" && "text-primary",
        tone === "invert" && "text-primary-foreground/70",
        className,
      )}
    >
      {children}
    </p>
  );
}
