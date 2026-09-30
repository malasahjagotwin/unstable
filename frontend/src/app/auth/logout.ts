"use server";

import { redirect } from "next/navigation";

import { clearSessionKey } from "@/lib/session-store";
import { LOGIN_PATH } from "@/lib/session";

export async function logout(): Promise<void> {
  await clearSessionKey();
  redirect(LOGIN_PATH);
}
