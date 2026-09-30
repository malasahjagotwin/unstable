import { cookies } from "next/headers";

import { SESSION_COOKIE, SESSION_MAX_AGE_SECONDS } from "@/lib/session";

export async function readSessionKey(): Promise<string | null> {
  const store = await cookies();
  return store.get(SESSION_COOKIE)?.value ?? null;
}

export async function writeSessionKey(key: string): Promise<void> {
  const store = await cookies();
  store.set(SESSION_COOKIE, key, {
    httpOnly: true,
    sameSite: "lax",
    secure: process.env.NODE_ENV === "production",
    path: "/",
    maxAge: SESSION_MAX_AGE_SECONDS,
  });
}

export async function clearSessionKey(): Promise<void> {
  const store = await cookies();
  store.delete(SESSION_COOKIE);
}
