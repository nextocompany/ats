"use client";

import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useInterviewRespond, useInterviewSession } from "@/lib/queries";
import type { InterviewTurn } from "@/lib/types";

interface InterviewChatProps {
  token: string;
}

export function InterviewChat({ token }: InterviewChatProps) {
  const { data, isLoading, isError } = useInterviewSession(token);
  const respond = useInterviewRespond(token);

  const [turns, setTurns] = useState<InterviewTurn[]>([]);
  const [done, setDone] = useState(false);
  const [input, setInput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const seeded = useRef(false);
  const listRef = useRef<HTMLUListElement>(null);

  // Seed local state once from the server. The local conversation stays
  // authoritative after seeding (it carries optimistic sends).
  useEffect(() => {
    if (data && !seeded.current) {
      seeded.current = true;
      setTurns(data.turns);
      setDone(data.done);
    }
  }, [data]);

  // Completion is true if we've seen it locally OR the server reports it (e.g. the
  // session was finished elsewhere) — derived, so no extra effect-driven setState.
  const isDone = done || Boolean(data?.done);

  // Keep the latest message in view by scrolling ONLY the chat list, never the
  // page (a window-level scrollIntoView made the whole screen jump to the bottom
  // after every reply). The list is its own scroll area.
  useEffect(() => {
    const el = listRef.current;
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  }, [turns, respond.isPending]);

  if (isLoading) {
    return <Skeleton className="h-80 w-full rounded-2xl" />;
  }

  if (isError) {
    return (
      <div className="rounded-xl border border-line bg-card p-6 text-center">
        <p className="text-base font-medium text-foreground">ไม่พบการสัมภาษณ์นี้</p>
        <p className="mt-2 text-sm text-muted-foreground">ลิงก์อาจหมดอายุหรือไม่ถูกต้อง กรุณาติดต่อทีม HR</p>
      </div>
    );
  }

  function submitAnswer() {
    const content = input.trim();
    if (!content || respond.isPending || isDone) return;

    setError(null);
    const snapshot = turns; // restore target if the send fails
    setTurns([...snapshot, { role: "user", content }]);
    setInput("");

    respond.mutate(content, {
      onSuccess: (s) => {
        setTurns(s.turns);
        setDone(s.done);
      },
      onError: (err) => {
        setTurns(snapshot); // roll back the optimistic message
        setInput(content);
        setError(err instanceof Error ? err.message : "ส่งคำตอบไม่สำเร็จ กรุณาลองใหม่");
      },
    });
  }

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-3">
        <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">สัมภาษณ์ AI เบื้องต้น</p>
        <h1 className="[font-size:var(--text-h2)] font-semibold leading-tight text-foreground">พูดคุยกับ HR ผู้ช่วย AI</h1>
        <p className="[font-size:var(--text-lead)] text-muted-foreground">ตอบคำถามสั้น ๆ ตามจริง ใช้เวลาประมาณ 5 นาที</p>
      </header>

      <div className="space-y-4 rounded-xl border border-line bg-card p-4 sm:p-6">
        <ul
          ref={listRef}
          className="max-h-[55vh] space-y-4 overflow-y-auto overscroll-contain pr-1"
          aria-live="polite"
          aria-label="บทสนทนาสัมภาษณ์"
        >
          {turns.map((t, i) => (
            <li key={`${i}-${t.role}`} className={t.role === "user" ? "flex justify-end" : "flex justify-start"}>
              <div
                className={
                  t.role === "user"
                    ? "max-w-[85%] rounded-2xl rounded-br-sm bg-primary px-4 py-2.5 text-sm leading-relaxed text-primary-foreground"
                    : "max-w-[85%] rounded-2xl rounded-bl-sm bg-muted px-4 py-2.5 text-sm leading-relaxed text-foreground"
                }
              >
                {t.content}
              </div>
            </li>
          ))}
          {respond.isPending && (
            <li className="flex justify-start">
              <div className="rounded-2xl rounded-bl-sm bg-muted px-4 py-2.5 text-sm text-muted-foreground">
                กำลังพิมพ์…
              </div>
            </li>
          )}
        </ul>
      </div>

      {error && (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      )}

      {isDone ? (
        <div className="rounded-xl border border-line bg-accent-soft p-6 text-center">
          <p className="text-base font-semibold text-foreground">ขอบคุณค่ะ การสัมภาษณ์เสร็จสิ้นแล้ว</p>
          <p className="mt-2 text-sm text-muted-foreground">ทีม HR จะพิจารณาและติดต่อกลับเร็ว ๆ นี้</p>
        </div>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            submitAnswer();
          }}
          className="flex items-end gap-2"
        >
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey && !respond.isPending) {
                e.preventDefault();
                submitAnswer();
              }
            }}
            rows={2}
            placeholder="พิมพ์คำตอบของคุณ…"
            className="min-h-12 flex-1 resize-none rounded-lg border border-input bg-card px-4 py-3 text-base outline-none transition-colors focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40"
            disabled={respond.isPending}
            aria-label="คำตอบ"
          />
          <Button type="submit" size="tap" disabled={respond.isPending || !input.trim()} aria-label="ส่งคำตอบ">
            ส่ง
          </Button>
        </form>
      )}
    </div>
  );
}
