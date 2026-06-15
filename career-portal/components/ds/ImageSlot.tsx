import { cn } from "@/lib/utils";

interface ImageSlotProps {
  // Tailwind aspect utility, e.g. "aspect-[4/3]" | "aspect-square" | "aspect-[3/4]".
  aspect?: string;
  // The Thai caption naming the intended photograph, e.g. "ภาพพนักงาน CP Axtra".
  caption?: string;
  className?: string;
  priority?: boolean;
}

// ImageSlot is an intentional placeholder for real-people photography (the #1
// premium lever, assets pending). It is a quiet muted panel with a hairline frame
// and a small centered caption naming the intended image — looks deliberate, not
// broken. When real assets arrive, swap the inner content for next/image.
export function ImageSlot({
  aspect = "aspect-[4/3]",
  caption = "ภาพพนักงาน CP Axtra",
  className,
}: ImageSlotProps) {
  return (
    <div
      role="img"
      aria-label={caption}
      className={cn(
        "relative w-full overflow-hidden rounded-xl border border-line bg-surface-muted",
        aspect,
        className,
      )}
    >
      {/* Quiet centered caption — the placeholder reads as a reserved photo slot. */}
      <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 px-6 text-center">
        <svg
          width="28"
          height="28"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
          className="text-muted-foreground/40"
        >
          <rect x="3" y="5" width="18" height="14" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
          <circle cx="8.5" cy="10" r="1.5" stroke="currentColor" strokeWidth="1.5" />
          <path d="m4 17 4.5-4 3 2.5L16 11l4 4.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <span className="text-xs tracking-wide text-muted-foreground/70">{caption}</span>
      </div>
    </div>
  );
}
