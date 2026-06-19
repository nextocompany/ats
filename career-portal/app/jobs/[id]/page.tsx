// Server wrapper for the job detail page. Provides per-job Open Graph metadata so
// links shared to LINE/Facebook render a rich preview (title + description) instead
// of the generic site card. The interactive body stays client-side (JobDetailClient).
import type { Metadata } from "next";

import type { PositionDetail } from "@/lib/types";
import { JobDetailClient } from "./JobDetailClient";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// fetchPosition pulls the public position server-side for metadata. Best-effort:
// any failure falls back to generic metadata (the client body still renders).
async function fetchPosition(id: string): Promise<PositionDetail | null> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/public/positions/${id}`, {
      next: { revalidate: 300 },
    });
    if (!res.ok) return null;
    const body = (await res.json()) as { data?: PositionDetail };
    return body.data ?? null;
  } catch {
    return null;
  }
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const position = await fetchPosition(id);
  const title = position ? `${position.title_th} | ร่วมงานกับ CP Axtra` : "ตำแหน่งงาน | ร่วมงานกับ CP Axtra";
  const description = position
    ? `สมัครตำแหน่ง ${position.title_th} กับ CP Axtra — สมัครออนไลน์ได้ในไม่กี่นาที`
    : "ดูตำแหน่งงานที่เปิดรับและสมัครงานได้ในไม่กี่ขั้นตอน";

  return {
    title,
    description,
    openGraph: {
      title,
      description,
      url: `/jobs/${id}`,
      type: "website",
    },
    twitter: { card: "summary", title, description },
  };
}

export default async function JobDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <JobDetailClient id={id} />;
}
