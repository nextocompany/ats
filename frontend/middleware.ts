import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

import { SESSION_COOKIE } from "@/lib/auth";

// Gate the dashboard behind the dev session cookie; redirect to /login otherwise.
export function middleware(req: NextRequest) {
  const hasSession = req.cookies.has(SESSION_COOKIE);
  if (!hasSession) {
    const url = req.nextUrl.clone();
    url.pathname = "/login";
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  // Protect everything except the login page, Next internals, and static assets.
  matcher: ["/((?!login|_next/static|_next/image|favicon.ico|icon.svg|apple-icon.png).*)"],
};
