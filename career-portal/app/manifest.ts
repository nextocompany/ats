import type { MetadataRoute } from "next";

// App Router web manifest for the candidate-facing career portal PWA.
// Thai-first copy (the audience is Thai job-seekers, often inside the LINE
// in-app browser). Colors mirror globals.css: theme_color is CP Axtra blue
// (--primary, oklch(46% 0.18 264) ≈ #0B47B8), background_color the bright
// near-white shell (--background ≈ #fbfbfe). start_url is /jobs — the
// primary landing surface and the offline-cached shell.
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "ร่วมงานกับเรา",
    short_name: "สมัครงาน",
    description: "ดูตำแหน่งงานที่เปิดรับและสมัครงานได้ในไม่กี่ขั้นตอน",
    start_url: "/jobs",
    scope: "/",
    display: "standalone",
    orientation: "portrait",
    lang: "th",
    dir: "ltr",
    background_color: "#fbfbfe",
    theme_color: "#0B47B8",
    icons: [
      { src: "/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/icon-maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
