import { cn } from "@/lib/utils";

import { Eyebrow } from "./Eyebrow";

interface SectionHeadingProps {
  // Optional plain uppercase context label above the heading.
  eyebrow?: string;
  heading: React.ReactNode;
  // Optional supporting lead paragraph below the heading.
  lead?: React.ReactNode;
  // Heading level — defaults to h2 (sections); use h1 for page titles.
  as?: "h1" | "h2";
  align?: "start" | "center";
  tone?: "default" | "invert";
  className?: string;
}

// SectionHeading is the canonical eyebrow + Anuphan heading + optional lead unit.
// Hierarchy comes from scale contrast (quiet eyebrow → large heading → calm lead),
// not color or decoration.
export function SectionHeading({
  eyebrow,
  heading,
  lead,
  as: Tag = "h2",
  align = "start",
  tone = "default",
  className,
}: SectionHeadingProps) {
  const invert = tone === "invert";
  return (
    <div
      className={cn(
        "flex max-w-2xl flex-col gap-4",
        align === "center" && "mx-auto items-center text-center",
        className,
      )}
    >
      {eyebrow ? <Eyebrow tone={invert ? "invert" : "muted"}>{eyebrow}</Eyebrow> : null}
      <Tag
        className={cn(
          "[font-size:var(--text-h2)] font-semibold leading-[1.1]",
          Tag === "h1" && "[font-size:var(--text-display)]",
          invert ? "text-primary-foreground" : "text-foreground",
        )}
      >
        {heading}
      </Tag>
      {lead ? (
        <p
          className={cn(
            "[font-size:var(--text-lead)] leading-relaxed",
            invert ? "text-primary-foreground/75" : "text-muted-foreground",
          )}
        >
          {lead}
        </p>
      ) : null}
    </div>
  );
}
