# web

React TypeScript 的模板项目，包含了常用的组件、工具、配置等。

基于 Vite 构建：https://cn.vitejs.dev/

## 快速创建项目

```bash
npx pcq -t vite [project-name]
```

## 开发

```bash
yarn dev
```

## 打包

```bash
yarn build
```

## 环境变量

在项目根目录 `.env` 中配置（修改后需重启开发服务）：

```env
# 系统名称 / 页面标题
VITE_APP_TITLE=React Template Vite

# 侧边栏宽度（px）
VITE_SIDEBAR_WIDTH_MIN=250
VITE_SIDEBAR_WIDTH_MAX=520
VITE_SIDEBAR_WIDTH_DEFAULT=288
```

统一读取入口：`src/config/index.ts`。

---

## 如何新加页面

新增一个后台页面通常需要 **3 步**：页面组件 → 路由 → 侧栏菜单。

### 1. 创建页面组件

在 `src/page/` 下新建目录，例如 `src/page/Hello/`：

```tsx
// src/page/Hello/index.tsx
import styles from './index.module.scss'

const Hello = () => {
  return <div className={styles.page}>Hello</div>
}

export default Hello
```

样式使用 CSS Modules（`index.module.scss`），颜色尽量使用主题变量（如 `var(--app-ink)`），以同时适配明亮 / 暗黑主题。

### 2. 注册路由

编辑 `src/router/config.tsx`，在 `RequireAuth` + `LoggedLayout` 的 `children` 中增加路由：

```tsx
import Hello from '../page/Hello'

// ...
children: [
  // ...
  { path: 'hello', element: <Hello /> },
]
```

访问路径即为 `/hello`。

> 需要登录才能访问的页面放在 `RequireAuth` 包裹的布局下；公开页（如登录）放在布局外，可用 `GuestOnly` 限制已登录用户访问。

### 3. 加入侧边栏菜单

编辑 `src/layout/LoggedLayout/menu.config.tsx`：

```tsx
{ icon: <ApiOutlined />, key: 'hello', label: 'Hello' }
```

菜单 `key` 会拼成路径。规则：

| 配置 | 最终路径 |
|------|----------|
| `{ key: 'hello' }` | `/hello` |
| `{ key: 'demo', children: [{ key: 'a' }] }` | `/demo/a` |
| `{ key: 'test/1' }` | `/test/1` |

父子菜单示例：

```tsx
{
  key: 'demo',
  label: '示例分组',
  children: [
    { key: 'hello', label: 'Hello' },
  ],
}
```

对应路由应写为 `path: 'demo/hello'`。

---

## 如何使用 API

API 客户端封装在 `src/api/index.tsx`，基于 `@hezebin/doraemon` 的 `newApi`，默认 `baseURL` 为 `/api`，并自动附带本地 token。

### 基本用法

```tsx
import { api } from '../../api'

api
  .get('/hello')
  .then((res) => {
    // res 为业务响应体，通常含 code / message / data
    console.log(res)
  })
  .catch((err) => {
    console.error(err)
  })

// 其他方法同理：api.post / api.put / api.delete ...
```

### 约定与行为

- **成功判定**：业务 `code` 为 `0` 或 `1` 视为正常；其他值会通过 `notification` 提示错误。
- **HTTP 错误**：`onError` / `onAbnormal` 中统一 `notification.error`。
- **401**：清空本地 token（后续可配合跳转登录）。
- **Token**：请求时通过 `withToken` 从本地读取；登录成功后由 store 写入。

参考示例：`src/page/Test/index.tsx`。

---

## 如何认证

当前模板为 **演示登录**：任意非空账号密码均可登录，会生成随机 token 并写入本地。

### 流程概览

1. 未登录访问后台路由 → `RequireAuth` 跳转到 `/login`，并带上 `state.from`。
2. 登录页提交 → `store.login(username)` 生成 token + user，持久化到 localStorage。
3. 跳转回 `from` 或首页 `/`。
4. 侧栏底部可 **退出登录** → `store.logout()`，清空 token / user，回到 `/login`。
5. 已登录访问 `/login` → `GuestOnly` 重定向到 `/`。

### 关键文件

| 文件 | 作用 |
|------|------|
| `src/router/AuthGate.tsx` | `RequireAuth` / `GuestOnly` 路由守卫 |
| `src/router/config.tsx` | 登录页与后台布局路由划分 |
| `src/store/index.ts` | `token` / `user`、`login` / `logout`，并同步 localStorage |
| `src/page/Login/index.tsx` | 登录页 UI 与调用 `login` |
| `src/api/index.tsx` | 请求自动带 token；401 时清 token |

### 接入真实后端

将 `store.login` 改为请求真实登录接口，用接口返回的 token / 用户信息调用 `setToken` / `setUser` 即可；路由守卫逻辑可保持不变（仍以本地是否有 `token` 为准）。
