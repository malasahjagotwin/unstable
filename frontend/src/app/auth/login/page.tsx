import { redirect } from "next/navigation";

import { login } from "@/app/auth/login/actions";
import { DASHBOARD_PATH } from "@/lib/session";
import { readSessionKey } from "@/lib/session-store";

type LoginPageProps = {
  searchParams: Promise<{ error?: string }>;
};

export default async function LoginPage({ searchParams }: LoginPageProps) {
  if (await readSessionKey()) {
    redirect(DASHBOARD_PATH);
  }

  const { error } = await searchParams;

  return (
    <section className="rounded-xl border border-zinc-200 bg-white p-8 shadow-sm dark:border-zinc-800 dark:bg-zinc-900">
      <h1 className="text-xl font-semibold tracking-tight">登录</h1>
      <p className="mt-2 text-sm text-zinc-500 dark:text-zinc-400">
        填写后端 <code className="font-mono">json/users.json</code> 中分配的 key。
      </p>

      {error ? (
        <p
          role="alert"
          className="mt-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950 dark:text-red-300"
        >
          {error}
        </p>
      ) : null}

      <form action={login} className="mt-6 flex flex-col gap-4">
        <label className="flex flex-col gap-1.5 text-sm">
          <span className="font-medium">key</span>
          <input
            name="key"
            type="text"
            autoComplete="off"
            autoFocus
            required
            placeholder="usr_…"
            className="rounded-md border border-zinc-300 bg-white px-3 py-2 font-mono text-sm outline-none focus:border-zinc-500 dark:border-zinc-700 dark:bg-zinc-950 dark:focus:border-zinc-400"
          />
        </label>

        <button
          type="submit"
          className="rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-700 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-white"
        >
          进入控制台
        </button>
      </form>

      <p className="mt-6 text-xs leading-5 text-zinc-400 dark:text-zinc-500">
        key 会保存在本机的 httpOnly cookie 中，由服务端转发给后端的
        <code className="font-mono"> /fetch </code>
        接口。首次提交任务时才会被后端校验。
      </p>
    </section>
  );
}
