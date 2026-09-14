import { createFromIconfontCN } from '@ant-design/icons'
import type { ComponentProps } from 'react'

/**
 * Iconfont 封装。将 iconfont 项目生成的 JS 放到 `public/iconfont.js`。
 *
 * @example
 * <IconFont type="http" />  // → icon-blog-http
 */
const IconFontBase = createFromIconfontCN({
  scriptUrl: '/iconfont.js',
})

const TYPE_PREFIX = 'icon-blog'

export type IconFontProps = Omit<ComponentProps<typeof IconFontBase>, 'type'> & {
  /** 图标名（不含前缀），如 `http` → `icon-blog-http` */
  type: string
}

export function IconFont({ type, ...rest }: IconFontProps) {
  return <IconFontBase type={`${TYPE_PREFIX}-${type}`} {...rest} />
}

export default IconFont
