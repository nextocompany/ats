import type { MetadataRoute } from "next";

// App Router web manifest for the candidate-facing career portal PWA.
// Thai-first copy (the audience is Thai job-seekers, often inside the LINE
// in-app browser). Colors mirror globals.css: theme_color is the friendly
// brand green (--primary, oklch(58% 0.15 150) ≈ #1f9d57), background_color the
// warm off-white shell (--background ≈ #fffdf7). start_url is /jobs — the
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
    background_color: "#fffdf7",
    theme_color: "#1f9d57",
    icons: [
      { src: "/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/icon-maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
