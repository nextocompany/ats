"use client";

import { useRouter } from "next/navigation";
import { ArrowRight, ShieldCheck } from "lucide-react";

import { signIn } from "@/lib/auth";
import { Button } from "@/components/ui/button";

export default function LoginPage() {
  const router = useRouter();

  return (
    <main className="grid min-h-dvh lg:grid-cols-[1.1fr_1fr]">
      {/* Brand panel — the emerald "front door" of the enterprise */}
      <section className="relative hidden flex-col justify-between overflow-hidden bg-sidebar p-12 text-sidebar-foreground lg:flex">
        <div
          aria-hidden
          className="pointer-events-none absolute -right-24 -top-24 size-96 rounded-full opacity-25"
          style={{ background: "radial-gradient(circle, var(--brass) 0%, transparent 65%)" }}
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 opacity-[0.06]"
          style={{
            backgroundImage:
              "radial-gradient(oklch(100% 0 0 / 0.6) 0.6px, transparent 0.6px)",
            backgroundSize: "6px 6px",
          }}
        />

        <div className="relative flex items-center gap-3">
          <span className="grid size-10 place-items-center rounded-lg bg-brand font-semibold text-brand-foreground ring-1 ring-white/10">
            HR
          </span>
          <span className="flex flex-col leading-none">
            <span className="text-base font-semibold tracking-tight">ATS Console</span>
            <span className="mt-1 text-[0.625rem] font-medium uppercase tracking-[0.2em] text-brass">
              Recruitment Operations
            </span>
          </span>
        </div>

        <div className="relative max-w-md">
          <p className="eyebrow">National recruitment platform</p>
          <h2 className="mt-3 font-heading text-[2.5rem] font-semibold leading-[1.05] tracking-tight">
            One pipeline,
            <br />
            <span className="text-brass">every hire.</span>
          </h2>
          <p className="mt-4 text-sm leading-relaxed text-sidebar-foreground/70">
            AI-assisted screening, ranked inboxes, and a single source of truth — from
            application to onboarding, across every store.
          </p>
        </div>

        <div className="relative flex items-center gap-2 text-xs text-sidebar-foreground/55">
          <ShieldCheck className="size-4 text-brass" />
          Secured access · Azure AD SSO wired in a later sprint
        </div>
      </section>

      {/* Sign-in panel */}
      <section className="flex items-center justify-center px-6 py-12">
        <div className="w-full max-w-sm">
          {/* Compact brand for mobile, where the left panel is hidden */}
          <div className="mb-8 flex items-center gap-2.5 lg:hidden">
            <span className="grid size-9 place-items-center rounded-md bg-brand text-sm font-semibold text-brand-foreground">
              HR
            </span>
            <span className="text-base font-semibold tracking-tight">ATS Console</span>
          </div>

          <p className="eyebrow">Welcome back</p>
          <h1 className="mt-1.5 font-heading text-2xl font-semibold tracking-tight">
            Sign in to the console
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Continue to the recruitment command center.
          </p>

          <Button
            className="mt-7 h-11 w-full gap-2 text-sm"
            onClick={() => {
              signIn();
              router.push("/applications");
            }}
          >
            Sign in as HR (dev)
            <ArrowRight className="size-4" />
          </Button>

          <div className="mt-6 rounded-lg border border-hairline bg-card/60 p-4">
            <p className="text-xs leading-relaxed text-muted-foreground">
              Development sign-in for the screening team. Production access will move to
              Azure AD single sign-on with role-based permissions.
            </p>
          </div>
        </div>
      </section>
    </main>
  );
}
