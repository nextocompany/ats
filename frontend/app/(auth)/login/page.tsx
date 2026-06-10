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
    <main className="grid min-h-dvh lg:grid-cols-[1.15fr_1fr]">
      {/* Brand panel — the CP Axtra blue "front door" of the enterprise */}
      <section className="relative hidden flex-col justify-between overflow-hidden bg-sidebar p-12 text-sidebar-foreground lg:flex">
        {/* Brass glow + dot dither — layered atmosphere, not a flat gradient */}
        <div
          aria-hidden
          className="pointer-events-none absolute -right-28 -top-28 size-[28rem] rounded-full opacity-25"
          style={{ background: "radial-gradient(circle, var(--brass) 0%, transparent 62%)" }}
        />
        <div
          aria-hidden
          className="pointer-events-none absolute -bottom-32 -left-24 size-[26rem] rounded-full opacity-20"
          style={{ background: "radial-gradient(circle, var(--brand) 0%, transparent 65%)" }}
        />
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 opacity-[0.06]"
          style={{
            backgroundImage: "radial-gradient(oklch(100% 0 0 / 0.7) 0.7px, transparent 0.7px)",
            backgroundSize: "7px 7px",
          }}
        />
        {/* Brass crown keyline */}
        <span aria-hidden className="absolute inset-x-0 top-0 h-px bg-sidebar-primary/40" />

        <div className="relative flex items-center gap-3">
          <span
            className="relative grid size-10 place-items-center rounded-xl bg-brand font-semibold text-brand-foreground"
            style={{ boxShadow: "inset 0 0 0 1px oklch(100% 0 0 / 0.14), 0 4px 14px -4px oklch(46% 0.18 264 / 0.7)" }}
          >
            HR
            <span
              aria-hidden
              className="absolute -right-1 -top-1 size-2.5 rounded-full bg-brass"
              style={{ boxShadow: "0 0 0 2px var(--sidebar)" }}
            />
          </span>
          <span className="flex flex-col leading-none">
            <span className="text-base font-semibold tracking-tight">ATS Console</span>
            <span className="mt-1 text-[0.625rem] font-medium uppercase tracking-[0.2em] text-brass">
              Recruitment Operations
            </span>
          </span>
        </div>

        <div className="relative max-w-md">
          <div className="dot-cluster mb-6" aria-hidden />
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
                <dt className="font-heading text-2xl font-semibold tabular-nums tracking-tight text-sidebar-foreground">
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

      {/* Sign-in panel */}
      <section className="relative flex items-center justify-center px-6 py-12">
        <div className="w-full max-w-sm">
          {/* Compact brand for mobile, where the left panel is hidden */}
          <div className="mb-8 flex items-center gap-2.5 lg:hidden">
            <span className="relative grid size-9 place-items-center rounded-lg bg-brand text-sm font-semibold text-brand-foreground">
              HR
              <span
                aria-hidden
                className="absolute -right-1 -top-1 size-2 rounded-full bg-brass"
                style={{ boxShadow: "0 0 0 2px var(--background)" }}
              />
            </span>
            <span className="text-base font-semibold tracking-tight">ATS Console</span>
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
