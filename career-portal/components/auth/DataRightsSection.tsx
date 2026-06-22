"use client";

// PDPA data-subject rights on the member account page (Phase 3): download a copy
// of my data (s.30/s.31) and erase my data (s.33). The backend is self-scoped to
// the caller's session; erasure returns "held" when a legal hold (hired) applies,
// in which case the request is queued for staff instead of erasing immediately.
import { useState } from "react";

import { Eyebrow } from "@/components/ds/Eyebrow";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { api, ApiError } from "@/lib/api";

type EraseResult = { status: "erased" | "held"; message?: string };

function errMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  return "เกิดข้อผิดพลาด กรุณาลองใหม่อีกครั้ง";
}

export function DataRightsSection({ onErased }: { onErased: () => void | Promise<void> }) {
  const [downloading, setDownloading] = useState(false);
  const [downloadErr, setDownloadErr] = useState("");

  const [confirming, setConfirming] = useState(false);
  const [acknowledged, setAcknowledged] = useState(false);
  const [erasing, setErasing] = useState(false);
  const [eraseErr, setEraseErr] = useState("");
  const [held, setHeld] = useState("");

  async function onDownload() {
    setDownloading(true);
    setDownloadErr("");
    try {
      // Fetch the self-scoped export and save just the data payload (pretty JSON),
      // not the API envelope, so the file is a clean copy of the member's data.
      const { data } = await api.get<unknown>("/api/v1/public/auth/me/export");
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "my-data.json";
      document.body.appendChild(a);
      a.click();
      a.remove();
      // Revoke after the download has been initiated (a later tick) so large
      // payloads on slower browsers are not cut off.
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (err) {
      setDownloadErr(errMessage(err));
    } finally {
      setDownloading(false);
    }
  }

  async function onErase() {
    setErasing(true);
    setEraseErr("");
    try {
      const { data } = await api.post<EraseResult>("/api/v1/public/auth/me/erase");
      if (data.status === "held") {
        // A legal hold applies (e.g. hired): the request is queued, not erased.
        setHeld(
          data.message ??
            "คำขอลบข้อมูลของคุณถูกบันทึกและจะดำเนินการโดยเจ้าหน้าที่ บางข้อมูลอาจต้องเก็บไว้ตามกฎหมาย",
        );
        setConfirming(false);
        return;
      }
      if (data.status === "erased") {
        // The account + session are gone; hand back to the page to log out.
        await onErased();
        return;
      }
      // Unexpected status: do not assume erasure happened.
      setEraseErr("ได้รับการตอบกลับที่ไม่คาดคิดจากระบบ กรุณาลองใหม่อีกครั้ง");
    } catch (err) {
      setEraseErr(errMessage(err));
    } finally {
      setErasing(false);
    }
  }

  return (
    <section aria-labelledby="pdpa-heading" className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Eyebrow>ความเป็นส่วนตัว</Eyebrow>
        <h2
          id="pdpa-heading"
          className="[font-size:var(--text-h3)] font-semibold leading-tight text-foreground"
        >
          สิทธิในข้อมูลส่วนบุคคล (PDPA)
        </h2>
        <p className="text-sm text-muted-foreground">
          คุณสามารถขอสำเนาหรือขอลบข้อมูลส่วนบุคคลของคุณได้ตลอดเวลา
        </p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Card>
          <CardContent className="flex flex-col gap-3">
            <div className="flex flex-col gap-1">
              <p className="text-sm font-medium text-foreground">ดาวน์โหลดข้อมูลของฉัน</p>
              <p className="text-sm text-muted-foreground">ขอสำเนาข้อมูลส่วนบุคคลทั้งหมดของคุณเป็นไฟล์ JSON</p>
            </div>
            <div className="mt-auto">
              <Button
                type="button"
                variant="secondary"
                className="h-11 w-full sm:w-auto"
                onClick={onDownload}
                disabled={downloading}
              >
                {downloading ? "กำลังเตรียมไฟล์…" : "ดาวน์โหลดข้อมูล"}
              </Button>
            </div>
            {downloadErr ? (
              <p role="status" aria-live="polite" className="text-sm text-destructive">
                {downloadErr}
              </p>
            ) : null}
          </CardContent>
        </Card>

        <Card className="border-destructive/30">
          <CardContent className="flex flex-col gap-3">
            <div className="flex flex-col gap-1">
              <p className="text-sm font-medium text-foreground">ลบข้อมูลของฉัน</p>
              <p className="text-sm text-muted-foreground">
                ขอให้ลบบัญชีและข้อมูลส่วนบุคคลของคุณอย่างถาวร การดำเนินการนี้ไม่สามารถย้อนกลับได้
              </p>
            </div>

            {held ? (
              <p role="status" aria-live="polite" className="rounded-lg bg-secondary p-3 text-sm text-foreground/90">
                {held}
              </p>
            ) : !confirming ? (
              <div className="mt-auto">
                <Button
                  type="button"
                  variant="destructive"
                  className="h-11 w-full sm:w-auto"
                  onClick={() => {
                    setConfirming(true);
                    setEraseErr("");
                    setAcknowledged(false);
                  }}
                >
                  ขอลบข้อมูลของฉัน
                </Button>
              </div>
            ) : (
              <div className="mt-auto flex flex-col gap-3">
                <label className="flex items-start gap-2.5 text-sm text-foreground/90">
                  <input
                    type="checkbox"
                    checked={acknowledged}
                    onChange={(e) => setAcknowledged(e.target.checked)}
                    className="mt-0.5 size-5 shrink-0"
                  />
                  <span>ฉันเข้าใจว่าการลบข้อมูลนี้เป็นการถาวรและไม่สามารถย้อนกลับได้</span>
                </label>
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    type="button"
                    variant="destructive"
                    className="h-11"
                    onClick={onErase}
                    disabled={!acknowledged || erasing}
                  >
                    {erasing ? "กำลังดำเนินการ…" : "ยืนยันการลบถาวร"}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    className="h-11"
                    onClick={() => {
                      setConfirming(false);
                      setEraseErr("");
                    }}
                    disabled={erasing}
                  >
                    ยกเลิก
                  </Button>
                </div>
              </div>
            )}
            {eraseErr ? (
              <p role="status" aria-live="polite" className="text-sm text-destructive">
                {eraseErr}
              </p>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </section>
  );
}
