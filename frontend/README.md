# unstable 控制台前端

Next.js 16（App Router）+ TypeScript + Tailwind CSS 4。依赖 Node 26，通过 nvm 使用。

## 结构

```
src/
  app/
    layout.tsx              根布局，中文元信息
    page.tsx                / 重定向到 /auth/login
    auth/
      layout.tsx            登录区域的居中布局
      login/
        page.tsx            登录页
        actions.ts          login 服务器动作：写入 cookie 后跳转
      logout.ts             logout 服务器动作：清除 cookie 后跳转
    dashboard/
      layout.tsx            受保护布局，缺少 cookie 时重定向到 /auth/login
      page.tsx              控制台占位页
  lib/
    session.ts              纯逻辑：cookie 名、key 校验、key 遮罩
    session.ts 的测试        session.test.ts
    session-store.ts        基于 next/headers 的 cookie 读写
```

## 登录模型

后端没有登录接口，唯一凭据是 `json/users.json` 里每个用户的 `key`。因此登录页收集 key 并写入 httpOnly cookie（`unstable_key`，30 天，`SameSite=Lax`，生产环境附加 `Secure`），后续请求由服务端把它带给 `/fetch`。

key 只在客户端做格式校验（长度上限 128、可见 ASCII 且不含空格），**真正的有效性由后端在首次 `/fetch` 时判定**。页面上的 key 一律经过 `maskKey` 遮罩，不把完整值写进 HTML。

## 命令

```bash
npm run dev        # 开发服务器
npm run build      # 生产构建
npm run start      # 运行生产构建
npm run lint       # ESLint
npm run typecheck  # tsc --noEmit
npm run test       # Vitest
```

## 待补

- 调用后端 `/fetch` 的任务分发表单、方法列表与结果视图。
- 后端地址配置：需要一个环境变量（如 `NEXT_PUBLIC_API_BASE_URL`）指向 Go 服务，尚未引入。
- 登录页无法预先校验 key，若需要即时反馈要给后端补一个鉴权端点。
