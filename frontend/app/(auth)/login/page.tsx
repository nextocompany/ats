"use client";

import { useRouter } from "next/navigation";
import { ArrowRight, Building2, ShieldCheck } from "lucide-react";

import { isEntraConfigured, signIn } from "@/lib/auth";
import { Button } from "@/components/ui/button";

// Confident, branded proof points on the front door — reads as a real platform.
const PROOF = [
  { value: "42", label: "Stores nationwide" },
  { value: "AI", label: "Screening engine" },
  { value: "1", label: "Source of truth" },
] as const;

export default function LoginPage() {
  const router = useRouter();
  const entra = isEntraConfigured();

  return (
    <main className="grid min-h-dvh lg:grid-cols-[1.4fr_minmax(26rem,0.85fr)]">
      {/* Brand panel — the CP Axtra blue "front door" of the enterprise */}
      <section className="relative hidden flex-col justify-between overflow-hidden bg-sidebar p-12 text-sidebar-foreground lg:flex">
        {/* Single brand glow — calm atmosphere, not a flat gradient */}
        <div
          aria-hidden
          className="pointer-events-none absolute -bottom-32 -left-24 size-[26rem] rounded-full opacity-20"
          style={{ background: "radial-gradient(circle, var(--brand) 0%, transparent 65%)" }}
        />
        {/* Crown keyline */}
        <span aria-hidden className="absolute inset-x-0 top-0 h-px bg-sidebar-primary/40" />

        <div className="relative flex flex-col leading-none">
          <span className="text-xs font-semibold uppercase tracking-[0.24em] text-sidebar-foreground/80">
            CP&nbsp;Axtra
          </span>
          <span className="mt-1.5 text-base font-semibold tracking-tight">ATS Console</span>
        </div>

        <div className="relative max-w-md">
          <p className="eyebrow text-brass">National recruitment platform</p>
          <h2 className="mt-3 font-heading text-[3rem] font-semibold leading-[1.02] tracking-tight">
            One pipeline,
            <br />
            <span className="text-brass">every hire.</span>
          </h2>
          <p className="mt-5 text-sm leading-relaxed text-sidebar-foreground/70">
            AI-assisted screening, ranked inboxes, and a single source of truth — from
            application to onboarding, across every store.
          </p>

          {/* Proof ticker — confident, tabular, brand-tinted */}
          <dl className="mt-9 flex divide-x divide-sidebar-border">
            {PROOF.map((p, i) => (
              <div key={p.label} className={i === 0 ? "pr-7" : "px-7"}>
                <dt className="num text-2xl font-semibold tabular-nums tracking-tight text-sidebar-foreground">
                  {p.value}
                </dt>
                <dd className="mt-1 text-[0.6875rem] uppercase tracking-[0.1em] text-sidebar-foreground/55">
                  {p.label}
                </dd>
              </div>
            ))}
          </dl>
        </div>

        <div className="relative flex items-center gap-2 text-xs text-sidebar-foreground/55">
          <ShieldCheck className="size-4 text-brass" />
          Secured access · Azure AD SSO wired in a later sprint
        </div>
      </section>

      {/* Sign-in panel — its own brass keyline + dot accent so the form half
          carries the CP Axtra identity instead of reading as a plain white pane. */}
      <section className="relative flex items-center justify-center px-6 py-10 sm:px-10 sm:py-12">
        {/* Brass keyline seam between the brand panel and the form (desktop) */}
        <span
          aria-hidden
          className="absolute inset-y-12 left-0 hidden w-px lg:block"
          style={{ background: "linear-gradient(to bottom, transparent, var(--brass) 18%, var(--brass) 82%, transparent)" }}
        />
        <div className="w-full max-w-sm">
          {/* Mobile brand hero — a compact version of the desktop "front door" so
              the brand holds at 390 instead of dropping a form into white space.
              Navy panel, text wordmark, headline, and a single proof line. */}
          <div className="relative mb-8 overflow-hidden rounded-2xl bg-sidebar p-6 text-sidebar-foreground lg:hidden">
            <span aria-hidden className="absolute inset-x-0 top-0 h-px bg-sidebar-primary/40" />
            <div className="relative flex flex-col leading-none">
              <span className="text-[0.625rem] font-semibold uppercase tracking-[0.22em] text-sidebar-foreground/80">
                CP&nbsp;Axtra
              </span>
              <span className="mt-1 text-sm font-semibold tracking-tight">ATS Console</span>
            </div>
            <h2 className="relative mt-5 font-heading text-2xl font-semibold leading-[1.05] tracking-tight">
              One pipeline, <span className="text-brass">every hire.</span>
            </h2>
            <p className="relative mt-3 flex items-center gap-2 text-xs text-sidebar-foreground/65">
              <span className="h-px w-5 shrink-0 bg-sidebar-border" aria-hidden />
              AI-assisted screening across 42 stores nationwide
            </p>
          </div>

          <p className="eyebrow brass-underline inline-block">Welcome back</p>
          <h1 className="mt-4 font-heading text-2xl font-semibold tracking-tight">
            Sign in to the console
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Continue to the recruitment command center.
          </p>

          {entra ? (
            <Button
              className="mt-8 h-11 w-full gap-2 text-sm shadow-sm"
              onClick={() => {
                // Redirect to Microsoft; AuthProvider routes onward post-login.
                void signIn();
              }}
            >
              <Building2 className="size-4" />
              Sign in with Microsoft
            </Button>
          ) : (
            <Button
              className="mt-8 h-11 w-full gap-2 text-sm shadow-sm"
              onClick={() => {
                signIn();
                router.push("/applications");
              }}
            >
              Sign in as HR (dev)
              <ArrowRight className="size-4" />
            </Button>
          )}

          <div className="mt-6 rounded-lg border border-hairline bg-card p-4">
            <p className="text-xs leading-relaxed text-muted-foreground">
              {entra
                ? "Single sign-on via Microsoft Entra ID. Access and role permissions are managed by your organization."
                : "Development sign-in for the screening team. Production access will move to Azure AD single sign-on with role-based permissions."}
            </p>
          </div>
        </div>
      </section>
    </main>
  );
}
