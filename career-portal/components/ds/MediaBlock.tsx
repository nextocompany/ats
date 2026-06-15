import { cn } from "@/lib/utils";

import { Eyebrow } from "./Eyebrow";
import { ImageSlot } from "./ImageSlot";

interface MediaBlockProps {
  eyebrow?: string;
  heading: React.ReactNode;
  body: React.ReactNode;
  // Optional bullet points rendered as a clean hairline-divided list.
  points?: string[];
  // Caption naming the intended photograph for the image slot.
  imageCaption?: string;
  imageAspect?: string;
  // "right" (default) puts the image on the right at lg+; "left" reverses it so
  // alternating sections read as an editorial rhythm.
  imageSide?: "left" | "right";
}

// MediaBlock is the alternating image-slot + text section used across the landing.
// A strict two-column grid at lg+, stacked on mobile. Hierarchy via scale and
// whitespace; the only color is the navy ink and the hairline.
export function MediaBlock({
  eyebrow,
  heading,
  body,
  points,
  imageCaption,
  imageAspect = "aspect-[4/3]",
  imageSide = "right",
}: MediaBlockProps) {
  return (
    <div className="grid items-center gap-10 lg:grid-cols-2 lg:gap-16">
      <div className={cn("flex flex-col gap-5", imageSide === "left" && "lg:order-2")}>
        {eyebrow ? <Eyebrow>{eyebrow}</Eyebrow> : null}
        <h3 className="[font-size:var(--text-h3)] font-semibold leading-snug text-foreground">{heading}</h3>
        <p className="[font-size:var(--text-lead)] leading-relaxed text-muted-foreground">{body}</p>
        {points && points.length > 0 ? (
          <ul className="mt-1 divide-y divide-line border-t border-line">
            {points.map((p) => (
              <li key={p} className="flex items-start gap-3 py-3 text-sm text-foreground/85">
                <span aria-hidden="true" className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                {p}
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      <div className={cn(imageSide === "left" && "lg:order-1")}>
        <ImageSlot aspect={imageAspect} caption={imageCaption} />
      </div>
    </div>
  );
}
