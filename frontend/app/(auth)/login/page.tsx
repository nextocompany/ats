"use client";

import { useRouter } from "next/navigation";

import { signIn } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

export default function LoginPage() {
  const router = useRouter();
  return (
    <main className="grid min-h-screen place-items-center bg-muted/30 px-4">
      <Card className="w-full max-w-sm p-8">
        <h1 className="text-2xl font-bold tracking-tight">
          HR<span className="text-[var(--color-accent)]">·</span>ATS
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">Recruitment screening console</p>
        <Button
          className="mt-6 w-full"
          onClick={() => {
            signIn();
            router.push("/applications");
          }}
        >
          Sign in as HR (dev)
        </Button>
        <p className="mt-4 text-xs text-muted-foreground">
          Development sign-in. Azure AD SSO is wired in a later sprint.
        </p>
      </Card>
    </main>
  );
}
