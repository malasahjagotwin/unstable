import { redirect } from "next/navigation";

import { logout } from "@/app/auth/logout";
import { LOGIN_PATH, maskKey } from "@/lib/session";
import { readSessionKey } from "@/lib/session-store";

export default async function DashboardLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const key = await readSessionKey();

  if (!key) {
    redirect(LOGIN_PATH);
  }

  return (
    <div className="flex min-h-screen flex-col">
      <header className="flex items-center justify-between gap-4 border-b border-zinc-200 px-6 py-3 dark:border-zinc-800">
        <span className="text-sm font-semibold tracking-tight">unstable 控制台</span>
        <div className="flex items-center gap-4">
          <span className="font-mono text-xs text-zinc-500 dark:text-zinc-400">
            {maskKey(key)}
          </span>
          <form action={logout}>
            <button
              type="submit"
              className="rounded-md border border-zinc-300 px-3 py-1.5 text-xs font-medium transition-colors hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-800"
            >
              退出登录
            </button>
          </form>
        </div>
      </header>
      <main className="flex-1 px-6 py-8">{children}</main>
    </div>
  );
}
